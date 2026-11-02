package routes

import (
	"log/slog"
	"net/http"

	"rudy/internal/services"
)

// Deps holds the services the HTTP layer delegates to.
type Deps struct {
	Users       *services.UserService
	Projects    *services.ProjectService
	Tasks       *services.TaskService
	Stats       *services.StatsService
	Preferences *services.PreferencesService
	Chat        *services.ChatService
	CORSOrigins []string
	Logger      *slog.Logger
}

type server struct {
	users       *services.UserService
	projects    *services.ProjectService
	tasks       *services.TaskService
	stats       *services.StatsService
	preferences *services.PreferencesService
	chat        *services.ChatService
	cors        *corsPolicy
	logger      *slog.Logger
}

// NewRouter builds the HTTP handler tree for the API.
func NewRouter(deps Deps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &server{
		users:       deps.Users,
		projects:    deps.Projects,
		tasks:       deps.Tasks,
		stats:       deps.Stats,
		preferences: deps.Preferences,
		chat:        deps.Chat,
		cors:        newCORSPolicy(deps.CORSOrigins),
		logger:      logger,
	}

	mux := http.NewServeMux()

	// Public routes: create an account or exchange credentials for a session.
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /register", s.register)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /auth/google", s.googleAuth)

	// Everything else requires an opaque bearer token.
	protected := func(handler http.HandlerFunc) http.Handler {
		return s.authenticate(handler)
	}

	mux.Handle("POST /logout", protected(s.logout))
	mux.Handle("GET /me", protected(s.me))
	mux.Handle("GET /users", protected(s.listUsers))
	mux.Handle("GET /leaderboard", protected(s.leaderboard))
	mux.Handle("GET /user/{id}/ledger", protected(s.ledger))
	mux.Handle("GET /me/preferences", protected(s.getPreferences))
	mux.Handle("PUT /me/preferences", protected(s.putPreferences))
	mux.Handle("GET /stats", protected(s.fetchStats))

	mux.Handle("GET /projects", protected(s.listProjects))
	mux.Handle("POST /project", protected(s.createProject))
	mux.Handle("GET /project/{id}", protected(s.fetchProject))

	// Projects gain explicit membership, curated by the project lead.
	mux.Handle("GET /project/{id}/members", protected(s.listProjectMembers))
	mux.Handle("POST /project/{id}/members", protected(s.addProjectMember))
	mux.Handle("DELETE /project/{id}/members/{userId}", protected(s.removeProjectMember))
	mux.Handle("POST /project/{id}/join-requests", protected(s.requestProjectJoin))
	mux.Handle("GET /project/{id}/join-requests", protected(s.listProjectJoinRequests))
	mux.Handle("POST /project/{id}/join-requests/{requestId}/approve", protected(s.approveProjectJoinRequest))
	mux.Handle("POST /project/{id}/join-requests/{requestId}/reject", protected(s.rejectProjectJoinRequest))
	mux.Handle("GET /project/{id}/chats", protected(s.listProjectChats))
	mux.Handle("POST /project/{id}/chats", protected(s.createProjectChat))

	// Chat: rooms, messages, attachments and the live event stream.
	mux.Handle("GET /chats", protected(s.listChats))
	mux.Handle("GET /chat/discover", protected(s.listDiscoverableChats))
	mux.Handle("GET /chat/requests", protected(s.listChatRequests))
	mux.Handle("GET /chat/unread", protected(s.chatUnread))
	mux.Handle("GET /chat/events", protected(s.chatEvents))
	mux.Handle("POST /chat", protected(s.createOpenChat))
	mux.Handle("POST /chat/dm", protected(s.openDM))
	mux.Handle("GET /chat/{id}", protected(s.fetchChat))
	mux.Handle("PUT /chat/{id}", protected(s.updateChat))
	mux.Handle("DELETE /chat/{id}", protected(s.archiveChat))
	mux.Handle("POST /chat/{id}/join", protected(s.joinChat))
	mux.Handle("POST /chat/{id}/leave", protected(s.leaveChat))
	mux.Handle("POST /chat/{id}/members", protected(s.addChatMember))
	mux.Handle("DELETE /chat/{id}/members/{userId}", protected(s.removeChatMember))
	mux.Handle("GET /chat/{id}/messages", protected(s.listChatMessages))
	mux.Handle("POST /chat/{id}/messages", protected(s.sendChatMessage))
	mux.Handle("POST /chat/{id}/read", protected(s.markChatRead))
	mux.Handle("POST /chat/{id}/attachments", protected(s.uploadChatAttachment))
	mux.Handle("PATCH /chat/message/{id}", protected(s.editChatMessage))
	mux.Handle("DELETE /chat/message/{id}", protected(s.deleteChatMessage))
	mux.Handle("GET /attachments/{id}", protected(s.serveChatAttachment))

	mux.Handle("GET /tasks", protected(s.listTasks))
	mux.Handle("GET /task/{id}", protected(s.fetchTask))
	mux.Handle("POST /project/{id}/task", protected(s.createTask))
	mux.Handle("POST /task/{id}/approve", protected(s.approveTask))
	mux.Handle("POST /task/{id}/reject", protected(s.rejectTask))
	mux.Handle("POST /task/{id}/assign", protected(s.assignTask))
	mux.Handle("POST /task/{id}/done/{gate}", protected(s.setGate))
	mux.Handle("POST /task/{id}/done/reviewer", protected(s.addReviewer))
	mux.Handle("DELETE /task/{id}/reviewer/{userId}", protected(s.removeReviewer))
	mux.Handle("POST /task/{id}/submit", protected(s.submitTask))
	mux.Handle("POST /task/{id}/review", protected(s.reviewTask))
	mux.Handle("POST /task/{id}/complete", protected(s.completeTask))

	return recoverPanic(logger, logRequests(logger, s.withCORS(requireJSON(mux))))
}
