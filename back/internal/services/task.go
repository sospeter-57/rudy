package services

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"rudy/internal/models"
	"rudy/internal/transaction"
)

type TaskService struct {
	db *transaction.DB
}

func NewTaskService(db *transaction.DB) *TaskService {
	return &TaskService{db: db}
}

func (s *TaskService) List(ctx context.Context, projectID, assigneeID, reviewerID int64, state models.TaskState) ([]models.Task, error) {
	return s.db.Repo().Tasks(ctx, transaction.TaskFilter{
		ProjectID:  projectID,
		AssigneeID: assigneeID,
		ReviewerID: reviewerID,
		State:      state,
	})
}

func (s *TaskService) Get(ctx context.Context, id int64) (models.Task, error) {
	return s.db.Repo().TaskByID(ctx, id)
}

func (s *TaskService) Create(
	ctx context.Context,
	projectID int64,
	actor models.User,
	description string,
	taskType models.TaskType,
	role models.Role,
	points int,
	dependencies []int64,
) (models.Task, error) {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) < 4 {
		return models.Task{}, models.Validation("describe the task in a little more detail")
	}
	if !models.IsTaskType(taskType) {
		return models.Task{}, models.Validation("task type is not valid")
	}
	if !models.IsRole(role) {
		return models.Task{}, models.Validation("role is not a known role")
	}
	if points < 1 || points > 100 {
		return models.Task{}, models.Validation("points must be between 1 and 100")
	}

	repo := s.db.Repo()
	if exists, err := repo.ProjectExists(ctx, projectID); err != nil {
		return models.Task{}, err
	} else if !exists {
		return models.Task{}, models.NotFound("project not found")
	}

	seen := map[int64]bool{}
	cleanDependencies := make([]int64, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency <= 0 || seen[dependency] {
			continue
		}
		seen[dependency] = true
		parent, err := repo.TaskByID(ctx, dependency)
		if err != nil {
			return models.Task{}, err
		}
		if parent.ProjectID != projectID {
			return models.Task{}, models.Validation("dependencies must belong to the same project")
		}
		if parent.TaskState == models.Completed {
			return models.Task{}, models.Validation("a completed task cannot be a dependency")
		}
		cleanDependencies = append(cleanDependencies, dependency)
	}

	now := nowUTC()
	task := models.Task{
		ProjectID:     projectID,
		Description:   description,
		TaskType:      taskType,
		TaskState:     models.Pending,
		ApprovalState: models.Proposed,
		CreatorID:     actor.ID,
		Role:          role,
		Points:        points,
		Dependencies:  cleanDependencies,
		Acceptance: models.Acceptance{
			Reviewers: []models.Review{},
		},
		CreatedAt: now,
	}

	var id int64
	err := s.db.WithTx(ctx, func(tx *transaction.Repo) error {
		var err error
		id, err = tx.CreateTask(ctx, task)
		return err
	})
	if err != nil {
		return models.Task{}, err
	}
	task.ID = id
	return task, nil
}

func (s *TaskService) Approve(ctx context.Context, taskID int64, actor models.User) (models.Task, error) {
	if err := requireManager(actor); err != nil {
		return models.Task{}, err
	}
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if task.TaskState == models.Completed {
		return models.Task{}, models.Conflict("completed tasks cannot be approved")
	}
	if task.ApprovalState == models.Approved {
		return models.Task{}, models.Conflict("this task is already approved")
	}
	if err := s.db.Repo().UpdateTaskApproval(ctx, taskID, models.Approved, ""); err != nil {
		return models.Task{}, err
	}
	return s.Get(ctx, taskID)
}

func (s *TaskService) Reject(ctx context.Context, taskID int64, actor models.User, reason string) (models.Task, error) {
	if err := requireManager(actor); err != nil {
		return models.Task{}, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return models.Task{}, models.Validation("give the creator a reason")
	}
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if task.TaskState == models.Completed {
		return models.Task{}, models.Conflict("completed tasks cannot be rejected")
	}
	if err := s.db.Repo().UpdateTaskApproval(ctx, taskID, models.ChangesRequested, reason); err != nil {
		return models.Task{}, err
	}
	return s.Get(ctx, taskID)
}

