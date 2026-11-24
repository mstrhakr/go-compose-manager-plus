package metrics

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func (c *Collector) runStateCapture(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		settings, sources, err := c.settings.LoadConfiguration(ctx)
		if err != nil {
			c.setStateControlProblem(err)
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		c.clearStateControlProblem()
		if !sources.StateEnabled {
			c.disableStateStatus(!settings.Enabled)
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		if settings.Enabled {
			c.updateStateCollectionStatus()
		} else {
			c.updateStatus(func(status *CollectorStatus) { status.StateEnabled = true })
		}
		if settings.Enabled {
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		if c.docker == nil {
			c.setStateWatcherProblem(errors.New("Docker socket is not configured"))
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		err = c.collectStateOnlySession(ctx, settings, sources)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errSettingsChanged) {
			continue
		}
		if err != nil {
			c.setStateStorageProblem(err)
			c.logger.ErrorContext(ctx, "container state collection paused", "error", err)
			if wait(ctx, ticker.C) != nil {
				return
			}
		}
	}
}

func (c *Collector) stateProblemLocked() string {
	problem := c.stateWatcherProblem
	if c.stateControlProblem != "" {
		if problem != "" {
			problem += "; settings: " + c.stateControlProblem
		} else {
			problem = "settings: " + c.stateControlProblem
		}
	}
	if c.stateStorageProblem != "" {
		if problem != "" {
			problem += "; storage: " + c.stateStorageProblem
		} else {
			problem = c.stateStorageProblem
		}
	}
	return problem
}

func (c *Collector) setStateWatcherProblem(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateWatcherProblem = err.Error()
	c.stateWatcherHealthy = false
	c.status.StateEnabled = true
	c.status.StateCollecting = false
	c.status.StateProblem = c.stateProblemLocked()
}

func (c *Collector) setStateStorageProblem(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateStorageProblem = err.Error()
	c.status.StateEnabled = true
	c.status.StateCollecting = false
	c.status.StateProblem = c.stateProblemLocked()
}

func (c *Collector) setStateControlProblem(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateControlProblem = err.Error()
	c.status.StateEnabled = true
	c.status.StateCollecting = false
	c.status.StateProblem = c.stateProblemLocked()
}

func (c *Collector) clearStateControlProblem() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateControlProblem = ""
	c.status.StateProblem = c.stateProblemLocked()
	c.status.StateCollecting = c.stateWatcherHealthy && c.status.StateProblem == ""
}

func (c *Collector) clearStateStorageProblem() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateStorageProblem = ""
	c.status.StateProblem = c.stateProblemLocked()
	c.status.StateCollecting = c.stateWatcherHealthy && c.status.StateProblem == ""
}

func (c *Collector) setStateWatcherHealthy() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateWatcherProblem = ""
	c.stateWatcherHealthy = true
	c.status.StateEnabled = true
	c.status.StateProblem = c.stateProblemLocked()
	c.status.StateCollecting = c.status.StateProblem == ""
}

func (c *Collector) updateStateCollectionStatus() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.StateEnabled = true
	c.status.StateProblem = c.stateProblemLocked()
	c.status.StateCollecting = c.status.Collecting && c.status.StateProblem == ""
}

func (c *Collector) disableStateStatus(noOtherEventOwner bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateWatcherProblem = ""
	c.stateControlProblem = ""
	c.stateStorageProblem = ""
	c.stateWatcherHealthy = false
	c.status.StateEnabled = false
	c.status.StateCollecting = false
	c.status.StateProblem = ""
	if noOtherEventOwner {
		c.status.EventConnected = false
	}
}

func (c *Collector) beginStateOnlySession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateWatcherHealthy = false
	c.status.StateCollecting = false
	c.status.EventConnected = false
}

