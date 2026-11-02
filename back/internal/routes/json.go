package routes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"rudy/internal/models"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// decodeJSON reads a single JSON value from the request body, rejecting
// unknown fields and trailing data.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

// readJSON decodes the body and writes the matching error response on failure.
// It reports whether decoding succeeded.
func (s *server) readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := decodeJSON(w, r, dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}

		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// respondError maps a domain error to an HTTP status and writes it. Unexpected
// errors are logged and reported as 500 without leaking details.
func (s *server) respondError(w http.ResponseWriter, err error) {
	status, message := mapError(err)
	if status == http.StatusInternalServerError {
		s.logger.Error("request failed", "error", err)
	}
	writeError(w, status, message)
}

func mapError(err error) (int, string) {
	var appErr *models.AppError
	if errors.As(err, &appErr) {
		switch appErr.Kind {
		case models.KindNotFound:
			return http.StatusNotFound, appErr.Message
		case models.KindUnauthorized:
			return http.StatusUnauthorized, appErr.Message
		case models.KindForbidden:
			return http.StatusForbidden, appErr.Message
		case models.KindConflict:
			return http.StatusConflict, appErr.Message
		case models.KindValidation:
			return http.StatusUnprocessableEntity, appErr.Message
		}
	}

	return http.StatusInternalServerError, "internal server error"
}