func (s *TaskService) Assign(ctx context.Context, taskID, assigneeID int64, actor models.User) (models.Task, error) {
	if err := requireManager(actor); err != nil {
		return models.Task{}, err
	}
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if task.ApprovalState != models.Approved {
		return models.Task{}, models.Conflict("approve the task before assigning it")
	}
	if task.TaskState == models.Completed {
		return models.Task{}, models.Conflict("completed tasks are locked")
	}
	exists, err := s.db.Repo().UserExists(ctx, assigneeID)
	if err != nil {
		return models.Task{}, err
	}
	if !exists {
		return models.Task{}, models.NotFound("assignee not found")
	}
	if err := s.db.Repo().UpdateTaskAssignee(ctx, taskID, assigneeID); err != nil {
		return models.Task{}, err
	}
	return s.Get(ctx, taskID)
}

func (s *TaskService) SetGate(ctx context.Context, taskID int64, gate, value string, actor models.User) (models.Task, error) {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if err := requirePreparation(task, actor); err != nil {
		return models.Task{}, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return models.Task{}, models.Validation("%s is required", gate)
	}
	if err := s.db.Repo().UpdateTaskGate(ctx, taskID, gate, value); err != nil {
		return models.Task{}, err
	}
	return s.Get(ctx, taskID)
}

func (s *TaskService) AddReviewer(ctx context.Context, taskID, reviewerID int64, actor models.User) (models.Task, error) {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if err := requirePreparation(task, actor); err != nil {
		return models.Task{}, err
	}
	if task.AssigneeID != nil && *task.AssigneeID == reviewerID {
		return models.Task{}, models.Conflict("the assignee cannot review their own work")
	}
	exists, err := s.db.Repo().UserExists(ctx, reviewerID)
	if err != nil {
		return models.Task{}, err
	}
	if !exists {
		return models.Task{}, models.NotFound("reviewer not found")
	}
	if err := s.db.Repo().AddReviewer(ctx, taskID, reviewerID); err != nil {
		if isUniqueViolation(err) {
			return models.Task{}, models.Conflict("that person is already a reviewer")
		}
		return models.Task{}, err
	}
	return s.Get(ctx, taskID)
}

func (s *TaskService) RemoveReviewer(ctx context.Context, taskID, reviewerID int64, actor models.User) (models.Task, error) {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if err := requirePreparation(task, actor); err != nil {
		return models.Task{}, err
	}
	if err := s.db.Repo().RemoveReviewer(ctx, taskID, reviewerID); err != nil {
		return models.Task{}, err
	}
	return s.Get(ctx, taskID)
}

func (s *TaskService) Submit(ctx context.Context, taskID int64, actor models.User) (models.Task, error) {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if !canWorkOn(task, actor) {
		return models.Task{}, models.Forbidden("only the assignee or a Lead can change this task")
	}
	if task.ApprovalState != models.Approved {
		return models.Task{}, models.Conflict("a lead must approve this task first")
	}
	if task.TaskState != models.Pending {
		return models.Task{}, models.Conflict("this task is not pending")
	}
	if !task.GatesComplete() {
		return models.Task{}, models.Conflict("code, tests and documentation are all required before review")
	}
	if len(task.Acceptance.Reviewers) == 0 {
		return models.Task{}, models.Conflict("add at least one reviewer before requesting review")
	}
	if err := s.db.Repo().SubmitForReview(ctx, taskID); err != nil {
		return models.Task{}, err
	}
	return s.Get(ctx, taskID)
}

func (s *TaskService) Review(ctx context.Context, taskID int64, actor models.User, decision models.ReviewDecision, comment string) (models.Task, error) {
	if decision != models.ReviewApproved && decision != models.ReviewChangesRequested {
		return models.Task{}, models.Validation("decision must be approved or changes_requested")
	}
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if task.TaskState != models.Auditing {
		return models.Task{}, models.Conflict("this task is not in review")
	}
	isReviewer := false
	for _, review := range task.Acceptance.Reviewers {
		if review.UserID == actor.ID {
			isReviewer = true
			break
		}
	}
	if !isReviewer {
		return models.Task{}, models.Forbidden("you are not a reviewer on this task")
	}
	if task.AssigneeID != nil && *task.AssigneeID == actor.ID {
		return models.Task{}, models.Forbidden("the assignee cannot review their own work")
	}

	now := nowUTC()
	if err := s.db.WithTx(ctx, func(tx *transaction.Repo) error {
		if err := tx.UpdateReviewerDecision(ctx, taskID, actor.ID, decision, strings.TrimSpace(comment), now); err != nil {
			return err
		}
		if decision == models.ReviewChangesRequested {
			return tx.UpdateTaskState(ctx, taskID, models.Pending)
		}
		return nil
	}); err != nil {
		return models.Task{}, err
	}
	return s.Get(ctx, taskID)
}

