package metrics

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEventAndStateHistoryReopenAndBoundaries(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	events := []Event{
		{ContainerID: id, Time: base, Kind: EventStart},
		{ContainerID: id, Time: base.Add(time.Second), Kind: EventHealth, Health: HealthHealthy},
		{ContainerID: id, Time: base.Add(time.Second), Kind: EventOOM},
		{ContainerID: other, Time: base, Kind: EventExit},
	}
	states := []State{
		{ContainerID: id, Time: base, State: "running", Health: HealthNone},
		{ContainerID: id, Time: base.Add(time.Second), State: "exited", Health: HealthUnknown},
		{ContainerID: other, Time: base, State: "unknown", Health: HealthUnknown},
	}
	if err := store.InsertEvents(ctx, events); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertStates(ctx, states); err != nil {
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
	gotEvents, err := store.SelectEvents(ctx, id, base, base.Add(2*time.Second), 10)
	if err != nil || !reflect.DeepEqual(gotEvents, events[:3]) {
		t.Fatalf("events = %#v, error = %v", gotEvents, err)
	}
	gotEvents, err = store.SelectEvents(ctx, id, base, base.Add(time.Second), 10)
	if err != nil || !reflect.DeepEqual(gotEvents, events[:1]) {
		t.Fatalf("bounded events = %#v, error = %v", gotEvents, err)
	}
	gotEvents, err = store.SelectEvents(ctx, id, base, base.Add(2*time.Second), 2)
	if err != nil || !reflect.DeepEqual(gotEvents, events[:2]) {
		t.Fatalf("limited events = %#v, error = %v", gotEvents, err)
	}
	gotStates, err := store.SelectStates(ctx, id, base, base.Add(2*time.Second), 10)
	if err != nil || !reflect.DeepEqual(gotStates, states[:2]) {
		t.Fatalf("states = %#v, error = %v", gotStates, err)
	}
	gotStates, err = store.SelectStates(ctx, other, base, base.Add(time.Second), 1)
	if err != nil || !reflect.DeepEqual(gotStates, states[2:]) {
		t.Fatalf("other container states = %#v, error = %v", gotStates, err)
	}
}

func TestInvalidHistoryAndAtomicStateInsert(t *testing.T) {
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
	state := State{ContainerID: id, Time: base, State: "running", Health: HealthHealthy}
	if err := store.InsertStates(ctx, []State{state}); err != nil {
		t.Fatal(err)
	}
	next := state
	next.Time = base.Add(time.Second)
	if err := store.InsertStates(ctx, []State{next, state}); err == nil {
		t.Fatal("duplicate state accepted")
	}
	states, err := store.SelectStates(ctx, id, base, base.Add(2*time.Second), 10)
	if err != nil || !reflect.DeepEqual(states, []State{state}) {
		t.Fatalf("partial state batch persisted: %#v, error = %v", states, err)
	}
	for _, invalid := range []Event{
		{ContainerID: id, Time: base, Kind: "invalid"},
		{ContainerID: id, Time: base, Kind: EventHealth, Health: HealthNone},
		{ContainerID: id, Time: base, Kind: EventStart, Health: HealthHealthy},
	} {
		if err := store.InsertEvents(ctx, []Event{invalid}); err == nil {
			t.Fatalf("accepted invalid event %#v", invalid)
		}
	}
	if err := store.InsertStates(ctx, []State{{ContainerID: id, Time: base, State: "running", Health: ""}}); err == nil {
		t.Fatal("accepted missing health state")
	}
	if _, err := store.SelectEvents(ctx, id, base, base, 1); err == nil {
		t.Fatal("accepted empty interval")
	}
}
