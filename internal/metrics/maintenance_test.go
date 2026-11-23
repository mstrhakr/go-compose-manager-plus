package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestMaintenanceReportsRetryAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time)
	reports := make(chan error)
	failure := errors.New("injected pressure")
	attempts := 0
	done := make(chan error, 1)
	go func() {
		done <- runMaintenance(ctx, func(context.Context, time.Time) error {
			attempts++
			if attempts == 1 {
				return failure
			}
			return nil
		}, time.Now, ticks, func(err error) { reports <- err })
	}()
	if err := <-reports; !errors.Is(err, failure) {
		t.Fatalf("first report=%v", err)
	}
	ticks <- time.Now()
	if err := <-reports; err != nil {
		t.Fatalf("recovery report=%v", err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown=%v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d", attempts)
	}
}

func TestScheduledMaintenanceExpiresIdleHistory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	now := time.Now().UTC()
	old := now.Add(-2 * time.Hour)
	if err := store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: old}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	managed, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := managed.Close(); err != nil {
			t.Error(err)
		}
	}()
	runCtx, cancel := context.WithCancel(ctx)
	reports := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		done <- RunMaintenance(runCtx, managed, time.Hour, func(err error) { reports <- err })
	}()
	if err := <-reports; err != nil {
		cancel()
		t.Fatal(err)
	}
	got, err := managed.Select(ctx, id, old, now, 1)
	if err != nil || len(got) != 0 {
		cancel()
		t.Fatalf("expired samples=%#v error=%v", got, err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("maintenance shutdown=%v", err)
	}
}

func TestMaintenanceRequiresExplicitConfiguration(t *testing.T) {
	if err := RunMaintenance(context.Background(), nil, time.Second, func(error) {}); err == nil {
		t.Fatal("accepted nil store")
	}
	if err := RunMaintenance(context.Background(), &ManagedStore{}, 0, func(error) {}); err == nil {
		t.Fatal("accepted zero interval")
	}
	if err := RunMaintenance(context.Background(), &ManagedStore{}, time.Second, nil); err == nil {
		t.Fatal("accepted nil reporter")
	}
}
