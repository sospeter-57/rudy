package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"

	"rudy/internal/models"
	"rudy/internal/transaction"
)

const (
	minUsernameRunes = 3
	maxUsernameRunes = 24
	minPasswordRunes = 8
)

// GoogleIdentity is the verified profile data Rudy needs from a Google ID token.
type GoogleIdentity struct {
	Subject       string
	Email         string
	Name          string
	EmailVerified bool
}

// GoogleValidator verifies a Google ID token for a client. Tests replace it
// with a stub so the auth flow can run without network access.
type GoogleValidator func(ctx context.Context, credential, clientID string) (GoogleIdentity, error)

type UserService struct {
	db              *transaction.DB
	sessionTTL      time.Duration
	bcryptCost      int
	googleClientID  string
	googleValidator GoogleValidator
}

func NewUserService(db *transaction.DB, sessionTTL time.Duration) *UserService {
	if sessionTTL <= 0 {
		sessionTTL = 30 * 24 * time.Hour
	}
	return &UserService{
		db:              db,
		sessionTTL:      sessionTTL,
		bcryptCost:      bcrypt.DefaultCost,
		googleValidator: validateGoogleCredential,
	}
}

// SetGoogleClientID enables Google sign-in for the given OAuth client. An empty
// value leaves the endpoint disabled instead of accepting unverified tokens.
func (s *UserService) SetGoogleClientID(clientID string) {
	s.googleClientID = strings.TrimSpace(clientID)
}

func (s *UserService) SetGoogleValidator(validator GoogleValidator) {
	if validator != nil {
		s.googleValidator = validator
	}
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

// GoogleAuth exchanges a Google ID token for a Rudy session. Existing accounts
// are matched by Google subject first, then by verified email so a password
// account can be linked without creating a duplicate. When no account exists a
// role is required: the client chooses one during "Create profile".
func (s *UserService) GoogleAuth(ctx context.Context, credential string, role *models.Role) (models.AuthResult, error) {
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return models.AuthResult{}, models.Validation("a Google credential is required")
	}
	if s.googleClientID == "" {
		return models.AuthResult{}, models.Validation("Google sign-in is not configured")
	}
	if s.googleValidator == nil {
		return models.AuthResult{}, models.NewError(models.KindInternal, "Google sign-in is unavailable")
	}

	identity, err := s.googleValidator(ctx, credential, s.googleClientID)
	if err != nil {
		return models.AuthResult{}, models.Unauthorized("Google sign-in could not be verified")
	}
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if identity.Subject == "" || email == "" || !identity.EmailVerified {
		return models.AuthResult{}, models.Unauthorized("Google account is missing a verified email")
	}

	now := time.Now().UTC()
	var result models.AuthResult
	err = s.db.WithTx(ctx, func(repo *transaction.Repo) error {
		user, err := repo.UserByGoogleSub(ctx, identity.Subject)
		if err == nil {
			result, err = s.sessionForUser(ctx, repo, user, now)
			return err
		}
		if !isNotFound(err) {
			return err
		}

		user, err = repo.UserByEmail(ctx, email)
		if err == nil {
			if err := repo.LinkGoogleAccount(ctx, user.ID, email, identity.Subject); err != nil {
				return err
			}
			user.Email = email
			result, err = s.sessionForUser(ctx, repo, user, now)
			return err
		}
		if !isNotFound(err) {
			return err
		}

		if role == nil {
			return models.Validation("no Rudy account uses this Google email; create a profile and choose a role first")
		}
		if !models.IsRole(*role) {
			return models.Validation("role is not a known role")
		}

		created, err := s.createGoogleUser(ctx, repo, identity, email, *role, now)
		if err != nil {
			return err
		}
		result, err = s.sessionForUser(ctx, repo, created, now)
		return err
	})
	if err != nil {
		return models.AuthResult{}, err
	}
	return result, nil
}

func (s *UserService) createGoogleUser(ctx context.Context, repo *transaction.Repo, identity GoogleIdentity, email string, role models.Role, now time.Time) (models.User, error) {
	base := googleUsername(identity.Name, email)

	for attempt := 0; attempt < 25; attempt++ {
		username := base
		if attempt > 0 {
			username = usernameWithSuffix(base, attempt+1)
		}

		user, err := repo.CreateGoogleUser(ctx, username, email, identity.Subject, unusablePasswordHash(), role, now)
		if err == nil {
			return user, nil
		}
		if !isUniqueViolation(err) {
			return models.User{}, err
		}

		// A concurrent request may have linked this identity already. Reuse it
		// instead of handing the caller an avoidable error.
		if existing, lookupErr := repo.UserByGoogleSub(ctx, identity.Subject); lookupErr == nil {
			return existing, nil
		}
		if existing, lookupErr := repo.UserByEmail(ctx, email); lookupErr == nil {
			return existing, nil
		}
	}

	return models.User{}, models.Conflict("could not allocate a username for this Google account")
}

func (s *UserService) sessionForUser(ctx context.Context, repo *transaction.Repo, user models.User, now time.Time) (models.AuthResult, error) {
	if err := repo.DeleteExpiredSessions(ctx, now); err != nil {
		return models.AuthResult{}, err
	}
	token, expiresAt, err := s.createSession(ctx, repo, user.ID, now)
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
			u.id, u.username, u.email, u.role, u.points, u.created_at,
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
			email     sql.NullString
			createdAt string
		)
		if err := rows.Scan(
			&row.User.ID, &row.User.Username, &email, &row.User.Role, &row.User.Points, &createdAt,
			&row.CompletedTasks, &row.Reviews,
		); err != nil {
			return nil, err
		}
		if email.Valid {
			row.User.Email = email.String
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

func isNotFound(err error) bool {
	var appErr *models.AppError
	return errors.As(err, &appErr) && appErr.Kind == models.KindNotFound
}

// validateGoogleCredential is the production Google ID-token verifier. It
// checks the signature, issuer, audience and expiry through Google's library.
func validateGoogleCredential(ctx context.Context, credential, clientID string) (GoogleIdentity, error) {
	payload, err := idtoken.Validate(ctx, credential, clientID)
	if err != nil {
		return GoogleIdentity{}, err
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	verified, _ := payload.Claims["email_verified"].(bool)

	return GoogleIdentity{
		Subject:       payload.Subject,
		Email:         email,
		Name:          name,
		EmailVerified: verified,
	}, nil
}

// unusablePasswordHash is deliberately not a bcrypt hash, so password login can
// never succeed for a Google-only account.
func unusablePasswordHash() string {
	token, err := randomToken()
	if err != nil {
		return "google-sign-in-only"
	}
	return "google-sign-in-only:" + token
}

func googleUsername(name, email string) string {
	base := strings.ToLower(strings.TrimSpace(email))
	if idx := strings.Index(base, "@"); idx > 0 {
		base = base[:idx]
	}
	base = sanitizeUsername(base)
	if utf8.RuneCountInString(base) < minUsernameRunes {
		base = sanitizeUsername(strings.ToLower(name))
	}
	if utf8.RuneCountInString(base) < minUsernameRunes {
		base = "builder"
	}
	return truncateRunes(base, maxUsernameRunes)
}

func sanitizeUsername(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			builder.WriteRune(r)
		}
	}
	return strings.Trim(builder.String(), "._-")
}

func usernameWithSuffix(base string, n int) string {
	suffix := "-" + strconv.Itoa(n)
	limit := maxUsernameRunes - utf8.RuneCountInString(suffix)
	if limit < minUsernameRunes {
		limit = minUsernameRunes
	}
	return truncateRunes(base, limit) + suffix
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
