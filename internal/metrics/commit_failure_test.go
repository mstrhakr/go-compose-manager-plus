package metrics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestManagedCommitFailureReportsUnknownOutcome(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	for _, tt := range []struct {
		name   string
		table  string
		insert func(*ManagedStore) error
	}{
		{"samples", "samples", func(m *ManagedStore) error {
			return m.Insert(ctx, now, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: now}}})
		}},
		{"events", "container_events", func(m *ManagedStore) error {
			return m.InsertEvents(ctx, now, []Event{{ContainerID: id, Time: now, Kind: EventOOM}})
		}},
		{"states", "container_states", func(m *ManagedStore) error {
			return m.InsertStates(ctx, now, []State{{ContainerID: id, Time: now, State: "running", Health: HealthNone}})
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			m, err := OpenManaged(ctx, dir, 10<<20)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := m.Close(); err != nil {
					t.Error(err)
				}
			}()
			for _, statement := range []string{
				`PRAGMA foreign_keys = ON`,
				`CREATE TABLE commit_parent (id INTEGER PRIMARY KEY)`,
				`CREATE TABLE commit_child (parent_id INTEGER REFERENCES commit_parent(id) DEFERRABLE INITIALLY DEFERRED)`,
				fmt.Sprintf(`CREATE TRIGGER fail_commit AFTER INSERT ON %s BEGIN INSERT INTO commit_child VALUES (1); END`, tt.table),
			} {
				if _, err := m.store.db.ExecContext(ctx, statement); err != nil {
					t.Fatal(err)
				}
			}
			err = tt.insert(m)
			if !errors.Is(err, ErrMetricsPaused) || !errors.Is(err, ErrWriteOutcomeUnknown) || errors.Is(err, ErrWriteCommitted) {
				t.Fatalf("commit error = %v", err)
			}
			status, err := m.Status(ctx)
			if err != nil || !status.Paused {
				t.Fatalf("status after commit failure = %#v, %v", status, err)
			}
			if _, err := m.store.db.ExecContext(ctx, `DROP TRIGGER fail_commit`); err != nil {
				t.Fatal(err)
			}
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			m, err = OpenManaged(ctx, dir, 10<<20)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.Maintain(ctx, now); !errors.Is(err, ErrReconcileWrite) {
				t.Fatalf("uncertain write did not block maintenance: %v", err)
			}
			if err := tt.insert(m); !errors.Is(err, ErrReconcileWrite) {
				t.Fatalf("uncertain write did not block new write: %v", err)
			}
			status, err = m.Status(ctx)
			if err != nil || status.UncertainWrite == nil || status.PendingWrite == nil || status.PendingWrite.Kind != tt.name {
				t.Fatalf("uncertainty not reported: %#v %v", status, err)
			}
			if err := m.AcknowledgeReconciledWrite(ctx); err != nil {
				t.Fatal(err)
			}
			if err := m.Maintain(ctx, now); err != nil {
				t.Fatalf("maintenance after reconciliation: %v", err)
			}
			if err := m.AcknowledgeReconciledWrite(ctx); err == nil {
				t.Fatal("accepted acknowledgement without uncertainty")
			}
		})
	}
}
