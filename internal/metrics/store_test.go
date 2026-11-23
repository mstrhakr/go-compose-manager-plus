package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestSamplesPersistExpireAndPreserveMissing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 123, time.UTC)
	cpu, zero, mem := 42.5, 0.0, uint64(4096)
	samples := []Sample{
		{ContainerID: id, Observation: runtime.Observations{Time: base, CPUPercent: &cpu, MemoryUsageBytes: &mem}},
		{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Second), NetworkRxBytesPerSec: &zero}},
		{ContainerID: other, Observation: runtime.Observations{Time: base.Add(time.Second), MemoryLimitBytes: &mem}},
	}
	if err := store.Insert(ctx, samples); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	got, err := store.Select(ctx, id, base, base.Add(2*time.Second), 10)
	if err != nil || !reflect.DeepEqual(got, samples[:2]) {
		t.Fatalf("reopened samples = %#v; error = %v", got, err)
	}
	limited, err := store.Select(ctx, id, base, base.Add(time.Second), 1)
	if err != nil || !reflect.DeepEqual(limited, samples[:1]) {
		t.Fatalf("bounded samples = %#v; error = %v", limited, err)
	}
	count, err := store.ExpireBefore(ctx, base.Add(time.Second))
	if err != nil || count != 1 {
		t.Fatalf("expired = %d; error = %v", count, err)
	}
	got, err = store.Select(ctx, id, base, base.Add(2*time.Second), 10)
	if err != nil || !reflect.DeepEqual(got, samples[1:2]) {
		t.Fatalf("remaining = %#v; error = %v", got, err)
	}
	got, err = store.Select(ctx, other, base, base.Add(2*time.Second), 10)
	if err != nil || !reflect.DeepEqual(got, samples[2:]) {
		t.Fatalf("other container = %#v; error = %v", got, err)
	}
}

