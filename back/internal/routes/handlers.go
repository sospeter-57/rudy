package routes

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"rudy/internal/models"
)

var validRoles = map[models.Role]struct{}{
	models.ProjectManager:    {},
	models.DatabaseEngineer:  {},
	models.FrontendEngineer:  {},
	models.BackendEngineer:   {},
	models.FullstackEngineer: {},
	models.QualityAssuranceE: {},
	models.SoftwareEngineer:  {},
	models.DataScientist:     {},
	models.UIUXDesigner:      {},
}

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerRequest struct {
	Username string      `json:"username"`
	Password string      `json:"password"`
	Role     models.Role `json:"role"`
}

type codeRequest struct {
	Code string `json:"code"`
}

type testsRequest struct {
	Tests string `json:"tests"`
}

type reviewerRequest struct {
	ReviewerID int64 `json:"reviewer_id"`
}

type documentationRequest struct {
	Documentation string `json:"documentation"`
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	if s.users == nil {
		writeError(w, http.StatusNotImplemented, "user service not configured")
		return
	}

	var req registerRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if n := len([]rune(req.Username)); n < 3 || n > 24 {
		writeError(w, http.StatusBadRequest, "username must be between 3 and 24 characters")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if _, ok := validRoles[req.Role]; !ok {
		writeError(w, http.StatusBadRequest, "role is not a known role")
		return
	}

	user, err := s.users.Register(r.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]models.User{"user": user})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	if s.users == nil {
		writeError(w, http.StatusNotImplemented, "user service not configured")
		return
	}

	var req credentialsRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := s.users.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]models.User{"user": user})
}

func (s *server) fetchProject(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}

	id, ok := pathID(w, r)
	if !ok {
		return
	}

	project, err := s.projects.Project(r.Context(), id)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (s *server) fetchUserTasks(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}

	userID, err := headerUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-User-ID header")
		return
	}

	tasks, err := s.tasks.UserTasks(r.Context(), userID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}

	writeJSON(w, http.StatusOK, map[string][]models.Task{"tasks": tasks})
}

func (s *server) doneCodeImplementation(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}

	taskID, ok := pathID(w, r)
	if !ok {
		return
	}

	var req codeRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	task, err := s.tasks.SetCode(r.Context(), taskID, req.Code)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) doneTests(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}

	taskID, ok := pathID(w, r)
	if !ok {
		return
	}

	var req testsRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Tests) == "" {
		writeError(w, http.StatusBadRequest, "tests are required")
		return
	}

	task, err := s.tasks.SetTests(r.Context(), taskID, req.Tests)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) doneReviewer(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}

	taskID, ok := pathID(w, r)
	if !ok {
		return
	}

	var req reviewerRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if req.ReviewerID <= 0 {
		writeError(w, http.StatusBadRequest, "reviewer_id must be a positive integer")
		return
	}

	task, err := s.tasks.AddReviewer(r.Context(), taskID, req.ReviewerID)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) doneDocumentation(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}

	taskID, ok := pathID(w, r)
	if !ok {
		return
	}

	var req documentationRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Documentation) == "" {
		writeError(w, http.StatusBadRequest, "documentation is required")
		return
	}

	task, err := s.tasks.SetDocumentation(r.Context(), taskID, req.Documentation)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

// pathID parses the positive integer named path parameter, writing a 400
// response when it is missing or invalid.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}

	return id, true
}

func headerUserID(r *http.Request) (int64, error) {
	raw := r.Header.Get("X-User-ID")
	if raw == "" {
		return 0, errors.New("missing X-User-ID header")
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid X-User-ID header %q", raw)
	}

	return id, nil
}
