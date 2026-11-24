package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func (c *Collector) runStateCapture(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		settings, sources, err := c.settings.LoadConfiguration(ctx)
		if err != nil {
			c.setStateProblem(err)
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		if !sources.StateEnabled {
			c.updateStatus(func(status *CollectorStatus) {
				status.StateEnabled = false
				status.StateCollecting = false
				status.StateProblem = ""
			})
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		c.updateStatus(func(status *CollectorStatus) {
			status.StateEnabled = true
			if settings.Enabled {
				status.StateCollecting = status.Collecting
				status.StateProblem = ""
			}
		})
		if settings.Enabled {
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		if c.docker == nil {
			c.setStateProblem(errors.New("Docker socket is not configured"))
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
			c.setStateProblem(err)
			c.logger.ErrorContext(ctx, "container state collection paused", "error", err)
			if wait(ctx, ticker.C) != nil {
				return
			}
		}
	}
}

func (c *Collector) setStateProblem(err error) {
	c.updateStatus(func(status *CollectorStatus) {
		status.StateEnabled = true
		status.StateCollecting = false
		status.StateProblem = err.Error()
	})
}

func (c *Collector) collectStateOnlySession(ctx context.Context, settings Settings, sources SourceSettings) error {
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
		if err := store.Close(); err != nil {
			c.logger.Error("close managed state store", "error", err)
		}
		c.storeMu.Unlock()
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
		c.setStateProblem(err)
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
				c.setStateProblem(err)
				continue
			}
			if currentMetrics.Enabled || !currentSources.StateEnabled ||
				currentSources.StateRevision != sources.StateRevision ||
				currentSources.CollectionRevision != sources.CollectionRevision ||
				currentMetrics.StorageLimit != settings.StorageLimit {
				cancel(errSettingsChanged)
				return errSettingsChanged
			}
		case now := <-maintenanceTicker.C:
			if err := store.Maintain(ctx, now.UTC()); err != nil {
				c.setStateProblem(err)
			} else {
				c.updateStatus(func(status *CollectorStatus) {
					status.StateCollecting = true
					status.StateProblem = ""
				})
			}
		}
	}
}

