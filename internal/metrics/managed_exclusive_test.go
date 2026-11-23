package metrics

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedProcessProbe(t *testing.T) {
	dir := os.Getenv("CMP_METRICS_LOCK_PROBE_DIR")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA busy_timeout = 10`); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO samples(container_id, observed_ns) VALUES (?, ?)`, strings.Repeat("b", 64), time.Now().UnixNano())
	if os.Getenv("CMP_METRICS_LOCK_PROBE_EXPECT") == "blocked" {
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "locked") {
			t.Fatalf("expected SQLite lock error, got %v", err)
		}
	} else if err != nil {
		t.Fatalf("write after owner closed: %v", err)
	}
}

func TestManagedExclusiveAcrossProcesses(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	run := func(expect string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestManagedProcessProbe$")
		cmd.Env = append(os.Environ(), "CMP_METRICS_LOCK_PROBE_DIR="+dir, "CMP_METRICS_LOCK_PROBE_EXPECT="+expect)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("subprocess %s: %v: %s", expect, err, output)
		}
	}
	run("blocked")
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	run("allowed")
}
