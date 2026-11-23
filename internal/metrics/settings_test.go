package metrics

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
)

func TestSettingsDefaultOffAndPersist(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := auth.OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	settings := NewSettingsStore(db.SQL)
	loaded, err := settings.Load(ctx)
	if err != nil || loaded != (Settings{}) {
		t.Fatalf("default settings=%#v error=%v", loaded, err)
	}
	initial := Settings{Enabled: true, Profile: ProfileLow, Scope: ScopeActivated, Interval: 15 * time.Second, StorageLimit: 123456}
	saved, err := settings.Save(ctx, initial)
	if err != nil || saved.Revision != 1 {
		t.Fatalf("saved=%#v error=%v", saved, err)
	}
	if _, err := settings.Save(ctx, initial); !errors.Is(err, ErrStaleSettings) {
		t.Fatalf("stale first save=%v", err)
	}
	saved.Enabled = false
	updated, err := settings.Save(ctx, saved)
	if err != nil || updated.Revision != 2 || updated.Enabled {
		t.Fatalf("disabled=%#v error=%v", updated, err)
	}
	if _, err := settings.Save(ctx, saved); !errors.Is(err, ErrStaleSettings) {
		t.Fatalf("stale update=%v", err)
	}
	if err := db.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = auth.OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	loaded, err = NewSettingsStore(db.SQL).Load(ctx)
	if err != nil || !reflect.DeepEqual(loaded, updated) {
		t.Fatalf("reopened=%#v error=%v", loaded, err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "metrics.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("saving settings touched metrics database: %v", err)
	}
}

func TestSettingsValidation(t *testing.T) {
	ctx := context.Background()
	db, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	store := NewSettingsStore(db.SQL)
	for _, tt := range []struct {
		name string
		set  Settings
		ok   bool
	}{
		{"low", Settings{Profile: ProfileLow, Scope: ScopeActivated, Interval: 15 * time.Second, StorageLimit: 1}, true},
		{"medium", Settings{Profile: ProfileMedium, Scope: ScopeActivated, Interval: 5 * time.Second, StorageLimit: 1}, true},
		{"high", Settings{Profile: ProfileHigh, Scope: ScopeAll, Interval: time.Second, StorageLimit: 1}, true},
		{"custom", Settings{Profile: ProfileCustom, Scope: ScopeActivated, Interval: 2 * time.Second, StorageLimit: 1}, true},
		{"low wrong scope", Settings{Profile: ProfileLow, Scope: ScopeAll, Interval: 15 * time.Second, StorageLimit: 1}, false},
		{"medium wrong interval", Settings{Profile: ProfileMedium, Scope: ScopeActivated, Interval: time.Second, StorageLimit: 1}, false},
		{"unknown profile", Settings{Profile: "other", Scope: ScopeAll, Interval: time.Second, StorageLimit: 1}, false},
		{"no budget", Settings{Profile: ProfileHigh, Scope: ScopeAll, Interval: time.Second}, false},
		{"bad revision", Settings{Profile: ProfileHigh, Scope: ScopeAll, Interval: time.Second, StorageLimit: 1, Revision: math.MaxInt64}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Save(ctx, tt.set)
			if (err == nil) != tt.ok {
				t.Fatalf("error=%v", err)
			}
			if tt.ok {
				if _, err := db.SQL.ExecContext(ctx, `DELETE FROM metrics_settings`); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestConcurrentSettingsUpdatesOneWinner(t *testing.T) {
	ctx := context.Background()
	db, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	store := NewSettingsStore(db.SQL)
	saved, err := store.Save(ctx, Settings{Profile: ProfileHigh, Scope: ScopeAll, Interval: time.Second, StorageLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Save(ctx, saved)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	var success, stale int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrStaleSettings):
			stale++
		default:
			t.Fatal(err)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatalf("success=%d stale=%d", success, stale)
	}
}
