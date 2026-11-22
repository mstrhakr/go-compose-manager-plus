package auth

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"
)

func TestBootstrapIsOneTimeAndPersists(t *testing.T) {
	ctx := context.Background()
	dataDir := filepath.Join(t.TempDir(), "state")
	database, err := OpenDatabase(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	needsBootstrap, err := database.NeedsBootstrap(ctx)
	if err != nil || !needsBootstrap {
		t.Fatalf("NeedsBootstrap() = (%v, %v)", needsBootstrap, err)
	}
	user, err := database.ProvisionInitialAdmin(ctx, "admin", "argon2id-test-hash")
	if err != nil {
		t.Fatal(err)
	}
	if !user.MustChange || user.AuthVersion != 1 || user.ID == "admin" {
		t.Fatalf("unexpected provisioned user: %+v", user)
	}
	if err := database.SQL.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = OpenDatabase(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.SQL.Close() })
	needsBootstrap, err = database.NeedsBootstrap(ctx)
	if err != nil || needsBootstrap {
		t.Fatalf("NeedsBootstrap() after reopen = (%v, %v)", needsBootstrap, err)
	}
	loaded, err := database.UserByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != user.ID || loaded.PasswordHash != "argon2id-test-hash" || !loaded.MustChange {
		t.Fatalf("reopened user differs: %+v", loaded)
	}
	if _, err := database.ProvisionInitialAdmin(ctx, "attacker", "another-hash"); !errors.Is(err, ErrBootstrapComplete) {
		t.Fatalf("second bootstrap error = %v", err)
	}
	if info, err := os.Stat(filepath.Join(dataDir, "app.db")); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("database permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestOpenDatabaseRefusesUnversionedExistingTables(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "app.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE unexpected (value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDatabase(ctx, dataDir); err == nil {
		t.Fatal("expected unversioned existing tables to be refused")
	}
}

func TestOpenDatabaseMigratesV1Sessions(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := migrateV1(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions(token, data, expiry_unix) VALUES ('old', X'01', 100)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := OpenDatabase(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.SQL.Close() })
	var deadline, revoked, version int
	if err := database.SQL.QueryRowContext(ctx, `SELECT deadline_unix, revoked FROM sessions WHERE token = 'old'`).Scan(&deadline, &revoked); err != nil {
		t.Fatal(err)
	}
	if deadline != 43300 || revoked != 0 {
		t.Fatalf("migrated session deadline=%d revoked=%d", deadline, revoked)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil || version != 3 {
		t.Fatalf("migration version=%d err=%v", version, err)
	}
}

func TestOpenDatabaseMigratesV2WithoutChangingAuth(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := migrateV1(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := migrateV2(ctx, tx); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO schema_migrations(version) VALUES (1), (2)`,
		`UPDATE app_state SET value = '1' WHERE key = 'bootstrap_complete'`,
		`INSERT INTO users VALUES ('user-1', 'admin', 'existing-hash', 0, 1, 100)`,
		`INSERT INTO sessions(token, data, expiry_unix, deadline_unix, revoked) VALUES ('token', X'01', 200, 300, 0)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.SQL.Close() })
	user, err := database.UserByUsername(ctx, "admin")
	if err != nil || user.PasswordHash != "existing-hash" || user.MustChange {
		t.Fatalf("migrated user = %+v, error = %v", user, err)
	}
	var deadline, version int
	if err := database.SQL.QueryRowContext(ctx, `SELECT deadline_unix FROM sessions WHERE token = 'token'`).Scan(&deadline); err != nil || deadline != 300 {
		t.Fatalf("migrated session deadline=%d error=%v", deadline, err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil || version != 3 {
		t.Fatalf("migration version=%d error=%v", version, err)
	}
}
