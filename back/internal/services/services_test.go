package services

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"rudy/internal/models"
	"rudy/internal/transaction"
)

type testEnv struct {
	db          *transaction.DB
	users       *UserService
	projects    *ProjectService
	tasks       *TaskService
	stats       *StatsService
	preferences *PreferencesService
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

	return &testEnv{
		db:          db,
		users:       NewUserService(db, time.Hour),
		projects:    NewProjectService(db),
		tasks:       NewTaskService(db),
		stats:       NewStatsService(db),
		preferences: NewPreferencesService(db),
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

	manager := env.register(t, "nova", models.ProjectManager).User
	engineer := env.register(t, "ari", models.FrontendEngineer).User
	project := env.project(t, manager)

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

	manager := env.register(t, "nova", models.ProjectManager).User
	engineer := env.register(t, "ari", models.FrontendEngineer).User
	reviewer := env.register(t, "mira", models.QualityAssuranceE).User
	project := env.project(t, manager)

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

	manager := env.register(t, "nova", models.ProjectManager).User
	engineer := env.register(t, "ari", models.FrontendEngineer).User
	reviewer := env.register(t, "mira", models.QualityAssuranceE).User
	project := env.project(t, manager)

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
	} else if managerAfter.Points != 0 {
		t.Fatalf("manager points = %d, want 0", managerAfter.Points)
	}

	if _, err := env.tasks.Complete(ctx, task.ID, manager); err == nil {
		t.Fatal("expected a second completion to be rejected")
	}
}

func TestStatsAndPreferences(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	user := env.register(t, "alice", models.FrontendEngineer).User
	env.register(t, "nova", models.ProjectManager)
	project := env.project(t, env.register(t, "nora", models.ProjectManager).User)

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
