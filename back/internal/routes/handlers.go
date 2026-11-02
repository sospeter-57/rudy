package routes

import (
	"net/http"
	"strconv"
	"strings"

	"rudy/internal/models"
)

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

	result, err := s.users.Register(r.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
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

	result, err := s.users.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		s.respondError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if s.users == nil {
		writeError(w, http.StatusNotImplemented, "user service not configured")
		return
	}

	if err := s.users.Logout(r.Context(), s.currentAuth(r).Token); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	user, err := s.users.Me(r.Context(), s.currentUser(r).ID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.users.Users(r.Context())
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.User{"users": users})
}

func (s *server) leaderboard(w http.ResponseWriter, r *http.Request) {
	rows, err := s.users.Leaderboard(r.Context())
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.LeaderboardRow{"leaderboard": rows})
}

func (s *server) ledger(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	userID, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	entries, err := s.tasks.Ledger(r.Context(), userID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.LedgerEntry{"ledger": entries})
}

func (s *server) getPreferences(w http.ResponseWriter, r *http.Request) {
	if s.preferences == nil {
		writeError(w, http.StatusNotImplemented, "preferences service not configured")
		return
	}
	preferences, err := s.preferences.Get(r.Context(), s.currentUser(r).ID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Preferences{"preferences": preferences})
}

func (s *server) putPreferences(w http.ResponseWriter, r *http.Request) {
	if s.preferences == nil {
		writeError(w, http.StatusNotImplemented, "preferences service not configured")
		return
	}

	var req preferencesRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	preferences, err := s.preferences.Update(r.Context(), s.currentUser(r).ID, req.Preferences())
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Preferences{"preferences": preferences})
}

func (s *server) fetchStats(w http.ResponseWriter, r *http.Request) {
	if s.stats == nil {
		writeError(w, http.StatusNotImplemented, "stats service not configured")
		return
	}
	stats, err := s.stats.Stats(r.Context(), r.URL.Query().Get("range"))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Stats{"stats": stats})
}

func (s *server) listProjects(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	projects, err := s.projects.List(r.Context())
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.Project{"projects": projects})
}

func (s *server) createProject(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}

	var req projectRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	project, err := s.projects.Create(r.Context(), s.currentUser(r), req.Name, req.Description, req.TTL)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]models.Project{"project": project})
}

func (s *server) fetchProject(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	project, err := s.projects.Get(r.Context(), id)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *server) listTasks(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}

	query := r.URL.Query()
	projectID := parseOptionalID(query.Get("project_id"))
	assigneeID := parseOptionalID(query.Get("assignee"))
	reviewerID := parseOptionalID(query.Get("reviewer"))
	state := models.TaskState(strings.TrimSpace(query.Get("state")))

	tasks, err := s.tasks.List(r.Context(), projectID, assigneeID, reviewerID, state)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.Task{"tasks": tasks})
}

func parseOptionalID(raw string) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func (s *server) fetchTask(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	task, err := s.tasks.Get(r.Context(), id)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	projectID, ok := pathInt(w, r, "id")
	if !ok {
		return
	}

	var req taskRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	task, err := s.tasks.Create(
		r.Context(), projectID, s.currentUser(r),
		req.Description, req.TaskType, req.Role, req.Points, req.Dependencies,
	)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]models.Task{"task": task})
}

func (s *server) approveTask(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	task, err := s.tasks.Approve(r.Context(), id, s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) rejectTask(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}

	var req rejectRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	task, err := s.tasks.Reject(r.Context(), id, s.currentUser(r), req.Reason)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) assignTask(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}

	var req assignRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	task, err := s.tasks.Assign(r.Context(), id, req.AssigneeID, s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) setGate(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}

	gate := r.PathValue("gate")
	var value string
	switch gate {
	case "code":
		var req codeRequest
		if !s.readJSON(w, r, &req) {
			return
		}
		value = req.Code
	case "tests":
		var req testsRequest
		if !s.readJSON(w, r, &req) {
			return
		}
		value = req.Tests
	case "documentation":
		var req documentationRequest
		if !s.readJSON(w, r, &req) {
			return
		}
		value = req.Documentation
	default:
		writeError(w, http.StatusNotFound, "unknown acceptance gate")
		return
	}

	task, err := s.tasks.SetGate(r.Context(), id, gate, value, s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) addReviewer(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}

	var req reviewerRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	task, err := s.tasks.AddReviewer(r.Context(), id, req.ReviewerID, s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) removeReviewer(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	reviewerID, ok := pathInt(w, r, "userId")
	if !ok {
		return
	}

	task, err := s.tasks.RemoveReviewer(r.Context(), id, reviewerID, s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) submitTask(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	task, err := s.tasks.Submit(r.Context(), id, s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) reviewTask(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}

	var req reviewRequest
	if !s.readJSON(w, r, &req) {
		return
	}

	task, err := s.tasks.Review(r.Context(), id, s.currentUser(r), req.Decision, req.Comment)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.Task{"task": task})
}

func (s *server) completeTask(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		writeError(w, http.StatusNotImplemented, "task service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	result, err := s.tasks.Complete(r.Context(), id, s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