func (c *Collector) collectStateOnlySession(ctx context.Context, settings Settings, sources SourceSettings) error {
	c.beginStateOnlySession()
	store, err := OpenManaged(ctx, c.dataDir, settings.StorageLimit)
	if err != nil {
		return fmt.Errorf("open managed state store: %w", err)
	}
	c.storeMu.Lock()
	c.store = store
	c.storeMu.Unlock()
	defer func() {
		c.storeMu.Lock()
		c.store = nil
		c.storeMu.Unlock()
		if err := store.Close(); err != nil {
			c.logger.Error("close managed state store", "error", err)
		}
	}()
	pending, err := store.ReconcilePendingWrite(ctx)
	if err != nil {
		return fmt.Errorf("reconcile interrupted state write: %w", err)
	}
	if pending != nil {
		c.logger.WarnContext(ctx, "reconciled interrupted state batch without replay",
			"kind", pending.Kind, "started_at", pending.CreatedAt, "committed", pending.Committed)
	}
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		c.setStateStorageProblem(err)
	} else {
		c.clearStateStorageProblem()
	}

	sessionCtx, cancel := context.WithCancelCause(ctx)
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		c.watchStateEvents(sessionCtx, store, sources.StateScope)
	}()
	settingsTicker := time.NewTicker(time.Second)
	defer settingsTicker.Stop()
	maintenanceTicker := time.NewTicker(5 * time.Second)
	defer maintenanceTicker.Stop()
	defer func() {
		cancel(context.Canceled)
		<-watchDone
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-settingsTicker.C:
			currentMetrics, currentSources, err := c.settings.LoadConfiguration(ctx)
			if err != nil {
				c.setStateControlProblem(err)
				continue
			}
			c.clearStateControlProblem()
			if currentMetrics.Enabled || !currentSources.StateEnabled ||
				currentSources.StateRevision != sources.StateRevision ||
				currentSources.CollectionRevision != sources.CollectionRevision ||
				currentMetrics.StorageLimit != settings.StorageLimit {
				cancel(errSettingsChanged)
				return errSettingsChanged
			}
		case now := <-maintenanceTicker.C:
			if err := store.Maintain(ctx, now.UTC()); err != nil {
				c.setStateStorageProblem(err)
			} else {
				c.clearStateStorageProblem()
			}
		}
	}
}

func (c *Collector) watchStateEvents(ctx context.Context, store *ManagedStore, scope CollectionScope) {
	known := make(map[string]State)
	var stateMu sync.Mutex
	projectsByName := make(map[string]bool)
	var connected atomic.Bool
	refresh := func() error {
		projects, scopeErr := c.eligibleProjects(ctx, scope)
		if scopeErr != nil {
			err := fmt.Errorf("partially resolved state collection scope: %w", scopeErr)
			c.setStateWatcherProblem(err)
			return err
		}
		stateMu.Lock()
		defer stateMu.Unlock()
		projectsByName = projects
		c.statePersistMu.Lock()
		if err := c.reconcileStateSnapshot(ctx, store, scope, projectsByName, known); err != nil {
			c.statePersistMu.Unlock()
			c.setStateWatcherProblem(err)
			return err
		}
		c.statePersistMu.Unlock()
		c.setStateWatcherHealthy()
		return nil
	}
	refreshTicker := time.NewTicker(c.stateRefreshEvery())
	defer refreshTicker.Stop()
	refreshDone := make(chan struct{})
	go func() {
		defer close(refreshDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-refreshTicker.C:
				if scope == ScopeActivated && connected.Load() {
					_ = refresh()
				}
			}
		}
	}()
	defer func() {
		<-refreshDone
	}()
	for ctx.Err() == nil {
		err := c.docker.WatchContainerEventsWithConnect(ctx, func() {
			connected.Store(true)
			c.updateStatus(func(status *CollectorStatus) {
				status.StateEnabled = true
				status.EventConnected = true
			})
			_ = refresh()
		}, func(event runtime.ContainerEvent) error {
			if event.Time.Before(time.Now().Add(-time.Hour)) {
				return nil
			}
			stateMu.Lock()
			defer stateMu.Unlock()
			if !containerInScope(runtime.Container{ID: event.ID, Project: event.Project}, scope, projectsByName) {
				return nil
			}
			converted, ok := convertEvent(event)
			if !ok {
				return nil
			}
			if err := c.reconcileWrite(ctx, store, store.InsertEvents(ctx, time.Now().UTC(), []Event{converted})); err != nil {
				c.setStateStorageProblem(err)
				return fmt.Errorf("persist observed container event: %w", err)
			}
			c.clearStateStorageProblem()
			switch event.Action {
			case "create", "start", "stop", "die", "restart", "pause", "unpause", "health_status":
				inspection, err := c.docker.InspectContainer(ctx, event.ID)
				if err != nil {
					c.statePersistMu.Lock()
					gapErr := startStateGapIfPreviouslyObserved(ctx, store, known, event.ID, time.Now().UTC())
					c.statePersistMu.Unlock()
					if gapErr != nil {
						c.setStateStorageProblem(gapErr)
					}
					c.setStateWatcherProblem(fmt.Errorf("inspect container %s after Docker event: %w", event.ID, err))
					c.logger.WarnContext(ctx, "inspect state after Docker event", "container_id", event.ID, "error", err)
					return nil
				}
				c.statePersistMu.Lock()
				err = c.persistObservedState(ctx, store, known, stateFromInspection(inspection, time.Now().UTC()))
				c.statePersistMu.Unlock()
				if err != nil {
					c.setStateStorageProblem(err)
					return err
				}
				return nil
			case "destroy":
				return nil
			default:
				return nil
			}
		})
		connected.Store(false)
		if ctx.Err() != nil {
			return
		}
		var gapErr error
		if gapErr = store.StartGlobalStateGap(ctx, time.Now().UTC()); gapErr != nil {
			c.logger.ErrorContext(ctx, "record shared state-source disconnect", "error", gapErr)
			c.setStateStorageProblem(gapErr)
		}
		c.updateStatus(func(status *CollectorStatus) { status.EventConnected = false })
		problem := errors.New("Docker event stream disconnected; state history may have gaps.")
		c.setStateWatcherProblem(problem)
		c.logger.WarnContext(ctx, "Docker state event stream disconnected", "error", err)
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

const stateScopeRefreshInterval = 15 * time.Second

func startStateGapIfPreviouslyObserved(ctx context.Context, store *ManagedStore, known map[string]State, id string, at time.Time) error {
	if _, ok := known[id]; ok {
		return store.StartStateGaps(ctx, []string{id}, at)
	}
	latest, err := store.SelectLatestState(ctx, id)
	if err != nil {
		return fmt.Errorf("read previous state for unavailable container %s: %w", id, err)
	}
	if latest == nil {
		return nil
	}
	if known != nil {
		known[id] = *latest
	}
	return store.StartStateGaps(ctx, []string{id}, at)
}

func (c *Collector) reconcileStateSnapshot(ctx context.Context, store *ManagedStore, scope CollectionScope, projects map[string]bool, known map[string]State) error {
	snapshot, err := c.docker.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("list containers for state baseline: %w", err)
	}
	_, globalGapOpen, err := store.OpenGlobalStateGap(ctx)
	if err != nil {
		return fmt.Errorf("read shared state observation gap: %w", err)
	}
	var failures error
	seen := make(map[string]bool, len(snapshot.Containers))
	for _, container := range snapshot.Containers {
		if !containerInScope(container, scope, projects) {
			continue
		}
		if seen[container.ID] {
			failures = errors.Join(failures, fmt.Errorf("Docker snapshot repeats container ID %s", container.ID))
			continue
		}
		seen[container.ID] = true
		inspection, err := c.docker.InspectContainer(ctx, container.ID)
		if err != nil {
			if gapErr := startStateGapIfPreviouslyObserved(ctx, store, known, container.ID, time.Now().UTC()); gapErr != nil {
				failures = errors.Join(failures, fmt.Errorf("record unavailable state inspection for %s: %w", container.ID, gapErr))
			}
			failures = errors.Join(failures, fmt.Errorf("inspect container %s for state baseline: %w", container.ID, err))
			continue
		}
		current := stateFromInspection(inspection, time.Now().UTC())
		_, gapOpen, err := store.OpenStateGap(ctx, current.ContainerID)
		if err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		current.ReconnectObserved = gapOpen || globalGapOpen
		if err := c.persistObservedState(ctx, store, known, current); err != nil {
			failures = errors.Join(failures, err)
		}
	}
	if globalGapOpen && failures == nil {
		if err := store.EndGlobalStateGap(ctx, time.Now().UTC()); err != nil {
			failures = errors.Join(failures, fmt.Errorf("end shared state observation gap: %w", err))
		}
	}
	return failures
}

