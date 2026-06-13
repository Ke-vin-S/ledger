package repository_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testPool is the shared connection pool for the package's integration tests.
// It is nil when integration tests are skipped (short mode), in which case
// requireDB skips the calling test.
var testPool *pgxpool.Pool

// TestMain spins up a single Postgres container for the whole package, applies
// the real migrations, and shares one pool across all integration tests.
// In -short mode it starts nothing, leaving testPool nil so every test skips.
func TestMain(m *testing.M) {
	// Parse flags so testing.Short() reflects the -short flag before we decide
	// whether to start the container.
	flag.Parse()

	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("splitleger_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests: could not start postgres container: %v\n", err)
		os.Exit(1)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests: connection string: %v\n", err)
		os.Exit(1)
	}

	if err := applyMigrations(connStr); err != nil {
		fmt.Fprintf(os.Stderr, "integration tests: migrate: %v\n", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests: pool: %v\n", err)
		os.Exit(1)
	}
	testPool = pool

	code := m.Run()

	pool.Close()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

// applyMigrations runs the project's golang-migrate migrations against the test DB.
func applyMigrations(connStr string) error {
	_, thisFile, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "db", "migrations")
	m, err := migrate.New("file://"+migrationsDir, connStr)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// requireDB returns the shared pool, skipping the test when integration tests
// are disabled (short mode / no Docker).
func requireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip("skipping integration test (short mode or no database)")
	}
	return testPool
}

// truncateAll clears mutable tables between tests so each starts from a clean slate.
// audit_log is immutable (DB triggers block DELETE), so it is removed via TRUNCATE,
// which the triggers do not intercept.
func truncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE audit_log, notifications, notification_prefs, expense_flags,
		         settlements, expense_splits, expense_versions, expenses,
		         invite_links, team_members, teams, claim_tokens, oauth_accounts,
		         loan_repayments, loans, users
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// ── seed helpers ────────────────────────────────────────────────────────────────

// seedUser inserts a registered user and returns its ID.
func seedUser(t *testing.T, pool *pgxpool.Pool, displayName string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, identity_type, display_name) VALUES ($1, 'registered', $2)`,
		id, displayName)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

// seedTeam inserts a team owned by owner and an active owner membership, returning the team ID.
func seedTeam(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO teams (id, name, currency, owner_id, created_by) VALUES ($1, 'Trip', 'LKR', $2, $2)`,
		id, owner); err != nil {
		t.Fatalf("seed team: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role, status) VALUES ($1, $2, 'owner', 'active')`,
		id, owner); err != nil {
		t.Fatalf("seed owner membership: %v", err)
	}
	return id
}
