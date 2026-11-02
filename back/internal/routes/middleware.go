package routes

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rudy/internal/models"
)

type authContext struct {
	User  models.User
	Token string
}

type contextKey string

const authContextKey contextKey = "rudy.auth"

// corsPolicy holds the cross-origin origins allowed by configuration. An empty
// policy leaves the API same-origin only, which is the default for the Vite
// proxy setup.
type corsPolicy struct {
	origins  map[string]struct{}
	allowAny bool
}

func newCORSPolicy(origins []string) *corsPolicy {
	policy := &corsPolicy{origins: make(map[string]struct{}, len(origins))}
	for _, origin := range origins {
		origin = strings.TrimSpace(strings.TrimSuffix(origin, "/"))
		if origin == "" {
			continue
		}
		if origin == "*" {
			policy.allowAny = true
			continue
		}
		policy.origins[origin] = struct{}{}
	}
	return policy
}

func (p *corsPolicy) allowed(origin string) (string, bool) {
	if origin == "" {
		return "", false
	}
	if p.allowAny {
		return "*", true
	}
	if _, ok := p.origins[origin]; ok {
		return origin, true
	}
	return "", false
}

// cors answers browser preflights and adds the CORS response headers for the
// origins listed in RUDY_CORS_ORIGINS. Origins are never hardcoded: when the
// variable is unset the API stays same-origin only.
func (s *server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedOrigin, ok := s.cors.allowed(strings.TrimSpace(r.Header.Get("Origin")))
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		w.Header().Set("Access-Control-Max-Age", "600")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authenticate resolves the bearer token to a user and rejects the request when
// it is missing, expired or revoked.
func (s *server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.users == nil {
			writeError(w, http.StatusNotImplemented, "user service not configured")
			return
		}

		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		user, err := s.users.Authenticate(r.Context(), token)
		if err != nil {
			s.respondError(w, err)
			return
		}

		ctx := context.WithValue(r.Context(), authContextKey, authContext{User: user, Token: token})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func (s *server) currentAuth(r *http.Request) authContext {
	value, _ := r.Context().Value(authContextKey).(authContext)
	return value
}

func (s *server) currentUser(r *http.Request) models.User {
	return s.currentAuth(r).User
}

func pathInt(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, name+" must be a positive integer")
		return 0, false
	}
	return id, true
}

// recoverPanic turns panics into a JSON 500 response instead of dropping the
// connection.
func recoverPanic(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered",
					"panic", recovered,
					"method", r.Method,
					"path", r.URL.Path,
				)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// isUploadPath reports whether a POST is the chat image upload endpoint.
func isUploadPath(path string) bool {
	if !strings.HasSuffix(path, "/attachments") {
		return false
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	return len(segments) == 3 && segments[0] == "chat"
}

// logRequests logs one line per request with method, path, status and latency.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

// Unwrap lets http.ResponseController reach the underlying writer for things
// like SetWriteDeadline on the SSE stream.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Flush forwards to the underlying writer so streaming handlers keep working
// through the logging middleware.
func (w *statusWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	flusher.Flush()
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.status = status
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// requireJSON enforces an application/json content type on requests that carry
// a body.
func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			// Requests without a body (logout, approve, complete, …) have no
			// media type to validate.
			if r.ContentLength == 0 {
				break
			}
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
			// Chat image uploads are the one multipart endpoint. Keep the JSON
			// rule everywhere else so every other request stays predictable.
			if mediaType == "multipart/form-data" && isUploadPath(r.URL.Path) {
				break
			}
			if mediaType != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
