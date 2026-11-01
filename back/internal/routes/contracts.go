package routes

import (
	"context"

	"rudy/internal/models"
)

// UserService is the part of the user domain the HTTP layer depends on.
// Implementations are responsible for hashing and verifying passwords.
type UserService interface {
	Register(ctx context.Context, username, password string, role models.Role) (models.User, error)
	Login(ctx context.Context, username, password string) (models.User, error)
}

// ProjectService is the part of the project domain the HTTP layer depends on.
type ProjectService interface {
	Project(ctx context.Context, id int64) (models.Project, error)
}

// TaskService is the part of the task domain the HTTP layer depends on.
type TaskService interface {
	UserTasks(ctx context.Context, userID int64) ([]models.Task, error)
	SetCode(ctx context.Context, taskID int64, code string) (models.Task, error)
	SetTests(ctx context.Context, taskID int64, tests string) (models.Task, error)
	AddReviewer(ctx context.Context, taskID, reviewerID int64) (models.Task, error)
	SetDocumentation(ctx context.Context, taskID int64, documentation string) (models.Task, error)
}
