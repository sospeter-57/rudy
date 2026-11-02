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
	if actor.Role != models.Lead {
		return models.Project{}, models.Forbidden("only a Lead can create a project")
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

	now := nowUTC()
	var project models.Project
	err := s.db.WithTx(ctx, func(repo *transaction.Repo) error {
		created, err := repo.CreateProject(ctx, name, description, ttl, actor.ID, now)
		if err != nil {
			return err
		}
		project = created
		return repo.AddProjectMember(ctx, project.ID, actor.ID, "lead", actor.ID, now)
	})
	if err != nil {
		return models.Project{}, err
	}
	return project, nil
}

func (s *ProjectService) Members(ctx context.Context, projectID int64) ([]models.ProjectMember, error) {
	if _, err := s.db.Repo().ProjectByID(ctx, projectID); err != nil {
		return nil, err
	}
	return s.db.Repo().ProjectMembers(ctx, projectID)
}

// AddMember is lead-curated: only the project creator may add a member, and the
// member must already have a Rudy account.
func (s *ProjectService) AddMember(ctx context.Context, projectID, actorID, userID int64) ([]models.ProjectMember, error) {
	project, err := s.db.Repo().ProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.CreatorID != actorID {
		return nil, models.Forbidden("only the project lead can add members")
	}
	if userID == project.CreatorID {
		return nil, models.Conflict("the project lead is already a member")
	}
	exists, err := s.db.Repo().UserExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, models.NotFound("user not found")
	}
	if err := s.db.Repo().AddProjectMember(ctx, projectID, userID, "member", actorID, nowUTC()); err != nil {
		return nil, err
	}
	return s.db.Repo().ProjectMembers(ctx, projectID)
}

func (s *ProjectService) RemoveMember(ctx context.Context, projectID, actorID, userID int64) ([]models.ProjectMember, error) {
	project, err := s.db.Repo().ProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.CreatorID != actorID {
		return nil, models.Forbidden("only the project lead can remove members")
	}
	if userID == project.CreatorID {
		return nil, models.Conflict("the project lead cannot be removed")
	}

	err = s.db.WithTx(ctx, func(repo *transaction.Repo) error {
		if err := repo.RemoveProjectMember(ctx, projectID, userID); err != nil {
			return err
		}
		return repo.RemoveMemberFromProjectChats(ctx, projectID, userID)
	})
	if err != nil {
		return nil, err
	}
	return s.db.Repo().ProjectMembers(ctx, projectID)
}
