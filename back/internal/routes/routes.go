package routes

import (
	"log/slog"
	"net/http"
)

// Deps holds the services the HTTP layer delegates to. Any service left nil
// causes the routes that depend on it to respond with 501 Not Implemented.
type Deps struct {
	Users    UserService
	Projects ProjectService
	Tasks    TaskService
	Logger   *slog.Logger
}

type server struct {
	users    UserService
	projects ProjectService
	tasks    TaskService
	logger   *slog.Logger
}

// NewRouter builds the HTTP handler tree for the API.
func NewRouter(deps Deps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &server{
		users:    deps.Users,
		projects: deps.Projects,
		tasks:    deps.Tasks,
		logger:   logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", s.register)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("GET /project/{id}", s.fetchProject)
	mux.HandleFunc("GET /user/tasks", s.fetchUserTasks)
	mux.HandleFunc("POST /task/{id}/done/code", s.doneCodeImplementation)
	mux.HandleFunc("POST /task/{id}/done/tests", s.doneTests)
	mux.HandleFunc("POST /task/{id}/done/reviewer", s.doneReviewer)
	mux.HandleFunc("POST /task/{id}/done/documentation", s.doneDocumentation)

	return recoverPanic(logger, logRequests(logger, requireJSON(mux)))
}