func (c *Collector) watchStateEvents(ctx context.Context, store *ManagedStore, scope CollectionScope) {
	known := make(map[string]State)
	projectsByName, scopeErr := c.eligibleProjects(ctx, scope)
	if scopeErr != nil {
		c.setStateProblem(fmt.Errorf("partially resolved state collection scope: %w", scopeErr))
	}
	connectedOnce := false
	for ctx.Err() == nil {
		err := c.docker.WatchContainerEventsWithConnect(ctx, func() {
			var connectionScopeErr error
			projectsByName, connectionScopeErr = c.eligibleProjects(ctx, scope)
			scopeErr = connectionScopeErr
			if connectionScopeErr != nil {
				c.logger.WarnContext(ctx, "partially resolved state collection scope", "error", connectionScopeErr)
			}
			c.updateStatus(func(status *CollectorStatus) {
				status.StateEnabled = true
				status.StateCollecting = true
				status.EventConnected = true
				status.StateProblem = ""
				if scopeErr != nil {
					status.StateProblem = fmt.Errorf("partially resolved state collection scope: %w", scopeErr).Error()
				}
			})
			initialSnapshot := !connectedOnce
			connectedOnce = true
			if err := c.reconcileStateSnapshot(ctx, store, scope, projectsByName, known, initialSnapshot); err != nil {
				if scopeErr != nil {
					err = errors.Join(fmt.Errorf("partially resolved state collection scope: %w", scopeErr), err)
				}
				c.setStateProblem(err)
			}
		}, func(event runtime.ContainerEvent) error {
			if event.Time.Before(time.Now().Add(-time.Hour)) {
				return nil
			}
			if !containerInScope(runtime.Container{ID: event.ID, Project: event.Project}, scope, projectsByName) {
				return nil
			}
			converted, ok := convertEvent(event)
			if !ok {
				return nil
			}
			if err := c.reconcileWrite(ctx, store, store.InsertEvents(ctx, time.Now().UTC(), []Event{converted})); err != nil {
				return fmt.Errorf("persist observed container event: %w", err)
			}
			switch event.Action {
			case "create", "start", "stop", "die", "restart", "pause", "unpause", "health_status":
				inspection, err := c.docker.InspectContainer(ctx, event.ID)
				if err != nil {
					if _, knownState := known[event.ID]; knownState {
						if gapErr := store.StartStateGaps(ctx, []string{event.ID}, time.Now().UTC()); gapErr != nil {
							c.setStateProblem(gapErr)
						}
					}
					c.logger.WarnContext(ctx, "inspect state after Docker event", "container_id", event.ID, "error", err)
					return nil
				}
				return c.persistObservedState(ctx, store, known, stateFromInspection(inspection, time.Now().UTC()), false)
			case "destroy":
				return nil
			default:
				return nil
			}
		})
		if ctx.Err() != nil {
			return
		}
		var gapErr error
		if gapErr = store.StartGlobalStateGap(ctx, time.Now().UTC()); gapErr != nil {
			c.logger.ErrorContext(ctx, "record shared state-source disconnect", "error", gapErr)
		}
		c.updateStatus(func(status *CollectorStatus) {
			status.EventConnected = false
			status.StateCollecting = false
			status.StateProblem = "Docker event stream disconnected; state history may have gaps."
			if gapErr != nil {
				status.StateProblem += " Gap persistence failed: " + gapErr.Error()
			}
		})
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

func (c *Collector) reconcileStateSnapshot(ctx context.Context, store *ManagedStore, scope CollectionScope, projects map[string]bool, known map[string]State, initial bool) error {
	snapshot, err := c.docker.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("list containers for state baseline: %w", err)
	}
	globalGapOpen, err := store.HasOpenGlobalStateGap(ctx)
	if err != nil {
		return fmt.Errorf("read shared state observation gap: %w", err)
	}
	var failures error
	seen := make(map[string]bool, len(snapshot.Containers))
	successfulInspections := 0
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
			if _, knownState := known[container.ID]; knownState {
				if gapErr := store.StartStateGaps(ctx, []string{container.ID}, time.Now().UTC()); gapErr != nil {
					failures = errors.Join(failures, fmt.Errorf("record unavailable state inspection for %s: %w", container.ID, gapErr))
				}
			}
			failures = errors.Join(failures, fmt.Errorf("inspect container %s for state baseline: %w", container.ID, err))
			continue
		}
		successfulInspections++
		current := stateFromInspection(inspection, time.Now().UTC())
		gapOpen, err := store.HasOpenStateGap(ctx, current.ContainerID)
		if err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		current.ReconnectObserved = gapOpen || globalGapOpen
		if err := c.persistObservedState(ctx, store, known, current, initial || gapOpen || globalGapOpen); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
	}
	if globalGapOpen && successfulInspections == 0 {
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

func (c *Collector) persistObservedState(ctx context.Context, store *ManagedStore, known map[string]State, current State, force bool) error {
	if _, err := validateState(current); err != nil {
		return fmt.Errorf("invalid observed state: %w", err)
	}
	gapOpen, err := store.HasOpenStateGap(ctx, current.ContainerID)
	if err != nil {
		return fmt.Errorf("read open state gap for %s: %w", current.ContainerID, err)
	}
	globalGapOpen, err := store.HasOpenGlobalStateGap(ctx)
	if err != nil {
		return fmt.Errorf("read shared state observation gap: %w", err)
	}
	current.ReconnectObserved = current.ReconnectObserved || gapOpen || globalGapOpen
	previous, found := known[current.ContainerID]
	if !found {
		latest, err := store.SelectLatestState(ctx, current.ContainerID)
		if err != nil {
			return fmt.Errorf("read previous state for %s: %w", current.ContainerID, err)
		}
		if latest != nil {
			previous, found = *latest, true
			known[current.ContainerID] = previous
		}
	}
	if found && previous.State == current.State && previous.Health == current.Health && !force && !current.ReconnectObserved {
		return nil
	}
	if err := c.reconcileWrite(ctx, store, store.InsertStates(ctx, time.Now().UTC(), []State{current})); err != nil {
		return fmt.Errorf("persist changed container state: %w", err)
	}
	known[current.ContainerID] = current
	if gapOpen {
		if err := store.EndStateGap(ctx, current.ContainerID, current.Time); err != nil {
			return fmt.Errorf("end state observation gap for %s: %w", current.ContainerID, err)
		}
	}
	if globalGapOpen {
		if err := store.EndGlobalStateGap(ctx, current.Time); err != nil {
			return fmt.Errorf("end shared state observation gap: %w", err)
		}
	}
	return nil
}
