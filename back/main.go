package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"rudy/internal/routes"
	"rudy/internal/services"
	"rudy/internal/transaction"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	loadEnvFile(logger)

	if err := run(logger); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// loadEnvFile reads RUDY_* configuration from a .env file when one is present.
// Variables already set in the process environment always win, so deployments
// and one-off overrides behave as expected. Set RUDY_ENV_FILE to use another
// path; the default is .env relative to the working directory.
func loadEnvFile(logger *slog.Logger) {
	path := env("RUDY_ENV_FILE", ".env")

	info, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Warn("could not inspect environment file", "path", path, "error", err)
		}
		return
	}
	if info.IsDir() {
		logger.Warn("environment file is a directory", "path", path)
		return
	}

	if err := godotenv.Load(path); err != nil {
		logger.Warn("could not load environment file", "path", path, "error", err)
		return
	}
	logger.Info("loaded environment file", "path", path)
}

func run(logger *slog.Logger) error {
	dbPath := env("RUDY_DB", "rudy.db")
	db, err := transaction.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	if err := db.Migrate(startupCtx); err != nil {
		return err
	}

	handler := routes.NewRouter(routes.Deps{
		Users:       services.NewUserService(db, sessionTTL()),
		Projects:    services.NewProjectService(db),
		Tasks:       services.NewTaskService(db),
		Stats:       services.NewStatsService(db),
		Preferences: services.NewPreferencesService(db),
		Logger:      logger,
	})

	srv := &http.Server{
		Addr:              listenAddr(),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", srv.Addr, "db", dbPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// listenAddr builds the HTTP bind address from the environment. RUDY_ADDR is a
// full override ("127.0.0.1:9000", "[::1]:9000") or a bare port ("9000").
// Otherwise RUDY_HOST and RUDY_PORT are combined. RUDY_PORT falls back to the
// platform-standard PORT, so hosts that inject a port at deploy time work
// without extra configuration.
func listenAddr() string {
	if addr := strings.TrimSpace(os.Getenv("RUDY_ADDR")); addr != "" {
		if isPort(addr) {
			return net.JoinHostPort(bindHost(), addr)
		}
		return addr
	}
	return net.JoinHostPort(bindHost(), bindPort())
}

func bindHost() string {
	if host := strings.TrimSpace(os.Getenv("RUDY_HOST")); host != "" {
		return host
	}
	return "0.0.0.0"
}

func bindPort() string {
	if port := strings.TrimSpace(os.Getenv("RUDY_PORT")); port != "" {
		return port
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return port
	}
	return "8080"
}

func isPort(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func sessionTTL() time.Duration {
	raw := os.Getenv("RUDY_SESSION_TTL_HOURS")
	if raw == "" {
		return 30 * 24 * time.Hour
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		return 30 * 24 * time.Hour
	}
	return time.Duration(hours) * time.Hour
}
