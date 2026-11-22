package discovery

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
)

func TestStorePreservesStateAcrossScansAndRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	path := filepath.Join(root, "project")
	db, err := auth.OpenDatabase(ctx, filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db.SQL)
	if err := store.SetRoot(ctx, root); err != nil {
		t.Fatal(err)
	}
	settings, err := store.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyScan(ctx, settings, []Project{{Path: path, Root: root}}); err != nil {
		t.Fatal(err)
	}
	settings, err = store.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Activate(ctx, settings, path); err != nil {
		t.Fatal(err)
	}
	settings, err = store.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyScan(ctx, settings, nil); err != nil {
		t.Fatal(err)
	}
	projects, err := store.Projects(ctx)
	if err != nil || len(projects) != 1 || !projects[0].Active || projects[0].Available {
		t.Fatalf("absent project = %+v, error %v", projects, err)
	}
	if err := db.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = auth.OpenDatabase(ctx, filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.SQL.Close() })
	store = NewStore(db.SQL)
	settings, err = store.Settings(ctx)
	if err != nil || settings.Root != root {
		t.Fatalf("settings = %+v, error %v", settings, err)
	}
	if err := store.ApplyScan(ctx, settings, []Project{{Path: path, Root: root}}); err != nil {
		t.Fatal(err)
	}
	projects, err = store.Projects(ctx)
	if err != nil || len(projects) != 1 || !projects[0].Active || !projects[0].Available {
		t.Fatalf("restored project = %+v, error %v", projects, err)
	}
	if err := store.SetRoot(ctx, filepath.Join(dir, "other")); err != nil {
		t.Fatal(err)
	}
	projects, err = store.Projects(ctx)
	if err != nil || len(projects) != 1 || !projects[0].Active {
		t.Fatalf("root change lost project: %+v, error %v", projects, err)
	}
}

func TestStoreRejectsStaleScansAndInvalidActivation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	path := filepath.Join(root, "project")
	db, err := auth.OpenDatabase(ctx, filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.SQL.Close() })
	store := NewStore(db.SQL)
	if err := store.SetRoot(ctx, root); err != nil {
		t.Fatal(err)
	}
	settings, err := store.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyScan(ctx, settings, []Project{{Path: path, Root: root, ResolutionError: "invalid input"}}); err != nil {
		t.Fatal(err)
	}
	current, err := store.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Activate(ctx, current, path); err == nil {
		t.Fatal("invalid project was activated")
	}
	if err := store.ApplyScan(ctx, settings, nil); !errors.Is(err, ErrStaleScan) {
		t.Fatalf("stale scan error = %v", err)
	}
	settings, err = store.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetRoot(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Activate(ctx, current, path); err == nil {
		t.Fatal("activation with stale scan revision succeeded")
	}
	if err := store.ApplyScan(ctx, settings, nil); !errors.Is(err, ErrStaleScan) {
		t.Fatalf("stale scan after root update = %v", err)
	}
	projects, err := store.Projects(ctx)
	if err != nil || len(projects) != 1 || !projects[0].Available || projects[0].Active {
		t.Fatalf("stale scan changed projects = %+v, error %v", projects, err)
	}
}
