package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"rudy/internal/models"
	"rudy/internal/storage"
	"rudy/internal/transaction"
)

type testEnv struct {
	db          *transaction.DB
	users       *UserService
	projects    *ProjectService
	tasks       *TaskService
	stats       *StatsService
	preferences *PreferencesService
	chat        *ChatService
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := transaction.Open(filepath.Join(t.TempDir(), "rudy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store, err := storage.NewLocal(filepath.Join(t.TempDir(), "uploads"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	return &testEnv{
		db:          db,
		users:       NewUserService(db, time.Hour),
		projects:    NewProjectService(db),
		tasks:       NewTaskService(db),
		stats:       NewStatsService(db),
		preferences: NewPreferencesService(db),
		chat:        NewChatService(db, store, nil),
	}
}

func (e *testEnv) register(t *testing.T, username string, role models.Role) models.AuthResult {
	t.Helper()
	result, err := e.users.Register(context.Background(), username, "password123", role)
	if err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	return result
}

func (e *testEnv) project(t *testing.T, creator models.User) models.Project {
	t.Helper()
	project, err := e.projects.Create(context.Background(), creator, "Aurora", "A test project", 1440)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project
}

func TestMigrateIsIdempotent(t *testing.T) {
	env := newTestEnv(t)
	if err := env.db.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestRegisterLoginAndSessionRevocation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	registered := env.register(t, "alice", models.FrontendEngineer)
	if registered.Token == "" {
		t.Fatal("expected a session token")
	}

	user, err := env.users.Authenticate(ctx, registered.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("got user %q", user.Username)
	}

	login, err := env.users.Login(ctx, "ALICE", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.Token == "" || login.Token == registered.Token {
		t.Fatal("expected a fresh session token")
	}

	if _, err := env.users.Login(ctx, "alice", "wrong-password"); err == nil {
		t.Fatal("expected invalid credentials")
	}

	if err := env.users.Logout(ctx, login.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := env.users.Authenticate(ctx, login.Token); err == nil {
		t.Fatal("expected revoked token to be rejected")
	}
	if _, err := env.users.Authenticate(ctx, registered.Token); err != nil {
		t.Fatalf("other session should still work: %v", err)
	}
}

func TestGatesRequireApprovalFirst(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	manager := env.register(t, "nova", models.Lead).User
	engineer := env.register(t, "ari", models.FrontendEngineer).User
	project := env.project(t, manager)
	if _, err := env.projects.AddMember(ctx, project.ID, manager.ID, engineer.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}

	task, err := env.tasks.Create(ctx, project.ID, engineer, "Build the checkout", models.Feature, models.FrontendEngineer, 20, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// The manager cannot record acceptance gates on a proposal either.
	if _, err := env.tasks.SetGate(ctx, task.ID, "code", "done", manager); err == nil {
		t.Fatal("expected a gate update to fail before approval")
	}
	if _, err := env.tasks.Assign(ctx, task.ID, engineer.ID, manager); err == nil {
		t.Fatal("expected assignment to fail before approval")
	}

	if _, err := env.tasks.Approve(ctx, task.ID, manager); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := env.tasks.Assign(ctx, task.ID, engineer.ID, manager); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if _, err := env.tasks.SetGate(ctx, task.ID, "code", "shipped", engineer); err != nil {
		t.Fatalf("set gate after approval: %v", err)
	}
}

func TestAssigneeCannotReviewOwnWork(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	manager := env.register(t, "nova", models.Lead).User
	engineer := env.register(t, "ari", models.FrontendEngineer).User
	reviewer := env.register(t, "mira", models.QualityAssuranceE).User
	project := env.project(t, manager)
	if _, err := env.projects.AddMember(ctx, project.ID, manager.ID, engineer.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}

	task, err := env.tasks.Create(ctx, project.ID, engineer, "Build the checkout", models.Feature, models.FrontendEngineer, 20, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if _, err := env.tasks.Approve(ctx, task.ID, manager); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := env.tasks.Assign(ctx, task.ID, engineer.ID, manager); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if _, err := env.tasks.AddReviewer(ctx, task.ID, engineer.ID, engineer); err == nil {
		t.Fatal("expected self-review to be rejected")
	}

	if _, err := env.tasks.SetGate(ctx, task.ID, "code", "shipped", engineer); err != nil {
		t.Fatalf("code gate: %v", err)
	}
	if _, err := env.tasks.SetGate(ctx, task.ID, "tests", "green", engineer); err != nil {
		t.Fatalf("tests gate: %v", err)
	}
	if _, err := env.tasks.SetGate(ctx, task.ID, "documentation", "written", engineer); err != nil {
		t.Fatalf("documentation gate: %v", err)
	}
	if _, err := env.tasks.AddReviewer(ctx, task.ID, reviewer.ID, engineer); err != nil {
		t.Fatalf("add reviewer: %v", err)
	}
	if _, err := env.tasks.Submit(ctx, task.ID, engineer); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.tasks.Review(ctx, task.ID, engineer, models.ReviewApproved, ""); err == nil {
		t.Fatal("expected the assignee to be unable to submit a review")
	}
}

func TestCompletionIsPaidOnceAndMovesPoints(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	manager := env.register(t, "nova", models.Lead).User
	engineer := env.register(t, "ari", models.FrontendEngineer).User
	reviewer := env.register(t, "mira", models.QualityAssuranceE).User
	project := env.project(t, manager)
	if _, err := env.projects.AddMember(ctx, project.ID, manager.ID, engineer.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}

	task, err := env.tasks.Create(ctx, project.ID, engineer, "Build the checkout", models.Feature, models.FrontendEngineer, 30, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if _, err := env.tasks.Approve(ctx, task.ID, manager); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := env.tasks.Assign(ctx, task.ID, engineer.ID, manager); err != nil {
		t.Fatalf("assign: %v", err)
	}
	for gate, value := range map[string]string{"code": "shipped", "tests": "green", "documentation": "written"} {
		if _, err := env.tasks.SetGate(ctx, task.ID, gate, value, engineer); err != nil {
			t.Fatalf("gate %s: %v", gate, err)
		}
	}
	if _, err := env.tasks.AddReviewer(ctx, task.ID, reviewer.ID, engineer); err != nil {
		t.Fatalf("add reviewer: %v", err)
	}
	if _, err := env.tasks.Submit(ctx, task.ID, engineer); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.tasks.Review(ctx, task.ID, reviewer, models.ReviewApproved, "looks good"); err != nil {
		t.Fatalf("review: %v", err)
	}

	completed, err := env.tasks.Complete(ctx, task.ID, manager)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.Task.TaskState != models.Completed {
		t.Fatalf("expected completed, got %q", completed.Task.TaskState)
	}
	if len(completed.Awards) != 2 {
		t.Fatalf("expected assignee and reviewer awards, got %d", len(completed.Awards))
	}

	engineerAfter, err := env.users.Me(ctx, engineer.ID)
	if err != nil {
		t.Fatalf("engineer: %v", err)
	}
	reviewerAfter, err := env.users.Me(ctx, reviewer.ID)
	if err != nil {
		t.Fatalf("reviewer: %v", err)
	}
	if engineerAfter.Points != 30 {
		t.Fatalf("engineer points = %d, want 30", engineerAfter.Points)
	}
	if reviewerAfter.Points != models.ReviewBonus(30) {
		t.Fatalf("reviewer points = %d, want %d", reviewerAfter.Points, models.ReviewBonus(30))
	}
	if managerAfter, err := env.users.Me(ctx, manager.ID); err != nil {
		t.Fatalf("manager: %v", err)
	} else if managerAfter.Points != 100 {
		t.Fatalf("manager points = %d, want 100", managerAfter.Points)
	}

	if _, err := env.tasks.Complete(ctx, task.ID, manager); err == nil {
		t.Fatal("expected a second completion to be rejected")
	}
}

func TestStatsAndPreferences(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	user := env.register(t, "alice", models.FrontendEngineer).User
	env.register(t, "nova", models.Lead)
	project := env.project(t, env.register(t, "nora", models.Lead).User)

	if _, err := env.tasks.Create(ctx, project.ID, user, "Build the checkout", models.Feature, models.FrontendEngineer, 15, nil); err != nil {
		t.Fatalf("create task: %v", err)
	}

	stats, err := env.stats.Stats(ctx, "30d")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Totals.Users != 3 {
		t.Fatalf("users = %d, want 3", stats.Totals.Users)
	}
	if stats.Totals.Projects != 1 || stats.Totals.Tasks != 1 {
		t.Fatalf("projects/tasks = %d/%d, want 1/1", stats.Totals.Projects, stats.Totals.Tasks)
	}
	if stats.TasksByState["pending"] != 1 {
		t.Fatalf("pending tasks = %d, want 1", stats.TasksByState["pending"])
	}

	defaults, err := env.preferences.Get(ctx, user.ID)
	if err != nil {
		t.Fatalf("get preferences: %v", err)
	}
	if defaults.ThemeMode != models.ThemeDark {
		t.Fatalf("default theme = %q", defaults.ThemeMode)
	}

	updated, err := env.preferences.Update(ctx, user.ID, models.Preferences{
		ThemeMode: models.ThemeLight,
		Accent:    "#112233",
		BlendA:    "#445566",
		BlendB:    "#778899",
	})
	if err != nil {
		t.Fatalf("update preferences: %v", err)
	}
	if updated.ThemeMode != models.ThemeLight || updated.Accent != "#112233" {
		t.Fatalf("unexpected preferences: %+v", updated)
	}

	if _, err := env.preferences.Update(ctx, user.ID, models.Preferences{
		ThemeMode: models.ThemeLight,
		Accent:    "red",
		BlendA:    "#445566",
		BlendB:    "#778899",
	}); err == nil {
		t.Fatal("expected an invalid color to be rejected")
	}
}

func TestGoogleAuthCreatesLinksAndGuardsNewAccounts(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.users.SetGoogleClientID("rudy-test-client")

	env.users.SetGoogleValidator(func(_ context.Context, credential, clientID string) (GoogleIdentity, error) {
		if clientID != "rudy-test-client" {
			t.Fatalf("unexpected client id %q", clientID)
		}
		if credential != "verified" {
			return GoogleIdentity{}, errors.New("invalid credential")
		}
		return GoogleIdentity{
			Subject:       "google-grace",
			Email:         "Grace@Example.com",
			Name:          "Grace Hopper",
			EmailVerified: true,
		}, nil
	})

	if _, err := env.users.GoogleAuth(ctx, "verified", nil); err == nil {
		t.Fatal("expected a new Google account to require a role")
	}

	role := models.FrontendEngineer
	created, err := env.users.GoogleAuth(ctx, "verified", &role)
	if err != nil {
		t.Fatalf("google create: %v", err)
	}
	if created.User.Email != "grace@example.com" {
		t.Fatalf("email = %q", created.User.Email)
	}
	if created.User.Username != "grace" {
		t.Fatalf("username = %q", created.User.Username)
	}
	if created.Token == "" {
		t.Fatal("expected a session token")
	}
	if _, err := env.users.Authenticate(ctx, created.Token); err != nil {
		t.Fatalf("google session should authenticate: %v", err)
	}

	second, err := env.users.GoogleAuth(ctx, "verified", nil)
	if err != nil {
		t.Fatalf("google sign-in: %v", err)
	}
	if second.User.ID != created.User.ID {
		t.Fatalf("expected the same account, got %d and %d", created.User.ID, second.User.ID)
	}

	if _, err := env.users.Login(ctx, created.User.Username, "password123"); err == nil {
		t.Fatal("Google-only accounts must not accept password login")
	}
	if _, err := env.users.GoogleAuth(ctx, "forged", nil); err == nil {
		t.Fatal("expected an invalid Google credential to be rejected")
	}
}

func TestGoogleAuthLinksAnExistingEmailAccount(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.users.SetGoogleClientID("rudy-test-client")

	local := env.register(t, "ada", models.Lead).User
	if _, err := env.db.SQL().ExecContext(ctx, `UPDATE users SET email = ? WHERE id = ?`, "ada@example.com", local.ID); err != nil {
		t.Fatalf("seed email: %v", err)
	}

	env.users.SetGoogleValidator(func(_ context.Context, _, _ string) (GoogleIdentity, error) {
		return GoogleIdentity{
			Subject:       "google-ada",
			Email:         "ADA@example.com",
			Name:          "Ada Lovelace",
			EmailVerified: true,
		}, nil
	})

	linked, err := env.users.GoogleAuth(ctx, "verified", nil)
	if err != nil {
		t.Fatalf("google link: %v", err)
	}
	if linked.User.ID != local.ID {
		t.Fatalf("expected local account %d, got %d", local.ID, linked.User.ID)
	}
	if linked.User.Username != "ada" {
		t.Fatalf("username = %q", linked.User.Username)
	}

	again, err := env.users.GoogleAuth(ctx, "verified", nil)
	if err != nil {
		t.Fatalf("second google sign-in: %v", err)
	}
	if again.User.ID != local.ID {
		t.Fatalf("expected linked account on repeat sign-in")
	}
}