func stateFromInspection(inspection runtime.ContainerInspection, at time.Time) State {
	return State{
		ContainerID: inspection.ID,
		Time:        at,
		State:       inspection.State,
		Health:      Health(inspection.Health),
	}
}

func (c *Collector) persistObservedState(ctx context.Context, store *ManagedStore, known map[string]State, current State) error {
	if _, err := validateState(current); err != nil {
		return fmt.Errorf("invalid observed state: %w", err)
	}
	gapStart, gapOpen, err := store.OpenStateGap(ctx, current.ContainerID)
	if err != nil {
		return fmt.Errorf("read open state gap for %s: %w", current.ContainerID, err)
	}
	current.ReconnectObserved = current.ReconnectObserved || (gapOpen && !gapStart.IsZero())
	changed, err := c.changedStates(ctx, store, []State{current})
	if err != nil {
		return err
	}
	if len(changed) == 0 {
		return nil
	}
	current = changed[0]
	writeErr := c.reconcileWrite(ctx, store, store.InsertStates(ctx, time.Now().UTC(), changed))
	if writeErr == nil || errors.Is(writeErr, ErrWriteCommitted) {
		known[current.ContainerID] = current
		c.rememberStates(changed)
	}
	if writeErr != nil {
		return fmt.Errorf("persist changed container state: %w", writeErr)
	}
	c.clearStateStorageProblem()
	if gapOpen {
		if err := store.EndStateGap(ctx, current.ContainerID, current.Time); err != nil {
			return fmt.Errorf("end state observation gap for %s: %w", current.ContainerID, err)
		}
	}
	return nil
}
