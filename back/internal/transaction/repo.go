package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"rudy/internal/models"
)

const timeLayout = time.RFC3339

// TimeLayout is the single timestamp format stored in SQLite and emitted over
// the API. It is fixed width in UTC so lexicographic comparisons work.
func TimeLayout() string {
	return timeLayout
}

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Repo struct {
	q Querier
}

func (d *DB) Repo() *Repo {
	return &Repo{q: d.sql}
}

func (d *DB) WithTx(ctx context.Context, fn func(*Repo) error) error {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&Repo{q: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func formatTime(value time.Time) string {
	return value.UTC().Format(timeLayout)
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(timeLayout, value)
	return parsed.UTC()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (models.User, error) {
	var (
		user      models.User
		email     sql.NullString
		createdAt string
	)
	err := row.Scan(&user.ID, &user.Username, &email, &user.Role, &user.Points, &createdAt)
	if err != nil {
		return models.User{}, err
	}
	if email.Valid {
		user.Email = email.String
	}
	user.CreatedAt = parseTime(createdAt)
	return user, nil
}

func (r *Repo) CreateUser(ctx context.Context, username, passwordHash string, role models.Role, createdAt time.Time) (models.User, error) {
	result, err := r.q.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, points, created_at)
		VALUES (?, ?, ?, 0, ?)`,
		username, passwordHash, role, formatTime(createdAt),
	)
	if err != nil {
		return models.User{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.User{}, err
	}
	return models.User{
		ID:        id,
		Username:  username,
		Role:      role,
		Points:    0,
		CreatedAt: createdAt.UTC(),
	}, nil
}

// CreateGoogleUser inserts an account that authenticates through Google only.
// passwordHash is an unusable placeholder, so password login can never match.
func (r *Repo) CreateGoogleUser(ctx context.Context, username, email, googleSub, passwordHash string, role models.Role, createdAt time.Time) (models.User, error) {
	result, err := r.q.ExecContext(ctx, `
		INSERT INTO users (username, email, google_sub, password_hash, role, points, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)`,
		username, email, googleSub, passwordHash, role, formatTime(createdAt),
	)
	if err != nil {
		return models.User{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.User{}, err
	}
	return models.User{
		ID:        id,
		Username:  username,
		Email:     email,
		Role:      role,
		Points:    0,
		CreatedAt: createdAt.UTC(),
	}, nil
}

func (r *Repo) UserByUsername(ctx context.Context, username string) (models.User, string, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT id, username, email, role, points, created_at, password_hash
		FROM users WHERE username = ? COLLATE NOCASE`, username)
	var (
		user         models.User
		email        sql.NullString
		createdAt    string
		passwordHash string
	)
	if err := row.Scan(&user.ID, &user.Username, &email, &user.Role, &user.Points, &createdAt, &passwordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, "", models.NotFound("user not found")
		}
		return models.User{}, "", err
	}
	if email.Valid {
		user.Email = email.String
	}
	user.CreatedAt = parseTime(createdAt)
	return user, passwordHash, nil
}

func (r *Repo) UserByID(ctx context.Context, id int64) (models.User, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT id, username, email, role, points, created_at FROM users WHERE id = ?`, id)
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, models.NotFound("user not found")
	}
	return user, err
}

func (r *Repo) UserByEmail(ctx context.Context, email string) (models.User, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT id, username, email, role, points, created_at
		FROM users WHERE email = ? COLLATE NOCASE`, email)
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, models.NotFound("user not found")
	}
	return user, err
}

func (r *Repo) UserByGoogleSub(ctx context.Context, sub string) (models.User, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT id, username, email, role, points, created_at
		FROM users WHERE google_sub = ?`, sub)
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, models.NotFound("user not found")
	}
	return user, err
}

// LinkGoogleAccount attaches a verified Google identity to an existing account.
func (r *Repo) LinkGoogleAccount(ctx context.Context, userID int64, email, sub string) error {
	result, err := r.q.ExecContext(ctx, `
		UPDATE users SET email = ?, google_sub = ? WHERE id = ?`, email, sub, userID)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "user not found")
}

func (r *Repo) Users(ctx context.Context) ([]models.User, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT id, username, email, role, points, created_at
		FROM users ORDER BY points DESC, username ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *Repo) CreateSession(ctx context.Context, id string, userID int64, tokenHash string, createdAt, expiresAt time.Time) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		id, userID, tokenHash, formatTime(createdAt), formatTime(expiresAt),
	)
	return err
}

func (r *Repo) SessionUser(ctx context.Context, tokenHash string, now time.Time) (models.User, string, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.email, u.role, u.points, u.created_at, s.id
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ?`,
		tokenHash, formatTime(now),
	)

	var (
		user      models.User
		email     sql.NullString
		createdAt string
		sessionID string
	)
	err := row.Scan(&user.ID, &user.Username, &email, &user.Role, &user.Points, &createdAt, &sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, "", models.Unauthorized("session expired or invalid")
	}
	if err != nil {
		return models.User{}, "", err
	}
	if email.Valid {
		user.Email = email.String
	}
	user.CreatedAt = parseTime(createdAt)
	return user, sessionID, nil
}

