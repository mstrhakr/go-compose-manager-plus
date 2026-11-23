package metrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileUsageIncludesSQLiteSidecars(t *testing.T) {
	store, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	usage, err := store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	if usage.Database == 0 || usage.WAL == 0 || usage.Shared == 0 ||
		usage.Total != usage.Database+usage.WAL+usage.Shared+usage.Journal {
		t.Fatalf("usage = %#v", usage)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := store.FileUsage()
	if err != nil || after.Database == 0 || after.WAL != 0 || after.Shared != 0 || after.Total != after.Database+after.Journal {
		t.Fatalf("after close = %#v, error %v", after, err)
	}
}

func TestFileUsageRejectsMissingAndNonregularFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.db")
	store := &Store{path: path}
	if _, err := store.FileUsage(); err == nil {
		t.Fatal("missing database accepted")
	}
	if err := os.WriteFile(path, []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+"-wal", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FileUsage(); err == nil {
		t.Fatal("nonregular WAL accepted")
	}
}
