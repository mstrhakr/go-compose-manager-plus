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

func TestConfigurationPreservesExplicitCustomAndOffRestoreAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := auth.OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSettingsStore(db.SQL)
	sources, err := store.LoadSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stats := Settings{Enabled: true, Profile: ProfileLow, Scope: ScopeActivated,
		Interval: 15 * time.Second, StorageLimit: DefaultMetricsStorageLimit}
	sources.StateEnabled = true
	sources.Logs.Configured, sources.Logs.Enabled = true, true
	sources.Logs.Profile, sources.Logs.DeliveryPreset = ProfileLow, ProfileLow
	sources.OverallProfile = ProfileCustom
	stats, sources, err = store.SaveConfiguration(ctx, stats, sources)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = auth.OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	store = NewSettingsStore(db.SQL)
	stats, sources, err = store.LoadConfiguration(ctx)
	if err != nil || sources.OverallProfile != ProfileCustom {
		t.Fatalf("explicit Custom reverted to a matching preset: %#v, %v", sources, err)
	}
	sources.RestoreStats, sources.RestoreState, sources.RestoreLogs = stats.Enabled, sources.StateEnabled, sources.Logs.Enabled
	stats.Enabled, sources.StateEnabled, sources.Logs.Enabled = false, false, false
	sources.OverallProfile = ProfileOff
	stats, sources, err = store.SaveConfiguration(ctx, stats, sources)
	if err != nil {
		t.Fatal(err)
	}
	stats, sources, err = store.LoadConfiguration(ctx)
	if err != nil || sources.OverallProfile != ProfileOff || stats.Enabled || sources.StateEnabled || sources.Logs.Enabled ||
		!sources.RestoreStats || !sources.RestoreState || !sources.RestoreLogs ||
		stats.Interval != 15*time.Second || sources.Logs.DeliveryPreset != ProfileLow {
		t.Fatalf("Off lost preferences/restore bits: %#v %#v, %v", stats, sources, err)
	}
}

func TestStaleOverallRevisionRollsBackEverySource(t *testing.T) {
	ctx := context.Background()
	db, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	store := NewSettingsStore(db.SQL)
	sources, err := store.LoadSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stats := Settings{Profile: ProfileLow, Scope: ScopeActivated, Interval: 15 * time.Second,
		StorageLimit: DefaultMetricsStorageLimit}
	stats, sources, err = store.SaveConfiguration(ctx, stats, sources)
	if err != nil {
		t.Fatal(err)
	}
	beforeStats, beforeSources := stats, sources
	stats.Enabled, sources.StateEnabled = true, true
	sources.OverallProfile = ProfileCustom
	sources.CollectionRevision--
	if _, _, err := store.SaveConfiguration(ctx, stats, sources); !errors.Is(err, ErrStaleSettings) {
		t.Fatalf("stale overall revision accepted: %v", err)
	}
	afterStats, afterSources, err := store.LoadConfiguration(ctx)
	if err != nil || !reflect.DeepEqual(beforeStats, afterStats) || !reflect.DeepEqual(beforeSources, afterSources) {
		t.Fatalf("stale save partially updated sources: %#v %#v, %v", afterStats, afterSources, err)
	}
}

func TestIndependentSourceSettingsDefaultOffAndPreserveLogPreferences(t *testing.T) {
	ctx := context.Background()
	db, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	store := NewSettingsStore(db.SQL)
	sources, err := store.LoadSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sources.StateEnabled || sources.StateScope != ScopeActivated || sources.Logs.Enabled ||
		sources.Logs.Configured || sources.Logs.StorageLimit != DefaultLogsStorageLimit {
		t.Fatalf("new source defaults = %#v", sources)
	}

	stats := Settings{
		Profile: ProfileLow, Scope: ScopeActivated, Interval: 15 * time.Second,
		StorageLimit: DefaultMetricsStorageLimit,
	}
	sources.StateEnabled, sources.StateScope = true, ScopeAll
	sources.Logs = LogSettings{
		Configured: true, Enabled: true, Profile: ProfileLow, Scope: ScopeActivated,
		DeliveryPreset: ProfileLow, StorageLimit: DefaultLogsStorageLimit, Revision: sources.Logs.Revision,
	}
	stats, sources, err = store.SaveConfiguration(ctx, stats, sources)
	if err != nil || stats.Revision != 1 || sources.StateRevision != 1 || sources.Logs.Revision != 2 {
		t.Fatalf("save sources stats=%#v sources=%#v error=%v", stats, sources, err)
	}
	sources.StateEnabled = false
	sources.Logs.Enabled = false
	stats, sources, err = store.SaveConfiguration(ctx, stats, sources)
	if err != nil || stats.Revision != 2 || sources.StateRevision != 2 || sources.Logs.Revision != 3 {
		t.Fatalf("disable sources stats=%#v sources=%#v error=%v", stats, sources, err)
	}
	loadedStats, err := store.Load(ctx)
	if err != nil || loadedStats.Enabled || loadedStats.StorageLimit != DefaultMetricsStorageLimit {
		t.Fatalf("loaded stats=%#v error=%v", loadedStats, err)
	}
	loaded, err := store.LoadSources(ctx)
	if err != nil || loaded.StateEnabled || loaded.StateScope != ScopeAll ||
		loaded.Logs.Enabled || !loaded.Logs.Configured || loaded.Logs.Profile != ProfileLow ||
		loaded.Logs.Scope != ScopeActivated || loaded.Logs.DeliveryPreset != ProfileLow ||
		loaded.Logs.StorageLimit != DefaultLogsStorageLimit {
		t.Fatalf("disabled sources did not preserve preferences: %#v error=%v", loaded, err)
	}
}

func TestUnconfiguredLogBudgetPersistsWhileOff(t *testing.T) {
	ctx := context.Background()
	db, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	store := NewSettingsStore(db.SQL)
	stats := Settings{Profile: ProfileLow, Scope: ScopeActivated, Interval: 15 * time.Second, StorageLimit: DefaultMetricsStorageLimit}
	sources, err := store.LoadSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sources.Logs.StorageLimit = 300_000_000
	stats, sources, err = store.SaveConfiguration(ctx, stats, sources)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSources(ctx)
	if err != nil || loaded.Logs.Configured || loaded.Logs.Enabled ||
		loaded.Logs.StorageLimit != 300_000_000 || loaded.Logs.Revision != sources.Logs.Revision {
		t.Fatalf("off log budget = %#v, error %v", loaded.Logs, err)
	}
}

func TestObservabilitySettingsSaveIsAtomicOnStaleSourceRevision(t *testing.T) {
	ctx := context.Background()
	db, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	store := NewSettingsStore(db.SQL)
	stats := Settings{Profile: ProfileLow, Scope: ScopeActivated, Interval: 15 * time.Second, StorageLimit: 100}
	sources, err := store.LoadSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stats, sources, err = store.SaveConfiguration(ctx, stats, sources)
	if err != nil {
		t.Fatal(err)
	}
	staleSources := sources
	sources.StateEnabled = true
	if _, _, err := store.SaveConfiguration(ctx, stats, sources); err != nil {
		t.Fatal(err)
	}
	stats.Enabled = true
	if _, _, err := store.SaveConfiguration(ctx, stats, staleSources); !errors.Is(err, ErrStaleSettings) {
		t.Fatalf("stale source save = %v", err)
	}
	loadedStats, err := store.Load(ctx)
	if err != nil || loadedStats.Enabled {
		t.Fatalf("stale source update partially saved metrics: %#v error=%v", loadedStats, err)
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
