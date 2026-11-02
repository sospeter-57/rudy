package services

import (
	"context"
	"strings"
	"unicode/utf8"

	"rudy/internal/models"
	"rudy/internal/transaction"
)

type ProjectService struct {
	db *transaction.DB
}

func NewProjectService(db *transaction.DB) *ProjectService {
	return &ProjectService{db: db}
}

func (s *ProjectService) List(ctx context.Context) ([]models.Project, error) {
	return s.db.Repo().Projects(ctx)
}

func (s *ProjectService) Get(ctx context.Context, id int64) (models.Project, error) {
	return s.db.Repo().ProjectByID(ctx, id)
}

func (s *ProjectService) Create(ctx context.Context, actor models.User, name, description string, ttl int) (models.Project, error) {
	if actor.Role != models.ProjectManager {
		return models.Project{}, models.Forbidden("only a Project Manager can create a project")
	}

	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if count := utf8.RuneCountInString(name); count < 3 || count > 24 {
		return models.Project{}, models.Validation("project name must be between 3 and 24 characters")
	}
	if description == "" {
		return models.Project{}, models.Validation("a project description is required")
	}
	if ttl <= 0 {
		return models.Project{}, models.Validation("time to live must be positive")
	}

	return s.db.Repo().CreateProject(ctx, name, description, ttl, actor.ID, nowUTC())
}
