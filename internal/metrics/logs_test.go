package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	docker "github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestLogsPersistOriginalMessagesAndFilterBeforeLimit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	id, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	log := Log{ContainerID: id, Time: now.Add(-time.Second), ReceivedAt: now,
		Stream: docker.LogStderr, Message: "  {\"level\":\"info\"}\r\ntraceback\x00\xff"}
	logs := []Log{log, log}
	for i := range 10 {
		logs = append(logs, Log{ContainerID: other, Time: now.Add(-time.Duration(i) * time.Millisecond),
			ReceivedAt: now, Stream: docker.LogTTY, Message: "busy container"})
	}
	if err := store.InsertLogs(ctx, now, logs); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	page, err := store.SelectLogs(ctx, id, now.Add(-time.Minute), now.Add(time.Second), 2)
	if err != nil || page.More || !reflect.DeepEqual(page.Logs, []Log{log, log}) {
		t.Fatalf("original duplicate messages after reopen = %#v, error %v", page, err)
	}
	page, err = store.SelectLogs(ctx, id, now.Add(-time.Minute), now.Add(time.Second), 1)
	if err != nil || !page.More || !reflect.DeepEqual(page.Logs, []Log{log}) {
		t.Fatalf("selected-container limit = %#v, error %v", page, err)
	}
	page, err = store.SelectLogs(ctx, "", now.Add(-time.Minute), now.Add(time.Second), 2)
	if err != nil || !page.More || len(page.Logs) != 2 || page.Logs[0].ContainerID != other ||
		!page.Logs[0].Time.Equal(now) || !page.Logs[1].Time.Equal(now.Add(-time.Millisecond)) {
		t.Fatalf("aggregate newest-first logs = %#v, error %v", page, err)
	}
	ids, err := store.SelectObservedContainers(ctx, now.Add(-time.Minute), now.Add(time.Second))
	if err != nil || !reflect.DeepEqual(ids, []string{id, other}) {
		t.Fatalf("log-only identities = %v, error %v", ids, err)
	}
}

