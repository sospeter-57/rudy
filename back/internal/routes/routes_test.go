package routes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"rudy/internal/services"
	"rudy/internal/storage"
	"rudy/internal/transaction"
)

type testAPI struct {
	server *httptest.Server
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	return newTestAPIWithOrigins(t, nil)
}

func newTestAPIWithOrigins(t *testing.T, corsOrigins []string) *testAPI {
	t.Helper()

	db, err := transaction.Open(filepath.Join(t.TempDir(), "rudy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store, err := storage.NewLocal(filepath.Join(t.TempDir(), "uploads"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	handler := NewRouter(Deps{
		Users:       services.NewUserService(db, time.Hour),
		Projects:    services.NewProjectService(db),
		Tasks:       services.NewTaskService(db),
		Stats:       services.NewStatsService(db),
		Preferences: services.NewPreferencesService(db),
		Chat:        services.NewChatService(db, store, nil),
		CORSOrigins: corsOrigins,
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &testAPI{server: server}
}

func (a *testAPI) do(t *testing.T, method, path, token string, body any) (int, map[string]any) {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequest(method, a.server.URL+path, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer response.Body.Close()

	payload := map[string]any{}
	if response.ContentLength != 0 {
		_ = json.NewDecoder(response.Body).Decode(&payload)
	}
	return response.StatusCode, payload
}

func TestBearerSessionFlow(t *testing.T) {
	api := newTestAPI(t)

	status, payload := api.do(t, http.MethodPost, "/register", "", map[string]any{
		"username": "nova",
		"password": "password123",
		"role":     "Lead",
	})
	if status != http.StatusCreated {
		t.Fatalf("register status = %d (%v)", status, payload)
	}
	token, _ := payload["token"].(string)
	if token == "" {
		t.Fatal("register did not return a token")
	}

	if status, _ := api.do(t, http.MethodGet, "/me", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /me status = %d, want 401", status)
	}
	if status, _ := api.do(t, http.MethodGet, "/me", token, nil); status != http.StatusOK {
		t.Fatalf("authenticated /me status = %d, want 200", status)
	}

	status, payload = api.do(t, http.MethodPost, "/project", token, map[string]any{
		"name":        "Aurora",
		"description": "A test project",
		"ttl":         1440,
	})
	if status != http.StatusCreated {
		t.Fatalf("create project status = %d (%v)", status, payload)
	}

	status, payload = api.do(t, http.MethodGet, "/stats?range=30d", token, nil)
	if status != http.StatusOK {
		t.Fatalf("stats status = %d (%v)", status, payload)
	}
	stats, _ := payload["stats"].(map[string]any)
	totals, _ := stats["totals"].(map[string]any)
	if users, _ := totals["users"].(float64); users != 1 {
		t.Fatalf("stats users = %v, want 1", totals["users"])
	}

	status, payload = api.do(t, http.MethodGet, "/me/preferences", token, nil)
	if status != http.StatusOK {
		t.Fatalf("preferences status = %d (%v)", status, payload)
	}

	status, _ = api.do(t, http.MethodPost, "/logout", token, nil)
	if status != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", status)
	}
	if status, _ := api.do(t, http.MethodGet, "/me", token, nil); status != http.StatusUnauthorized {
		t.Fatalf("revoked /me status = %d, want 401", status)
	}
}

func TestGoogleAuthEndpoint(t *testing.T) {
	db, err := transaction.Open(filepath.Join(t.TempDir(), "rudy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	clientID := "rudy-http-client"
	users := services.NewUserService(db, time.Hour)
	users.SetGoogleClientID(clientID)
	users.SetGoogleValidator(func(_ context.Context, credential, id string) (services.GoogleIdentity, error) {
		if id != clientID {
			return services.GoogleIdentity{}, errors.New("unexpected client id")
		}
		if credential != "verified" {
			return services.GoogleIdentity{}, errors.New("invalid credential")
		}
		return services.GoogleIdentity{
			Subject:       "google-http",
			Email:         "http@example.com",
			Name:          "HTTP Tester",
			EmailVerified: true,
		}, nil
	})

	handler := NewRouter(Deps{
		Users:       users,
		Projects:    services.NewProjectService(db),
		Tasks:       services.NewTaskService(db),
		Stats:       services.NewStatsService(db),
		Preferences: services.NewPreferencesService(db),
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	encoded, err := json.Marshal(map[string]any{
		"credential": "verified",
		"role":       "Frontend Engineer",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/auth/google", bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["token"] == nil {
		t.Fatalf("expected a token, got %v", payload)
	}
}

func TestCORSPolicy(t *testing.T) {
	api := newTestAPIWithOrigins(t, []string{"https://app.example.com"})

	client := &http.Client{}
	request, _ := http.NewRequest(http.MethodOptions, api.server.URL+"/health", nil)
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Access-Control-Request-Method", "GET")
	request.Header.Set("Access-Control-Request-Headers", "Authorization")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", response.StatusCode)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("preflight ACAO = %q", got)
	}
	if got := response.Header.Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Methods")
	}
	if got := response.Header.Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Headers")
	}
	if got := response.Header.Get("Vary"); got == "" {
		t.Fatalf("expected Vary: Origin")
	}

	request, _ = http.NewRequest(http.MethodGet, api.server.URL+"/health", nil)
	request.Header.Set("Origin", "https://app.example.com")
	response, err = client.Do(request)
	if err != nil {
		t.Fatalf("get with origin: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", response.StatusCode)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("get ACAO = %q", got)
	}

	request, _ = http.NewRequest(http.MethodGet, api.server.URL+"/health", nil)
	request.Header.Set("Origin", "https://evil.example.com")
	response, err = client.Do(request)
	if err != nil {
		t.Fatalf("get with disallowed origin: %v", err)
	}
	response.Body.Close()
	if response.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("disallowed origin should not receive ACAO")
	}

	api = newTestAPIWithOrigins(t, []string{"*"})
	request, _ = http.NewRequest(http.MethodGet, api.server.URL+"/health", nil)
	request.Header.Set("Origin", "https://any.example.com")
	response, err = client.Do(request)
	if err != nil {
		t.Fatalf("get wildcard: %v", err)
	}
	response.Body.Close()
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("wildcard ACAO = %q", got)
	}
}

func TestChatEndpointsAndUploads(t *testing.T) {
	api := newTestAPI(t)

	register := func(username string) (int64, string) {
		t.Helper()
		status, payload := api.do(t, http.MethodPost, "/register", "", map[string]any{
			"username": username,
			"password": "password123",
			"role":     "Frontend Engineer",
		})
		if status != http.StatusCreated {
			t.Fatalf("register %s: %d (%v)", username, status, payload)
		}
		user, _ := payload["user"].(map[string]any)
		id, _ := user["id"].(float64)
		token, _ := payload["token"].(string)
		if id == 0 || token == "" {
			t.Fatalf("register %s returned %v", username, payload)
		}
		return int64(id), token
	}

	_, aliceToken := register("alice")
	bobID, bobToken := register("bob")

	status, payload := api.do(t, http.MethodPost, "/chat/dm", aliceToken, map[string]any{"userId": bobID})
	if status != http.StatusOK {
		t.Fatalf("open dm: %d (%v)", status, payload)
	}
	room, _ := payload["room"].(map[string]any)
	roomID, _ := room["id"].(float64)
	if roomID == 0 {
		t.Fatalf("open dm returned %v", payload)
	}

	status, payload = api.do(t, http.MethodPost, "/chat/"+strconv.FormatInt(int64(roomID), 10)+"/messages", aliceToken, map[string]any{
		"body": "hello bob",
	})
	if status != http.StatusCreated {
		t.Fatalf("send message: %d (%v)", status, payload)
	}
	message, _ := payload["message"].(map[string]any)
	messageID, _ := message["id"].(float64)
	if messageID == 0 {
		t.Fatalf("send message returned %v", payload)
	}

	status, payload = api.do(t, http.MethodGet, "/chat/"+strconv.FormatInt(int64(roomID), 10)+"/messages", bobToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list messages: %d (%v)", status, payload)
	}
	messages, _ := payload["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("messages = %v", payload)
	}

	status, payload = api.do(t, http.MethodGet, "/chat/unread", bobToken, nil)
	if unread, _ := payload["unread"].(float64); status != http.StatusOK || unread != 1 {
		t.Fatalf("unread = %d (%v)", status, payload)
	}

	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "shot.png")
	if err != nil {
		t.Fatalf("multipart: %v", err)
	}
	if _, err := part.Write(imageBytes.Bytes()); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, api.server.URL+"/chat/"+strconv.FormatInt(int64(roomID), 10)+"/attachments", &body)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+aliceToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	uploadPayload := map[string]any{}
	_ = json.NewDecoder(response.Body).Decode(&uploadPayload)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d (%v)", response.StatusCode, uploadPayload)
	}
	attachment, _ := uploadPayload["attachment"].(map[string]any)
	attachmentID, _ := attachment["id"].(float64)
	if attachmentID == 0 {
		t.Fatalf("upload returned %v", uploadPayload)
	}

	request, _ = http.NewRequest(http.MethodGet, api.server.URL+"/attachments/"+strconv.FormatInt(int64(attachmentID), 10), nil)
	request.Header.Set("Authorization", "Bearer "+bobToken)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("attachment status = %d type=%q", response.StatusCode, response.Header.Get("Content-Type"))
	}
	if !bytes.Equal(data, imageBytes.Bytes()) {
		t.Fatal("attachment bytes did not round-trip")
	}

	status, _ = api.do(t, http.MethodGet, "/attachments/"+strconv.FormatInt(int64(attachmentID), 10), "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated attachment status = %d, want 401", status)
	}
}

func TestChatEventStreamStartsThroughMiddleware(t *testing.T) {
	api := newTestAPI(t)

	status, payload := api.do(t, http.MethodPost, "/register", "", map[string]any{
		"username": "streamer",
		"password": "password123",
		"role":     "Frontend Engineer",
	})
	if status != http.StatusCreated {
		t.Fatalf("register: %d (%v)", status, payload)
	}
	token, _ := payload["token"].(string)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, api.server.URL+"/chat/events", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d, want 200", response.StatusCode)
	}

	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read ready frame: %v", err)
	}
	if strings.TrimSpace(line) != "event: ready" {
		t.Fatalf("first frame = %q, want event: ready", strings.TrimSpace(line))
	}
}