func (s *TaskService) Complete(ctx context.Context, taskID int64, actor models.User) (models.CompletionResult, error) {
	if err := requireManager(actor); err != nil {
		return models.CompletionResult{}, err
	}

	var result models.CompletionResult
	err := s.db.WithTx(ctx, func(tx *transaction.Repo) error {
		task, err := tx.TaskByID(ctx, taskID)
		if err != nil {
			return err
		}
		if task.TaskState == models.Completed {
			return models.Conflict("this task is already completed")
		}
		if task.ApprovalState != models.Approved {
			return models.Conflict("a task must be approved before it can be completed")
		}
		if task.TaskState != models.Pending && task.TaskState != models.Auditing {
			return models.Conflict("this task is not in a completable state")
		}
		if !task.GatesComplete() {
			return models.Conflict("code, tests and documentation must all be present")
		}
		if len(task.Acceptance.Reviewers) > 0 && !task.AllReviewersApproved() {
			return models.Conflict("every reviewer must approve before this task can be completed")
		}
		if task.AssigneeID == nil {
			return models.Conflict("assign this task before completing it so the points have an owner")
		}

		now := nowUTC()
		taskIDCopy := task.ID
		projectIDCopy := task.ProjectID
		awards := []models.LedgerEntry{}

		assigneeID := *task.AssigneeID
		awards = append(awards, models.LedgerEntry{
			UserID:    assigneeID,
			TaskID:    &taskIDCopy,
			ProjectID: &projectIDCopy,
			Reason:    models.ReasonTaskCompleted,
			Points:    task.Points,
			Note:      fmt.Sprintf("Completed %q", task.Description),
			CreatedAt: now,
		})

		bonus := models.ReviewBonus(task.Points)
		for _, review := range task.Acceptance.Reviewers {
			if review.Decision != models.ReviewApproved || review.UserID == assigneeID {
				continue
			}
			awards = append(awards, models.LedgerEntry{
				UserID:    review.UserID,
				TaskID:    &taskIDCopy,
				ProjectID: &projectIDCopy,
				Reason:    models.ReasonReviewBonus,
				Points:    bonus,
				Note:      fmt.Sprintf("Reviewed %q", task.Description),
				CreatedAt: now,
			})
		}

		for index := range awards {
			entry, err := tx.InsertLedgerEntry(ctx, awards[index])
			if err != nil {
				if isUniqueViolation(err) {
					return models.Conflict("this task has already been paid out")
				}
				return err
			}
			awards[index] = entry
		}

		if err := tx.MarkTaskCompleted(ctx, taskID, now); err != nil {
			return err
		}

		userIDs := make([]int64, 0, len(awards))
		for _, award := range awards {
			userIDs = append(userIDs, award.UserID)
		}
		if err := tx.RecomputePoints(ctx, uniqueIDs(userIDs)); err != nil {
			return err
		}

		completed, err := tx.TaskByID(ctx, taskID)
		if err != nil {
			return err
		}
		result = models.CompletionResult{Task: completed, Awards: awards}
		return nil
	})
	if err != nil {
		return models.CompletionResult{}, err
	}
	return result, nil
}

func (s *TaskService) Ledger(ctx context.Context, userID int64) ([]models.LedgerEntry, error) {
	return s.db.Repo().LedgerForUser(ctx, userID)
}

func requireManager(actor models.User) error {
	if actor.Role != models.Lead {
		return models.Forbidden("only a Lead can do that")
	}
	return nil
}

func canWorkOn(task models.Task, actor models.User) bool {
	return actor.Role == models.Lead || (task.AssigneeID != nil && *task.AssigneeID == actor.ID)
}

// requirePreparation enforces the approval-first rule. Acceptance evidence and
// reviewers may only change after a lead approves the task and before it is
// submitted for review.
func requirePreparation(task models.Task, actor models.User) error {
	if !canWorkOn(task, actor) {
		return models.Forbidden("only the assignee or a Lead can change this task")
	}
	if task.ApprovalState != models.Approved {
		return models.Conflict("a lead must approve this task before work can begin")
	}
	if task.TaskState != models.Pending {
		return models.Conflict("acceptance criteria are locked while the task is in review")
	}
	return nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}