func (r *Repo) TouchSession(ctx context.Context, sessionID string, now time.Time) error {
	_, err := r.q.ExecContext(ctx, `UPDATE sessions SET last_used_at = ? WHERE id = ?`, formatTime(now), sessionID)
	return err
}

func (r *Repo) RevokeSession(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.q.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL`,
		formatTime(now), tokenHash,
	)
	return err
}

func (r *Repo) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ? OR revoked_at IS NOT NULL`, formatTime(now))
	return err
}

func (r *Repo) CreateProject(ctx context.Context, name, description string, ttl int, creatorID int64, createdAt time.Time) (models.Project, error) {
	result, err := r.q.ExecContext(ctx, `
		INSERT INTO projects (name, description, ttl, creator_id, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		name, description, ttl, creatorID, formatTime(createdAt),
	)
	if err != nil {
		return models.Project{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Project{}, err
	}
	return models.Project{
		ID:          id,
		Name:        name,
		Description: description,
		TTL:         ttl,
		CreatorID:   creatorID,
		CreatedAt:   createdAt.UTC(),
	}, nil
}

func (r *Repo) ProjectByID(ctx context.Context, id int64) (models.Project, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT id, name, description, ttl, creator_id, created_at FROM projects WHERE id = ?`, id)
	var (
		project   models.Project
		createdAt string
	)
	err := row.Scan(&project.ID, &project.Name, &project.Description, &project.TTL, &project.CreatorID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Project{}, models.NotFound("project not found")
	}
	if err != nil {
		return models.Project{}, err
	}
	project.CreatedAt = parseTime(createdAt)
	return project, nil
}

