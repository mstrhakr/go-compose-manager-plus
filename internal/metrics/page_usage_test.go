package metrics

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestDeletedPagesCanBeReusedWithoutShrinkingDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
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
	samples := make([]Sample, 5000)
	for i := range samples {
		samples[i] = Sample{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Duration(i) * time.Second)}}
	}
	if err := store.Insert(ctx, samples); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertEvents(ctx, []Event{{ContainerID: id, Time: base, Kind: EventStart}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	initialPages, err := store.PageUsage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var maximum int64
	if err := store.db.QueryRowContext(ctx, fmt.Sprintf("PRAGMA max_page_count = %d", initialPages.Pages)).Scan(&maximum); err != nil {
		t.Fatal(err)
	}
	if maximum != initialPages.Pages {
		t.Fatalf("page cap = %d, want %d", maximum, initialPages.Pages)
	}
	evicted, err := store.EvictOldestSamples(ctx, 4000)
	if err != nil || evicted.Count != 4000 || !evicted.Oldest.Equal(base) || !evicted.Newest.Equal(base.Add(3999*time.Second)) {
		t.Fatalf("eviction=%#v error=%v", evicted, err)
	}
	evictionFiles, err := store.FileUsage()
	if err != nil || evictionFiles.WAL == 0 {
		t.Fatalf("eviction WAL=%d error=%v", evictionFiles.WAL, err)
	}
	if err := store.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	survivors, err := store.Select(ctx, id, base, base.Add(5000*time.Second), 5000)
	if err != nil || len(survivors) != 1000 || !survivors[0].Observation.Time.Equal(base.Add(4000*time.Second)) {
		t.Fatalf("surviving samples=%d error=%v", len(survivors), err)
	}
	events, err := store.SelectEvents(ctx, id, base, base.Add(time.Second), 1)
	if err != nil || len(events) != 1 {
		t.Fatalf("event history=%#v error=%v", events, err)
	}
	deletedFile, err := store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	deletedPages, err := store.PageUsage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deletedFile.Database != before.Database || deletedPages.ReusablePages == 0 {
		t.Fatalf("after eviction: file before=%#v after=%#v pages=%#v", before, deletedFile, deletedPages)
	}
	newSamples := make([]Sample, 2000)
	for i := range newSamples {
		newSamples[i] = Sample{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Duration(5000+i) * time.Second)}}
	}
	if err := store.Insert(ctx, newSamples); err != nil {
		t.Fatal(err)
	}
	after, err := store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	// The insertion above also writes the WAL; checkpoint it only after
	// measuring page reuse below.
	reused, err := store.PageUsage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.WAL == 0 {
		t.Fatal("expected WAL writes while reusing database pages")
	}
	if err := store.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	final, err := store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	if final.Database != before.Database || reused.Pages != initialPages.Pages || reused.ReusablePages >= deletedPages.ReusablePages {
		t.Fatalf("after reuse: file before=%#v after=%#v pages=%#v", before, final, reused)
	}
	got, err := store.Select(ctx, id, base, base.Add(7000*time.Second), 7000)
	if err != nil || len(got) != 3000 {
		t.Fatalf("reused pages samples=%d error=%v", len(got), err)
	}
	t.Logf("page size=%d initial pages=%d; database bytes before=%d after eviction=%d; eviction WAL bytes=%d; reusable pages after eviction=%d after insertion=%d; insertion WAL bytes=%d",
		reused.PageSize, initialPages.Pages, before.Database, deletedFile.Database, evictionFiles.WAL, deletedPages.ReusablePages, reused.ReusablePages, after.WAL)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err = store.Select(ctx, id, base, base.Add(7000*time.Second), 7000)
	if err != nil || len(got) != 3000 {
		t.Fatalf("reopened samples=%d error=%v", len(got), err)
	}
	history, err := store.SampleEvictionHistory(ctx)
	if err != nil || history != evicted {
		t.Fatalf("reopened eviction history=%#v error=%v", history, err)
	}
}

func TestPageUsageReportsCancellationAndClose(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.PageUsage(canceled); err == nil {
		t.Error("canceled page accounting succeeded")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PageUsage(ctx); err == nil {
		t.Error("page accounting on closed store succeeded")
	}
}
