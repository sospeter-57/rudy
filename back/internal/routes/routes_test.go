package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"rudy/internal/services"
	"rudy/internal/transaction"
)

type testAPI struct {
	server *httptest.Server
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()

	db, err := transaction.Open(filepath.Join(t.TempDir(), "rudy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	handler := NewRouter(Deps{
		Users:       services.NewUserService(db, time.Hour),
		Projects:    services.NewProjectService(db),
		Tasks:       services.NewTaskService(db),
		Stats:       services.NewStatsService(db),
		Preferences: services.NewPreferencesService(db),
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
		"role":     "Project Manager",
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
