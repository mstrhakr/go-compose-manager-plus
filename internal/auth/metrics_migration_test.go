package auth

import (
	"context"
	"testing"
)

func TestOpenDatabaseMigratesV3MetricsSettings(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO discovered_projects(path, root, active, available, resolution_error) VALUES ('/root/a', '/root', 1, 1, '')`,
		`DROP TABLE state_settings`,
		`DROP TABLE log_settings`,
		`DROP TABLE collection_settings`,
		`DROP TABLE metrics_settings`,
		`DELETE FROM schema_migrations WHERE version >= 4`,
	} {
		if _, err := db.SQL.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	var count int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM metrics_settings`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("migrated settings count=%d error=%v", count, err)
	}
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM discovered_projects WHERE path = '/root/a' AND active = 1`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("migrated projects count=%d error=%v", count, err)
	}
	if _, err := db.SQL.ExecContext(ctx, `INSERT INTO metrics_settings VALUES (1, 1, 'high', 'all', 1000000000, 1000000, 1)`); err != nil {
		t.Fatalf("write migrated settings: %v", err)
	}
}

func TestOpenDatabasePreservesLegacyCaptureEnablementWithoutEnablingLogs(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.ExecContext(ctx, `INSERT INTO metrics_settings
		(id, enabled, profile, scope, interval_ns, storage_limit_bytes, revision)
		VALUES (1, 1, 'high', 'all', 1000000000, 7654321, 4)`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`DROP TABLE state_settings`,
		`DROP TABLE log_settings`,
		`DROP TABLE collection_settings`,
		`DELETE FROM schema_migrations WHERE version >= 5`,
	} {
		if _, err := db.SQL.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	var enabled int
	var scope string
	var revision int64
	if err := db.SQL.QueryRowContext(ctx, `SELECT enabled, scope, revision FROM state_settings WHERE id = 1`).
		Scan(&enabled, &scope, &revision); err != nil || enabled != 1 || scope != "all" || revision != 1 {
		t.Fatalf("migrated state source enabled=%d scope=%q revision=%d error=%v", enabled, scope, revision, err)
	}
	var configured, logEnabled int
	var logStorage int64
	if err := db.SQL.QueryRowContext(ctx, `SELECT configured, enabled, storage_limit_bytes FROM log_settings WHERE id = 1`).
		Scan(&configured, &logEnabled, &logStorage); err != nil || configured != 0 || logEnabled != 0 || logStorage != 256000000 {
		t.Fatalf("migrated log defaults configured=%d enabled=%d storage=%d error=%v", configured, logEnabled, logStorage, err)
	}
	var storageLimit int64
	if err := db.SQL.QueryRowContext(ctx, `SELECT storage_limit_bytes FROM metrics_settings WHERE id = 1`).
		Scan(&storageLimit); err != nil || storageLimit != 7654321 {
		t.Fatalf("migrated metrics storage target=%d error=%v", storageLimit, err)
	}
}
