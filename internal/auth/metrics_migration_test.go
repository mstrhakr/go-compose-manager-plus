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
		`DROP TABLE metrics_settings`,
		`DELETE FROM schema_migrations WHERE version = 4`,
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