func (r *Repo) Projects(ctx context.Context) ([]models.Project, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT id, name, description, ttl, creator_id, created_at
		FROM projects ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []models.Project{}
	for rows.Next() {
		var (
			project   models.Project
			createdAt string
		)
		if err := rows.Scan(&project.ID, &project.Name, &project.Description, &project.TTL, &project.CreatorID, &createdAt); err != nil {
			return nil, err
		}
		project.CreatedAt = parseTime(createdAt)
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (r *Repo) CreateTask(ctx context.Context, task models.Task) (int64, error) {
	var assignee any
	if task.AssigneeID != nil {
		assignee = *task.AssigneeID
	}
	result, err := r.q.ExecContext(ctx, `
		INSERT INTO tasks (
			project_id, description, task_type, task_state, approval_state,
			creator_id, assignee_id, role, points, rejection_reason, created_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ProjectID, task.Description, task.TaskType, task.TaskState, task.ApprovalState,
		task.CreatorID, assignee, task.Role, task.Points, nullString(task.RejectionReason),
		formatTime(task.CreatedAt), nil,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if _, err := r.q.ExecContext(ctx, `
		INSERT INTO task_acceptance (task_id, code, tests, documentation) VALUES (?, '', '', '')`, id); err != nil {
		return 0, err
	}

	for _, dependency := range task.Dependencies {
		if _, err := r.q.ExecContext(ctx, `
			INSERT INTO task_dependencies (task_id, depends_on_task_id) VALUES (?, ?)`, id, dependency); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

type TaskFilter struct {
	ProjectID  int64
	AssigneeID int64
	ReviewerID int64
	State      models.TaskState
}

func (r *Repo) Tasks(ctx context.Context, filter TaskFilter) ([]models.Task, error) {
	query := `SELECT t.id FROM tasks t WHERE 1 = 1`
	args := []any{}
	if filter.ProjectID > 0 {
		query += ` AND t.project_id = ?`
		args = append(args, filter.ProjectID)
	}
	if filter.AssigneeID > 0 {
		query += ` AND t.assignee_id = ?`
		args = append(args, filter.AssigneeID)
	}
	if filter.ReviewerID > 0 {
		query += ` AND EXISTS (SELECT 1 FROM task_reviewers tr WHERE tr.task_id = t.id AND tr.user_id = ?)`
		args = append(args, filter.ReviewerID)
	}
	if filter.State != "" {
		query += ` AND t.task_state = ?`
		args = append(args, filter.State)
	}
	query += ` ORDER BY t.created_at DESC, t.id DESC`

	rows, err := r.q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	tasks := make([]models.Task, 0, len(ids))
	for _, id := range ids {
		task, err := r.TaskByID(ctx, id)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (r *Repo) TaskByID(ctx context.Context, id int64) (models.Task, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT
			t.id, t.project_id, t.description, t.task_type, t.task_state, t.approval_state,
			t.creator_id, t.assignee_id, t.role, t.points, t.rejection_reason,
			t.created_at, t.completed_at,
			COALESCE(a.code, ''), COALESCE(a.tests, ''), COALESCE(a.documentation, '')
		FROM tasks t
		LEFT JOIN task_acceptance a ON a.task_id = t.id
		WHERE t.id = ?`, id)

	var (
		task        models.Task
		assignee    sql.NullInt64
		rejection   sql.NullString
		createdAt   string
		completedAt sql.NullString
	)
	err := row.Scan(
		&task.ID, &task.ProjectID, &task.Description, &task.TaskType, &task.TaskState, &task.ApprovalState,
		&task.CreatorID, &assignee, &task.Role, &task.Points, &rejection,
		&createdAt, &completedAt,
		&task.Acceptance.Code, &task.Acceptance.Tests, &task.Acceptance.Documentation,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Task{}, models.NotFound("task not found")
	}
	if err != nil {
		return models.Task{}, err
	}
	if assignee.Valid {
		value := assignee.Int64
		task.AssigneeID = &value
	}
	if rejection.Valid {
		task.RejectionReason = rejection.String
	}
	task.CreatedAt = parseTime(createdAt)
	if completedAt.Valid {
		value := parseTime(completedAt.String)
		task.CompletedAt = &value
	}

	dependencies, err := r.taskDependencies(ctx, id)
	if err != nil {
		return models.Task{}, err
	}
	task.Dependencies = dependencies

	reviewers, err := r.taskReviewers(ctx, id)
	if err != nil {
		return models.Task{}, err
	}
	task.Acceptance.Reviewers = reviewers
	return task, nil
}

func (r *Repo) taskDependencies(ctx context.Context, taskID int64) ([]int64, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT depends_on_task_id FROM task_dependencies WHERE task_id = ? ORDER BY depends_on_task_id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (r *Repo) taskReviewers(ctx context.Context, taskID int64) ([]models.Review, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT user_id, decision, COALESCE(comment, ''), decided_at
		FROM task_reviewers WHERE task_id = ? ORDER BY user_id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.Review{}
	for rows.Next() {
		var (
			review    models.Review
			decidedAt sql.NullString
		)
		if err := rows.Scan(&review.UserID, &review.Decision, &review.Comment, &decidedAt); err != nil {
			return nil, err
		}
		if decidedAt.Valid {
			value := parseTime(decidedAt.String)
			review.DecidedAt = &value
		}
		result = append(result, review)
	}
	return result, rows.Err()
}

func (r *Repo) UserExists(ctx context.Context, id int64) (bool, error) {
	var count int
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repo) ProjectExists(ctx context.Context, id int64) (bool, error) {
	var count int
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id = ?`, id).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repo) UpdateTaskApproval(ctx context.Context, id int64, approval models.ApprovalState, rejection string) error {
	result, err := r.q.ExecContext(ctx, `
		UPDATE tasks SET approval_state = ?, rejection_reason = ? WHERE id = ?`,
		approval, nullString(rejection), id,
	)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "task not found")
}

func (r *Repo) UpdateTaskAssignee(ctx context.Context, id, assigneeID int64) error {
	result, err := r.q.ExecContext(ctx, `UPDATE tasks SET assignee_id = ? WHERE id = ?`, assigneeID, id)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "task not found")
}

func (r *Repo) UpdateTaskGate(ctx context.Context, id int64, gate, value string) error {
	var column string
	switch gate {
	case "code":
		column = "code"
	case "tests":
		column = "tests"
	case "documentation":
		column = "documentation"
	default:
		return models.Validation("unknown acceptance gate %q", gate)
	}

	result, err := r.q.ExecContext(ctx, fmt.Sprintf(`UPDATE task_acceptance SET %s = ? WHERE task_id = ?`, column), value, id)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "task not found")
}

func (r *Repo) AddReviewer(ctx context.Context, taskID, userID int64) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO task_reviewers (task_id, user_id, decision) VALUES (?, ?, 'pending')`, taskID, userID)
	return err
}

func (r *Repo) RemoveReviewer(ctx context.Context, taskID, userID int64) error {
	result, err := r.q.ExecContext(ctx, `DELETE FROM task_reviewers WHERE task_id = ? AND user_id = ?`, taskID, userID)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "reviewer not found")
}

func (r *Repo) SubmitForReview(ctx context.Context, taskID int64) error {
	if _, err := r.q.ExecContext(ctx, `UPDATE tasks SET task_state = 'auditing' WHERE id = ?`, taskID); err != nil {
		return err
	}
	_, err := r.q.ExecContext(ctx, `
		UPDATE task_reviewers SET decision = 'pending', comment = NULL, decided_at = NULL WHERE task_id = ?`, taskID)
	return err
}

func (r *Repo) UpdateReviewerDecision(ctx context.Context, taskID, userID int64, decision models.ReviewDecision, comment string, decidedAt time.Time) error {
	result, err := r.q.ExecContext(ctx, `
		UPDATE task_reviewers SET decision = ?, comment = ?, decided_at = ?
		WHERE task_id = ? AND user_id = ?`,
		decision, nullString(comment), formatTime(decidedAt), taskID, userID,
	)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "you are not a reviewer on this task")
}