func TestFullUnsignedMemoryRangePersists(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	usage, limit := uint64(math.MaxInt64)+1, uint64(math.MaxUint64)
	sample := Sample{ContainerID: id, Observation: runtime.Observations{
		Time: base, MemoryUsageBytes: &usage, MemoryLimitBytes: &limit,
	}}
	if err := store.Insert(ctx, []Sample{sample}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	got, err := store.Select(ctx, id, base, base.Add(time.Second), 1)
	if err != nil || !reflect.DeepEqual(got, []Sample{sample}) {
		t.Fatalf("reopened unsigned metrics = %#v, error = %v", got, err)
	}
}

func TestVersionOneMigrationPreservesSamples(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE samples (
			container_id TEXT NOT NULL, observed_ns INTEGER NOT NULL, cpu_percent REAL,
			memory_usage_bytes INTEGER, memory_limit_bytes INTEGER, network_rx_bytes_per_sec REAL,
			network_tx_bytes_per_sec REAL, block_read_bytes_per_sec REAL, block_write_bytes_per_sec REAL,
			PRIMARY KEY(container_id, observed_ns)) WITHOUT ROWID`,
		`CREATE INDEX samples_time ON samples(observed_ns)`,
		`PRAGMA user_version = 1`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	id := strings.Repeat("a", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(ctx, `INSERT INTO samples(container_id, observed_ns, memory_usage_bytes) VALUES (?, ?, ?)`, id, base.UnixNano(), int64(math.MaxInt64)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	got, err := store.Select(ctx, id, base, base.Add(time.Second), 1)
	if err != nil || len(got) != 1 || got[0].Observation.MemoryUsageBytes == nil ||
		*got[0].Observation.MemoryUsageBytes != uint64(math.MaxInt64) || got[0].Observation.MemoryLimitBytes != nil {
		t.Fatalf("migrated samples = %#v, error = %v", got, err)
	}
	var version int
	if err := store.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schema version = %d, error = %v", version, err)
	}
	maxValue := uint64(math.MaxUint64)
	if err := store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{
		Time: base.Add(time.Second), MemoryLimitBytes: &maxValue,
	}}}); err != nil {
		t.Fatalf("insert full-width integer after migration: %v", err)
	}
	if err := store.InsertEvents(ctx, []Event{{ContainerID: id, Time: base, Kind: EventStart}}); err != nil {
		t.Fatalf("insert event after migration: %v", err)
	}
	if err := store.InsertStates(ctx, []State{{ContainerID: id, Time: base, State: "running", Health: HealthNone}}); err != nil {
		t.Fatalf("insert state after migration: %v", err)
	}
}

func TestVersionTwoMigrationCreatesHistory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE samples (
			container_id TEXT NOT NULL, observed_ns INTEGER NOT NULL, cpu_percent REAL,
			memory_usage_bytes TEXT, memory_limit_bytes TEXT, network_rx_bytes_per_sec REAL,
			network_tx_bytes_per_sec REAL, block_read_bytes_per_sec REAL, block_write_bytes_per_sec REAL,
			PRIMARY KEY(container_id, observed_ns)) WITHOUT ROWID`,
		`CREATE INDEX samples_time ON samples(observed_ns)`,
		`PRAGMA user_version = 2`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	id := strings.Repeat("b", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	if err := store.InsertEvents(ctx, []Event{{ContainerID: id, Time: base, Kind: EventOOM}}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertStates(ctx, []State{{ContainerID: id, Time: base, State: "exited", Health: HealthUnknown}}); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := store.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schema version = %d, error = %v", version, err)
	}
}

func TestVersionThreeMigrationRecordsEviction(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("c", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	if err := store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: base}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertEvents(ctx, []Event{{ContainerID: id, Time: base, Kind: EventOOM}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE sample_eviction_history`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE pressure_gate`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE pending_write`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `PRAGMA user_version = 3`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	var version int
	if err := store.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schema version = %d, error = %v", version, err)
	}
	evicted, err := store.EvictOldestSamples(ctx, 1)
	if err != nil || evicted.Count != 1 || !evicted.Oldest.Equal(base) {
		t.Fatalf("eviction after migration = %#v, error = %v", evicted, err)
	}
	history, err := store.SampleEvictionHistory(ctx)
	if err != nil || history != evicted {
		t.Fatalf("history after migration = %#v, error = %v", history, err)
	}
	events, err := store.SelectEvents(ctx, id, base, base.Add(time.Second), 1)
	if err != nil || len(events) != 1 || events[0].Kind != EventOOM {
		t.Fatalf("event after migration = %#v, error = %v", events, err)
	}
}

func TestInsertValidationAndAtomicity(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	id := strings.Repeat("a", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	sample := Sample{ContainerID: id, Observation: runtime.Observations{Time: base}}
	if err := store.Insert(ctx, []Sample{sample}); err != nil {
		t.Fatal(err)
	}
	next := Sample{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Second)}}
	if err := store.Insert(ctx, []Sample{next, sample}); err == nil {
		t.Fatal("duplicate sample accepted")
	}
	got, err := store.Select(ctx, id, base, base.Add(2*time.Second), 10)
	if err != nil || !reflect.DeepEqual(got, []Sample{sample}) {
		t.Fatalf("failed batch was not rolled back: %#v, %v", got, err)
	}
	nan, neg := math.NaN(), -1.0
	for _, invalid := range []Sample{
		{ContainerID: "bad", Observation: sample.Observation},
		{ContainerID: id},
		{ContainerID: id, Observation: runtime.Observations{Time: base, CPUPercent: &nan}},
		{ContainerID: id, Observation: runtime.Observations{Time: base, BlockReadBytesPerSec: &neg}},
		{ContainerID: id, Observation: runtime.Observations{Time: time.Date(2400, 1, 1, 0, 0, 0, 0, time.UTC)}},
	} {
		if err := store.Insert(ctx, []Sample{invalid}); err == nil {
			t.Fatalf("accepted invalid sample %#v", invalid)
		}
	}
	if err := store.Insert(ctx, nil); err == nil {
		t.Fatal("empty batch accepted")
	}
	if _, err := store.Select(ctx, id, base, base, 1); err == nil {
		t.Fatal("empty interval accepted")
	}
	if _, err := store.Select(ctx, id, base, base.Add(time.Second), 0); err == nil {
		t.Fatal("zero limit accepted")
	}
}

func TestConcurrentInsertAndExpire(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	id := strings.Repeat("a", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Duration(i) * time.Second)}}}); err != nil {
				t.Error(err)
			}
			if _, err := store.ExpireBefore(ctx, base); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	got, err := store.Select(ctx, id, base, base.Add(20*time.Second), 20)
	if err != nil || len(got) != 20 {
		t.Fatalf("concurrent result count = %d, error = %v", len(got), err)
	}
	for i, sample := range got {
		if !sample.Observation.Time.Equal(base.Add(time.Duration(i) * time.Second)) {
			t.Fatalf("out-of-order sample %d: %v", i, sample.Observation.Time)
		}
	}
}

func TestOpenRejectsNonRegularDatabase(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "metrics.db"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), dir); err == nil {
		t.Fatal("accepted directory as database")
	}
}

func TestFullDatabaseRejectsBatchWithoutPartialCommit(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	var pages int
	if err := store.db.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	var limit int
	if err := store.db.QueryRowContext(ctx, `PRAGMA max_page_count = `+fmt.Sprint(pages+1)).Scan(&limit); err != nil {
		t.Fatal(err)
	}
	if limit != pages+1 {
		t.Fatalf("page limit = %d, want %d", limit, pages+1)
	}
	id := strings.Repeat("a", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	samples := make([]Sample, 1000)
	for i := range samples {
		samples[i] = Sample{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Duration(i) * time.Second)}}
	}
	if err := store.Insert(ctx, samples); err == nil || !strings.Contains(err.Error(), "database or disk is full") {
		t.Fatalf("expected SQLite full error, got %v", err)
	}
	got, err := store.Select(ctx, id, base, base.Add(1000*time.Second), 1000)
	if err != nil || len(got) != 0 {
		t.Fatalf("full database left %d samples, error = %v", len(got), err)
	}
	if _, err := store.db.ExecContext(ctx, `PRAGMA max_page_count = 1000`); err != nil {
		t.Fatal(err)
	}
	if err := store.Insert(ctx, samples[:1]); err != nil {
		t.Fatalf("retry after increasing limit: %v", err)
	}
	got, err = store.Select(ctx, id, base, base.Add(time.Second), 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("retry result = %#v, error = %v", got, err)
	}
}

func TestOpenRejectsUnknownSchema(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name string
		sql  string
	}{
		{"newer version", `PRAGMA user_version = 7`},
		{"unversioned database", `CREATE TABLE unrelated (id INTEGER)`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			db, err := sql.Open("sqlite", filepath.Join(dir, "metrics.db"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, tt.sql); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if store, err := Open(ctx, dir); err == nil {
				_ = store.Close()
				t.Fatal("accepted unsupported schema")
			}
		})
	}
}

func TestCanceledInsertReturnsError(t *testing.T) {
	store, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Insert(ctx, []Sample{{ContainerID: strings.Repeat("a", 64),
		Observation: runtime.Observations{Time: time.Now()}}}); err == nil {
		t.Fatal("canceled insert succeeded")
	}
}
