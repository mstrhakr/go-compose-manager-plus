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