func (r *Repo) UpdateTaskState(ctx context.Context, taskID int64, state models.TaskState) error {
	result, err := r.q.ExecContext(ctx, `UPDATE tasks SET task_state = ? WHERE id = ?`, state, taskID)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "task not found")
}

func (r *Repo) MarkTaskCompleted(ctx context.Context, taskID int64, completedAt time.Time) error {
	result, err := r.q.ExecContext(ctx, `
		UPDATE tasks SET task_state = 'completed', completed_at = ? WHERE id = ?`, formatTime(completedAt), taskID)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "task not found")
}

func (r *Repo) InsertLedgerEntry(ctx context.Context, entry models.LedgerEntry) (models.LedgerEntry, error) {
	result, err := r.q.ExecContext(ctx, `
		INSERT INTO ledger_entries (user_id, task_id, project_id, reason, points, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.UserID, entry.TaskID, entry.ProjectID, entry.Reason, entry.Points, entry.Note, formatTime(entry.CreatedAt),
	)
	if err != nil {
		return models.LedgerEntry{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.LedgerEntry{}, err
	}
	entry.ID = id
	return entry, nil
}

func (r *Repo) RecomputePoints(ctx context.Context, userIDs []int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(userIDs))
	args := make([]any, len(userIDs))
	for index, id := range userIDs {
		placeholders[index] = "?"
		args[index] = id
	}
	_, err := r.q.ExecContext(ctx, fmt.Sprintf(`
		UPDATE users SET points = COALESCE((SELECT SUM(points) FROM ledger_entries WHERE user_id = users.id), 0)
		WHERE id IN (%s)`, strings.Join(placeholders, ",")), args...)
	return err
}

func (r *Repo) LedgerForUser(ctx context.Context, userID int64) ([]models.LedgerEntry, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT id, user_id, task_id, project_id, reason, points, note, created_at
		FROM ledger_entries WHERE user_id = ? ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []models.LedgerEntry{}
	for rows.Next() {
		var (
			entry     models.LedgerEntry
			taskID    sql.NullInt64
			projectID sql.NullInt64
			createdAt string
		)
		if err := rows.Scan(&entry.ID, &entry.UserID, &taskID, &projectID, &entry.Reason, &entry.Points, &entry.Note, &createdAt); err != nil {
			return nil, err
		}
		if taskID.Valid {
			value := taskID.Int64
			entry.TaskID = &value
		}
		if projectID.Valid {
			value := projectID.Int64
			entry.ProjectID = &value
		}
		entry.CreatedAt = parseTime(createdAt)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *Repo) PreferencesByUser(ctx context.Context, userID int64) (models.Preferences, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT theme_mode, accent, blend_a, blend_b, updated_at
		FROM user_preferences WHERE user_id = ?`, userID)
	var (
		preferences models.Preferences
		updatedAt   string
	)
	err := row.Scan(&preferences.ThemeMode, &preferences.Accent, &preferences.BlendA, &preferences.BlendB, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.DefaultPreferences(), nil
	}
	if err != nil {
		return models.Preferences{}, err
	}
	preferences.UpdatedAt = parseTime(updatedAt)
	return preferences, nil
}

