package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"rudy/internal/models"
	"rudy/internal/transaction"
)

const (
	minUsernameRunes = 3
	maxUsernameRunes = 24
	minPasswordRunes = 8
)

type UserService struct {
	db         *transaction.DB
	sessionTTL time.Duration
	bcryptCost int
}

func NewUserService(db *transaction.DB, sessionTTL time.Duration) *UserService {
	if sessionTTL <= 0 {
		sessionTTL = 30 * 24 * time.Hour
	}
	return &UserService{db: db, sessionTTL: sessionTTL, bcryptCost: bcrypt.DefaultCost}
}

func (s *UserService) Register(ctx context.Context, username, password string, role models.Role) (models.AuthResult, error) {
	username = strings.TrimSpace(username)
	if count := utf8.RuneCountInString(username); count < minUsernameRunes || count > maxUsernameRunes {
		return models.AuthResult{}, models.Validation("username must be between %d and %d characters", minUsernameRunes, maxUsernameRunes)
	}
	if utf8.RuneCountInString(password) < minPasswordRunes {
		return models.AuthResult{}, models.Validation("password must be at least %d characters", minPasswordRunes)
	}
	if !models.IsRole(role) {
		return models.AuthResult{}, models.Validation("role is not a known role")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return models.AuthResult{}, err
	}

	now := time.Now().UTC()
	var user models.User
	var token string
	var expiresAt time.Time
	err = s.db.WithTx(ctx, func(repo *transaction.Repo) error {
		created, err := repo.CreateUser(ctx, username, string(hash), role, now)
		if err != nil {
			if isUniqueViolation(err) {
				return models.Conflict("username already taken")
			}
			return err
		}
		user = created

		if err := repo.DeleteExpiredSessions(ctx, now); err != nil {
			return err
		}
		token, expiresAt, err = s.createSession(ctx, repo, user.ID, now)
		return err
	})
	if err != nil {
		return models.AuthResult{}, err
	}

	return models.AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *UserService) Login(ctx context.Context, username, password string) (models.AuthResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return models.AuthResult{}, models.Validation("username and password are required")
	}

	repo := s.db.Repo()
	user, hash, err := repo.UserByUsername(ctx, username)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok && appErr.Kind == models.KindNotFound {
			return models.AuthResult{}, models.Unauthorized("invalid credentials")
		}
		return models.AuthResult{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return models.AuthResult{}, models.Unauthorized("invalid credentials")
	}

	now := time.Now().UTC()
	var token string
	var expiresAt time.Time
	err = s.db.WithTx(ctx, func(tx *transaction.Repo) error {
		if err := tx.DeleteExpiredSessions(ctx, now); err != nil {
			return err
		}
		var err error
		token, expiresAt, err = s.createSession(ctx, tx, user.ID, now)
		return err
	})
	if err != nil {
		return models.AuthResult{}, err
	}

	return models.AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *UserService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.db.Repo().RevokeSession(ctx, hashToken(token), time.Now().UTC())
}

func (s *UserService) Authenticate(ctx context.Context, token string) (models.User, error) {
	if strings.TrimSpace(token) == "" {
		return models.User{}, models.Unauthorized("missing bearer token")
	}
	now := time.Now().UTC()
	repo := s.db.Repo()
	user, sessionID, err := repo.SessionUser(ctx, hashToken(token), now)
	if err != nil {
		return models.User{}, err
	}
	_ = repo.TouchSession(ctx, sessionID, now)
	return user, nil
}

func (s *UserService) Me(ctx context.Context, userID int64) (models.User, error) {
	return s.db.Repo().UserByID(ctx, userID)
}

func (s *UserService) Users(ctx context.Context) ([]models.User, error) {
	return s.db.Repo().Users(ctx)
}

func (s *UserService) Leaderboard(ctx context.Context) ([]models.LeaderboardRow, error) {
	rows, err := s.db.SQL().QueryContext(ctx, `
		SELECT
			u.id, u.username, u.role, u.points, u.created_at,
			(SELECT COUNT(*) FROM tasks t WHERE t.assignee_id = u.id AND t.task_state = 'completed') AS completed_tasks,
			(SELECT COUNT(*) FROM task_reviewers r WHERE r.user_id = u.id AND r.decision = 'approved') AS reviews
		FROM users u
		ORDER BY u.points DESC, completed_tasks DESC, u.username ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.LeaderboardRow{}
	for rows.Next() {
		var (
			row       models.LeaderboardRow
			createdAt string
		)
		if err := rows.Scan(
			&row.User.ID, &row.User.Username, &row.User.Role, &row.User.Points, &createdAt,
			&row.CompletedTasks, &row.Reviews,
		); err != nil {
			return nil, err
		}
		row.User.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		row.Rank = len(result) + 1
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *UserService) createSession(ctx context.Context, repo *transaction.Repo, userID int64, now time.Time) (string, time.Time, error) {
	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	sessionID, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := now.Add(s.sessionTTL)
	if err := repo.CreateSession(ctx, sessionID, userID, hashToken(token), now, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed")
}
