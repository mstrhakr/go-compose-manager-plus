package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
)

func TestMetricsSettingsAuthenticatedConfiguration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := auth.OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.SQL.Close() })
	manager, err := auth.NewManager(ctx, db, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Bootstrap(ctx, "operator", "bootstrap-password-123"); err != nil {
		t.Fatal(err)
	}
	origin, err := url.Parse(testOrigin)
	if err != nil {
		t.Fatal(err)
	}
	store := metrics.NewSettingsStore(db.SQL)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	collector, err := metrics.NewCollector(t.TempDir(), store, discovery.NewStore(db.SQL), nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	handler := Handler(manager, discovery.NewStore(db.SQL), store, collector, origin, nil, logger, nil)
	cookies := map[string]*http.Cookie{}
	response := perform(handler, http.MethodGet, "/settings", nil, cookies, "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("anonymous settings = %d %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/login", nil, cookies, "")
	token := tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/login", url.Values{"username": {"operator"}, "password": {"bootstrap-password-123"}}, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("login = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/settings", nil, cookies, "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup/password" {
		t.Fatalf("password gate = %d %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/setup/password", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/setup/password", url.Values{"password": {"new-local-password-456"}, "confirm_password": {"new-local-password-456"}}, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("password change = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/settings", nil, cookies, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Disabled; no background metrics collection") ||
		!strings.Contains(response.Body.String(), `aria-label="Settings categories"`) {
		t.Fatalf("settings page = %d %q", response.Code, response.Body.String())
	}
	token = tokenFromBody(t, response.Body.String())
	values := url.Values{"profile": {"high"}, "scope": {"activated"}, "interval": {"15s"}, "storage_limit": {"10.48576"}, "storage_unit": {"MB"}, "revision": {"0"}}
	response = perform(handler, http.MethodPost, "/settings/metrics", values, cookies, "bad-token")
	if response.Code != http.StatusForbidden {
		t.Fatalf("CSRF failure = %d", response.Code)
	}
	response = perform(handler, http.MethodPost, "/settings/metrics", values, cookies, token)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings" {
		t.Fatalf("save = %d %q", response.Code, response.Body.String())
	}
	saved, err := store.Load(ctx)
	if err != nil || saved.Enabled || saved.Profile != metrics.ProfileCustom || saved.Scope != metrics.ScopeAll || saved.Interval != time.Second || saved.StorageLimit != 10485760 || saved.Revision != 1 {
		t.Fatalf("saved = %#v, %v", saved, err)
	}
	enable := url.Values{"profile": {"custom"}, "scope": {"activated"}, "interval": {"15s"}, "storage_limit": {"10.48576"}, "storage_unit": {"MB"}, "revision": {"1"}, "enabled": {"true"}}
	response = perform(handler, http.MethodPost, "/settings/metrics", enable, cookies, token)
	if !strings.Contains(response.Body.String(), "Configure a Docker socket") {
		t.Fatalf("enable without Docker = %d %q", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "metrics.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("saving preferences created metrics database: %v", err)
	}
	response = perform(handler, http.MethodPost, "/settings/metrics", values, cookies, token)
	if !strings.Contains(response.Body.String(), "Settings changed in another session") {
		t.Fatalf("stale save = %d %q", response.Code, response.Body.String())
	}
	invalid := url.Values{"profile": {"custom"}, "scope": {"all"}, "interval": {"-3s"}, "storage_limit": {"1"}, "storage_unit": {"B"}, "revision": {"1"}}
	response = perform(handler, http.MethodPost, "/settings/metrics", invalid, cookies, token)
	if !strings.Contains(response.Body.String(), "positive duration") {
		t.Fatalf("invalid interval = %d %q", response.Code, response.Body.String())
	}
	custom := url.Values{"profile": {"custom"}, "scope": {"activated"}, "interval": {"250ms"}, "storage_limit": {"20"}, "storage_unit": {"MB"}, "revision": {"1"}}
	response = perform(handler, http.MethodPost, "/settings/metrics", custom, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("custom save = %d %q", response.Code, response.Body.String())
	}
	saved, err = store.Load(ctx)
	if err != nil || saved.Enabled || saved.Profile != metrics.ProfileCustom || saved.Scope != metrics.ScopeActivated || saved.Interval != 250*time.Millisecond || saved.Revision != 2 {
		t.Fatalf("custom settings = %#v, %v", saved, err)
	}
	response = perform(handler, http.MethodGet, "/settings", nil, cookies, "")
	if !strings.Contains(response.Body.String(), `value="250ms"`) || !strings.Contains(response.Body.String(), `value="20"`) ||
		!strings.Contains(response.Body.String(), `value="MB" selected`) || !strings.Contains(response.Body.String(), `value="2"`) {
		t.Fatalf("settings reload = %d %q", response.Code, response.Body.String())
	}
	legacy := url.Values{"profile": {"custom"}, "scope": {"activated"}, "interval": {"250ms"}, "storage_limit": {"500000"}, "revision": {"2"}}
	response = perform(handler, http.MethodPost, "/settings/metrics", legacy, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("legacy byte-based save = %d %q", response.Code, response.Body.String())
	}
	saved, err = store.Load(ctx)
	if err != nil || saved.Profile != metrics.ProfileCustom || saved.StorageLimit != 500000 || saved.Revision != 3 {
		t.Fatalf("legacy byte-based settings = %#v, %v", saved, err)
	}
	preset := url.Values{"profile": {"high"}, "storage_limit": {"500000"}, "revision": {"3"}}
	response = perform(handler, http.MethodPost, "/settings/metrics", preset, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("unchanged storage preset save = %d %q", response.Code, response.Body.String())
	}
	saved, err = store.Load(ctx)
	if err != nil || saved.Profile != metrics.ProfileHigh || saved.Scope != metrics.ScopeAll || saved.Interval != time.Second || saved.StorageLimit != 500000 || saved.Revision != 4 {
		t.Fatalf("preset settings = %#v, %v", saved, err)
	}
	if err := db.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := auth.OpenDatabase(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.SQL.Close() }()
	saved, err = metrics.NewSettingsStore(reopened.SQL).Load(ctx)
	if err != nil || saved.Enabled || saved.Profile != metrics.ProfileHigh || saved.Scope != metrics.ScopeAll || saved.StorageLimit != 500000 || saved.Interval != time.Second || saved.Revision != 4 {
		t.Fatalf("reopened settings = %#v, %v", saved, err)
	}
}

func TestMetricsStorageLimitUnits(t *testing.T) {
	tests := []struct {
		value string
		unit  string
		want  int64
		valid bool
	}{
		{"500000", "B", 500000, true},
		{"500", "KB", 500000, true},
		{"10.48576", "MB", 10485760, true},
		{"1.5", "GB", 1500000000, true},
		{"0.001", "KB", 1, true},
		{"0.0001", "KB", 0, false},
		{"1/2", "MB", 0, false},
		{"0", "MB", 0, false},
		{"-1", "GB", 0, false},
		{"1", "TB", 0, false},
		{"9223372037", "GB", 0, false},
	}
	for _, test := range tests {
		t.Run(test.value+" "+test.unit, func(t *testing.T) {
			got, err := parseStorageLimit(test.value, test.unit)
			if test.valid && (err != nil || got != test.want) {
				t.Fatalf("parseStorageLimit(%q, %q) = %d, %v; want %d", test.value, test.unit, got, err, test.want)
			}
			if !test.valid && err == nil {
				t.Fatalf("parseStorageLimit(%q, %q) = %d, nil; want error", test.value, test.unit, got)
			}
		})
	}
}

func TestFormatStorageLimitRoundTrips(t *testing.T) {
	for _, want := range []int64{1, 999, 500000, 10485760, 50000000, 1234567890, math.MaxInt64} {
		value, unit := formatStorageLimit(want)
		got, err := parseStorageLimit(value, unit)
		if err != nil || got != want {
			t.Fatalf("formatStorageLimit(%d) = %q %s; round trip = %d, %v", want, value, unit, got, err)
		}
	}
}

func TestCollectionHistoryWarningReportsCumulativeBounds(t *testing.T) {
	oldest := time.Date(2026, time.November, 22, 10, 0, 0, 0, time.UTC)
	newest := oldest.Add(5 * time.Minute)
	got := collectionHistoryWarning(metrics.EvictedSamples{Count: 12, Oldest: oldest, Newest: newest})
	want := "Cumulative storage-pressure evictions have shortened history: 12 samples removed between 2026-11-22T10:00:00Z and 2026-11-22T10:05:00Z (these bounds may not be a continuous gap)."
	if got != want {
		t.Fatalf("collection history warning = %q, want %q", got, want)
	}
	if got := collectionHistoryWarning(metrics.EvictedSamples{}); got != "" {
		t.Fatalf("warning without evictions = %q", got)
	}
}