func (r *Repo) UpsertPreferences(ctx context.Context, userID int64, preferences models.Preferences, updatedAt time.Time) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, theme_mode, accent, blend_a, blend_b, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			theme_mode = excluded.theme_mode,
			accent = excluded.accent,
			blend_a = excluded.blend_a,
			blend_b = excluded.blend_b,
			updated_at = excluded.updated_at`,
		userID, preferences.ThemeMode, preferences.Accent, preferences.BlendA, preferences.BlendB, formatTime(updatedAt),
	)
	return err
}

// ---------- project join requests ----------

func (r *Repo) CreateProjectJoinRequest(ctx context.Context, projectID, requesterID int64, createdAt time.Time) (int64, error) {
	result, err := r.q.ExecContext(ctx, `
		INSERT INTO project_join_requests (project_id, requester_id, status, created_at)
		VALUES (?, ?, 'pending', ?)`, projectID, requesterID, formatTime(createdAt))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *Repo) ProjectJoinRequests(ctx context.Context, projectID int64) ([]models.ProjectJoinRequest, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT r.id, r.project_id, r.requester_id, u.username, r.status, r.created_at, r.reviewed_by, r.reviewed_at
		FROM project_join_requests r
		JOIN users u ON u.id = r.requester_id
		WHERE r.project_id = ?
		ORDER BY CASE WHEN r.status = 'pending' THEN 0 ELSE 1 END, r.created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := []models.ProjectJoinRequest{}
	for rows.Next() {
		var (
			request    models.ProjectJoinRequest
			status     string
			createdAt  string
			reviewedBy sql.NullInt64
			reviewedAt sql.NullString
		)
		if err := rows.Scan(&request.ID, &request.ProjectID, &request.RequesterID, &request.RequesterName, &status, &createdAt, &reviewedBy, &reviewedAt); err != nil {
			return nil, err
		}
		request.Status = models.ProjectJoinRequestStatus(status)
		request.CreatedAt = parseTime(createdAt)
		if reviewedBy.Valid {
			value := reviewedBy.Int64
			request.ReviewedBy = &value
		}
		if reviewedAt.Valid {
			value := parseTime(reviewedAt.String)
			request.ReviewedAt = &value
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (r *Repo) ProjectJoinRequestByID(ctx context.Context, id int64) (models.ProjectJoinRequest, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT r.id, r.project_id, r.requester_id, u.username, r.status, r.created_at, r.reviewed_by, r.reviewed_at
		FROM project_join_requests r
		JOIN users u ON u.id = r.requester_id
		WHERE r.id = ?`, id)
	var (
		request    models.ProjectJoinRequest
		status     string
		createdAt  string
		reviewedBy sql.NullInt64
		reviewedAt sql.NullString
	)
	err := row.Scan(&request.ID, &request.ProjectID, &request.RequesterID, &request.RequesterName, &status, &createdAt, &reviewedBy, &reviewedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ProjectJoinRequest{}, models.NotFound("join request not found")
	}
	if err != nil {
		return models.ProjectJoinRequest{}, err
	}
	request.Status = models.ProjectJoinRequestStatus(status)
	request.CreatedAt = parseTime(createdAt)
	if reviewedBy.Valid {
		value := reviewedBy.Int64
		request.ReviewedBy = &value
	}
	if reviewedAt.Valid {
		value := parseTime(reviewedAt.String)
		request.ReviewedAt = &value
	}
	return request, nil
}

func (r *Repo) HasPendingProjectJoinRequest(ctx context.Context, projectID, requesterID int64) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_join_requests WHERE project_id = ? AND requester_id = ? AND status = 'pending'`,
		projectID, requesterID).Scan(&count)
	return count > 0, err
}

func (r *Repo) ReviewProjectJoinRequest(ctx context.Context, id int64, status models.ProjectJoinRequestStatus, reviewerID int64, reviewedAt time.Time) error {
	result, err := r.q.ExecContext(ctx, `
		UPDATE project_join_requests
		SET status = ?, reviewed_by = ?, reviewed_at = ?
		WHERE id = ? AND status = 'pending'`,
		status, reviewerID, formatTime(reviewedAt), id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return models.Conflict("join request is no longer pending")
	}
	return nil
}

func (r *Repo) requireAffected(result sql.Result, message string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return models.NotFound(message)
	}
	return nil
}
