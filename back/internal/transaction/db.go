package transaction

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

//go:embed migrations
var migrationFS embed.FS

type rebindDB struct {
	*sql.DB
	driver string
}

type rebindTx struct {
	*sql.Tx
	driver string
}

type DB struct {
	sql    *rebindDB
	driver string
}

func (d *DB) Driver() string {
	return d.driver
}

func (r *rebindDB) rebindQuery(query string) string {
	if r.driver != "pgx" {
		return query
	}
	return rebind(query)
}

func (r *rebindDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.DB.ExecContext(ctx, r.rebindQuery(query), args...)
}

func (r *rebindDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.DB.QueryContext(ctx, r.rebindQuery(query), args...)
}

func (r *rebindDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return r.DB.QueryRowContext(ctx, r.rebindQuery(query), args...)
}

func (r *rebindDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return r.DB.BeginTx(ctx, opts)
}

func (t *rebindTx) rebindQuery(query string) string {
	if t.driver != "pgx" {
		return query
	}
	return rebind(query)
}

func (t *rebindTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, t.rebindQuery(query), args...)
}

func (t *rebindTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, t.rebindQuery(query), args...)
}

func (t *rebindTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.Tx.QueryRowContext(ctx, t.rebindQuery(query), args...)
}

func rebind(query string) string {
	var b strings.Builder
	inSingle := false
	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		if inSingle {
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(query) && query[i+1] == '\'' {
					b.WriteByte(query[i+1])
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if c == '\'' {
			inSingle = true
			b.WriteByte(c)
			continue
		}
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// Open opens either a PostgreSQL database (when dsn starts with postgres:// or
// postgresql://) or a SQLite file (otherwise).
func Open(dsn string) (*DB, error) {
	driver := "sqlite"
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		driver = "pgx"
	} else {
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		dsn += separator + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	if driver == "sqlite" {
		// SQLite only supports one writer. A single connection keeps transactions
		// predictable and avoids lock contention in this small service.
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &DB{sql: &rebindDB{DB: db, driver: driver}, driver: driver}, nil
}

func (d *DB) Close() error {
	return d.sql.DB.Close()
}

func (d *DB) SQL() *rebindDB {
	return d.sql
}

type migration struct {
	version    int
	name       string
	statements []string
}

// Migrate applies every embedded migration that has not been recorded yet.
// Migrations are ordered by their numeric filename prefix and each one runs in
// its own transaction.
func (d *DB) Migrate(ctx context.Context) error {
	if _, err := d.sql.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	rows, err := d.sql.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return err
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	// Close before starting the migration transactions: the pool has a single
	// connection, so an open result set would block BeginTx forever.
	rows.Close()

	migrations, err := loadMigrations(d.driver)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := d.applyMigration(ctx, m); err != nil {
			return fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
		}
	}
	return nil
}

func (d *DB) applyMigration(ctx context.Context, m migration) error {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	wrapped := &rebindTx{Tx: tx, driver: d.driver}
	defer func() { _ = tx.Rollback() }()

	for _, statement := range m.statements {
		if _, err := wrapped.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("%s: %w", statement, err)
		}
	}
	if _, err := wrapped.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
		m.version, m.name,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func loadMigrations(driver string) ([]migration, error) {
	dir := "migrations"
	if driver == "pgx" {
		dir = "migrations/postgres"
	}
	entries, err := fs.ReadDir(migrationFS, dir)
	if err != nil {
		return nil, err
	}

	result := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, name, err := parseMigrationName(entry.Name())
		if err != nil {
			return nil, err
		}
		body, err := migrationFS.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			return nil, err
		}
		result = append(result, migration{
			version:    version,
			name:       name,
			statements: splitStatements(string(body)),
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	return result, nil
}

func parseMigrationName(filename string) (int, string, error) {
	base := strings.TrimSuffix(filename, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("migration %q must be named <version>_<name>.sql", filename)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil || version <= 0 {
		return 0, "", fmt.Errorf("migration %q has an invalid version", filename)
	}
	return version, parts[1], nil
}

// splitStatements is intentionally simple: this project has no semicolons
// inside SQL string literals. Line comments are stripped before splitting.
func splitStatements(body string) []string {
	var cleaned strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		cleaned.WriteString(line)
		cleaned.WriteByte('\n')
	}

	parts := strings.Split(cleaned.String(), ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements
}