func TestLogBatchValidationAndRollback(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	log := Log{ContainerID: strings.Repeat("a", 64), Time: now, ReceivedAt: now, Stream: docker.LogStdout}
	for _, test := range []struct {
		name   string
		change func(*Log)
	}{
		{"invalid ID", func(log *Log) { log.ContainerID = "invalid" }},
		{"missing timestamp", func(log *Log) { log.Time = time.Time{} }},
		{"missing receipt", func(log *Log) { log.ReceivedAt = time.Time{} }},
		{"expired", func(log *Log) { log.Time = now.Add(-time.Hour - time.Nanosecond) }},
		{"future timestamp", func(log *Log) { log.Time = now.Add(time.Nanosecond) }},
		{"future receipt", func(log *Log) { log.ReceivedAt = now.Add(time.Nanosecond) }},
		{"stream", func(log *Log) { log.Stream = "error" }},
		{"oversized", func(log *Log) { log.Message = strings.Repeat("x", docker.MaxLogMessageBytes+1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := log
			test.change(&invalid)
			if err := store.InsertLogs(ctx, now, []Log{log, invalid}); err == nil {
				t.Fatal("accepted invalid record")
			}
		})
	}
	for _, batch := range [][]Log{nil, make([]Log, maxLogBatch+1)} {
		if err := store.InsertLogs(ctx, now, batch); err == nil {
			t.Fatal("accepted unbounded/empty log batch")
		}
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER fail_log_insert BEFORE INSERT ON container_logs
		WHEN NEW.message = 'fail' BEGIN SELECT RAISE(ABORT, 'injected log write failure'); END`); err != nil {
		t.Fatal(err)
	}
	fail := log
	fail.Message = "fail"
	if err := store.InsertLogs(ctx, now, []Log{log, fail}); err == nil {
		t.Fatal("injected write failure not reported")
	}
	page, err := store.SelectLogs(ctx, "", now.Add(-time.Second), now.Add(time.Second), 10)
	if err != nil || len(page.Logs) != 0 {
		t.Fatalf("failed batch partially committed: %#v, error %v", page, err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER fail_log_insert`); err != nil {
		t.Fatal(err)
	}
	log.Time = now.Add(-time.Hour)
	log.Message, log.Truncated = strings.Repeat("x", docker.MaxLogMessageBytes), true
	if err := store.InsertLogs(ctx, now, []Log{log}); err != nil {
		t.Fatalf("exact cutoff/size rejected: %v", err)
	}
	page, err = store.SelectLogs(ctx, log.ContainerID, log.Time, now, 1)
	if err != nil || !reflect.DeepEqual(page.Logs, []Log{log}) {
		t.Fatalf("truncation/cutoff record differs: count %d, more %t, error %v", len(page.Logs), page.More, err)
	}
	for _, test := range []struct {
		id    string
		start time.Time
		end   time.Time
		limit int
	}{
		{"bad", now.Add(-time.Second), now, 1},
		{"", now.Add(-time.Second), now, 0},
		{"", now.Add(-time.Second), now, maxLogQuery + 1},
		{"", now.Add(-time.Hour - time.Nanosecond), now, 1},
		{"", now, now, 1},
	} {
		if _, err := store.SelectLogs(ctx, test.id, test.start, test.end, test.limit); err == nil {
			t.Fatalf("accepted invalid query: %#v", test)
		}
	}
}

func TestLogQueryBoundsMessageBytes(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	recordCount := maxLogQueryBytes / docker.MaxLogMessageBytes
	var logs []Log
	for i := range recordCount + 1 {
		logs = append(logs, Log{ContainerID: id, Time: now.Add(-time.Duration(i) * time.Second),
			ReceivedAt: now, Stream: docker.LogStdout, Message: strings.Repeat("x", docker.MaxLogMessageBytes)})
	}
	if err := store.InsertLogs(ctx, now, logs); err != nil {
		t.Fatal(err)
	}
	page, err := store.SelectLogs(ctx, id, now.Add(-time.Minute), now.Add(time.Second), maxLogQuery)
	if err != nil || !page.More || len(page.Logs) != recordCount {
		t.Fatalf("byte-bounded query: count %d, more %t, error %v", len(page.Logs), page.More, err)
	}
	total := 0
	for _, record := range page.Logs {
		total += len(record.Message)
	}
	if total != maxLogQueryBytes {
		t.Fatalf("message bytes = %d, want exact %d-byte budget", total, maxLogQueryBytes)
	}
}

func TestVersionSevenLogMigrationPreservesExistingHistoryAndPendingWrite(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	event := Event{ContainerID: id, Time: now, Kind: EventOOM}
	if err := store.InsertEvents(ctx, []Event{event}); err != nil {
		t.Fatal(err)
	}
	removeCurrentStateSchema(t, store.db)
	for _, statement := range []string{
		`DROP TABLE container_logs`,
		`INSERT INTO pending_write VALUES (1, 'events', 123, 0)`,
		`PRAGMA user_version = 7`,
	} {
		if _, err := store.db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	events, err := store.SelectEvents(ctx, id, now.Add(-time.Second), now.Add(time.Second), 10)
	if err != nil || !reflect.DeepEqual(events, []Event{event}) {
		t.Fatalf("migration changed event history: %#v, error %v", events, err)
	}
	var kind string
	var committed bool
	if err := store.db.QueryRowContext(ctx, `SELECT kind, committed FROM pending_write WHERE id = 1`).Scan(&kind, &committed); err != nil ||
		kind != "events" || committed {
		t.Fatalf("migration changed pending marker: %s/%t, error %v", kind, committed, err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM pending_write`); err != nil {
		t.Fatal(err)
	}
	log := Log{ContainerID: id, Time: now, ReceivedAt: now, Stream: docker.LogStdout, Message: "after migration\n"}
	if err := store.InsertLogs(ctx, now, []Log{log}); err != nil {
		t.Fatalf("migrated log write: %v", err)
	}
	page, err := store.SelectLogs(ctx, id, now.Add(-time.Second), now.Add(time.Second), 10)
	if err != nil || !reflect.DeepEqual(page.Logs, []Log{log}) {
		t.Fatalf("migrated log query: %#v, error %v", page, err)
	}
}

func TestDockerLogAPIToSQLiteAndOneHourExpiry(t *testing.T) {
	listener, socket := newDockerSocket(t)
	id := strings.Repeat("a", 64)
	at := time.Now().UTC().Add(-time.Minute)
	message := " {\"level\":\"info\",\"message\":\"not an error\"}\r\n"
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("mutating Docker request: %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":true},"State":{"Status":"running"}}`, id)
		case "/v1.40/containers/" + id + "/logs":
			fmt.Fprint(w, at.Format(time.RFC3339Nano)+" "+message)
		default:
			t.Errorf("unexpected Docker request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := OpenManagedLogs(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	err = docker.NewClient(socket).ReadContainerLogs(ctx, id, docker.LogOptions{Since: at, Until: at.Add(time.Second)},
		func(record docker.ContainerLog) error {
			return store.InsertLogs(ctx, time.Now(), []Log{{
				ContainerID: record.ID, Time: record.Time, ReceivedAt: record.ReceivedAt,
				Stream: record.Stream, Message: record.Message, Truncated: record.Truncated,
			}})
		})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenManagedLogs(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.SelectLogs(ctx, id, at, at.Add(time.Second), 1)
	if err != nil || len(page.Logs) != 1 || page.Logs[0].Message != message ||
		page.Logs[0].Stream != docker.LogTTY || !page.Logs[0].Time.Equal(at) {
		t.Fatalf("API-to-SQLite round trip = %#v, error %v", page, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	managed, err := OpenManagedLogs(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer managed.Close()
	for _, offset := range []time.Duration{time.Hour, time.Hour + time.Nanosecond} {
		if err := managed.Maintain(ctx, at.Add(offset)); err != nil {
			t.Fatalf("maintenance at age %s: %v", offset, err)
		}
		page, err := managed.store.SelectLogs(ctx, id, at, at.Add(time.Second), 1)
		want := 1
		if offset > time.Hour {
			want = 0
		}
		if err != nil || len(page.Logs) != want {
			t.Fatalf("records at age %s = %#v, error %v", offset, page, err)
		}
	}
}
