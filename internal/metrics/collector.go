package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

var errSettingsChanged = errors.New("metrics settings changed")

const collectorWorkers = 8

type projectReader interface {
	Projects(context.Context) ([]discovery.Project, error)
}

type CollectorStatus struct {
	Enabled        bool
	Collecting     bool
	EventConnected bool
	StoragePaused  bool
	Containers     int
	LastCycle      time.Time
	Problem        string
	Evictions      EvictedSamples
}

type Collector struct {
	dataDir  string
	settings *SettingsStore
	projects projectReader
	docker   *runtime.Client
	logger   *slog.Logger
	mu       sync.RWMutex
	status   CollectorStatus
}

func NewCollector(dataDir string, settings *SettingsStore, projects projectReader, docker *runtime.Client, logger *slog.Logger) (*Collector, error) {
	if dataDir == "" || settings == nil || projects == nil || logger == nil {
		return nil, errors.New("metrics collector requires data directory, stores and logger")
	}
	return &Collector{dataDir: dataDir, settings: settings, projects: projects, docker: docker, logger: logger}, nil
}

func (c *Collector) Status() CollectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Collector) updateStatus(update func(*CollectorStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	update(&c.status)
}

func (c *Collector) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		settings, err := c.settings.Load(ctx)
		if err != nil {
			c.problem(err)
			if err := wait(ctx, ticker.C); err != nil {
				return err
			}
			continue
		}
		if !settings.Enabled {
			c.updateStatus(func(status *CollectorStatus) {
				evictions := status.Evictions
				*status = CollectorStatus{Evictions: evictions}
			})
			if err := wait(ctx, ticker.C); err != nil {
				return err
			}
			continue
		}
		c.updateStatus(func(status *CollectorStatus) { status.Enabled = true })
		if c.docker == nil {
			c.problem(errors.New("Docker socket is not configured"))
			if err := wait(ctx, ticker.C); err != nil {
				return err
			}
			continue
		}
		if err := c.collect(ctx, settings); err != nil && !errors.Is(err, errSettingsChanged) && ctx.Err() == nil {
			c.problem(err)
			c.logger.ErrorContext(ctx, "metrics collection paused", "error", err)
			if err := wait(ctx, ticker.C); err != nil {
				return err
			}
		}
	}
}

func wait(ctx context.Context, ticks <-chan time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticks:
		return nil
	}
}

func (c *Collector) problem(err error) {
	c.updateStatus(func(status *CollectorStatus) {
		status.Enabled = true
		status.Collecting = false
		status.Problem = err.Error()
	})
}

type eventContinuity struct {
	mu         sync.RWMutex
	connected  bool
	generation uint64
	eligible   map[string]bool
}

func (state *eventContinuity) setConnected(connected bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.connected = connected
	state.generation++
}

func (state *eventContinuity) snapshot() (bool, uint64) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.connected, state.generation
}

func (state *eventContinuity) setEligible(ids map[string]bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.eligible = ids
}

func (state *eventContinuity) contains(id string) bool {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.eligible[id]
}

func (c *Collector) collect(ctx context.Context, settings Settings) error {
	store, err := OpenManaged(ctx, c.dataDir, settings.StorageLimit)
	if err != nil {
		return fmt.Errorf("open managed metrics store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			c.logger.Error("close managed metrics store", "error", err)
		}
	}()
	pending, err := store.ReconcilePendingWrite(ctx)
	if err != nil {
		return fmt.Errorf("reconcile interrupted metrics write: %w", err)
	}
	if pending != nil {
		c.logger.WarnContext(ctx, "reconciled interrupted metrics batch without replay",
			"kind", pending.Kind, "started_at", pending.CreatedAt, "committed", pending.Committed)
	}
	c.updateStatus(func(status *CollectorStatus) {
		*status = CollectorStatus{Enabled: true, Collecting: true}
	})
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		c.problem(err)
		c.updateStatus(func(status *CollectorStatus) { status.StoragePaused = true })
		if errors.Is(err, ErrReconcileWrite) {
			return err
		}
	}
	if err := c.refreshEvictions(ctx, store); err != nil {
		return err
	}

	continuity := &eventContinuity{eligible: make(map[string]bool)}
	samplers := make(map[string]*Sampler)
	previousCycleOK := false
	_, previousGeneration := continuity.snapshot()
	if !c.Status().StoragePaused {
		if err := c.collectCycle(ctx, settings, store, continuity, samplers, &previousCycleOK, &previousGeneration); err != nil {
			c.recordCycleError(err)
		}
	}
	streamCtx, cancelStream := context.WithCancel(ctx)
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		c.watchEvents(streamCtx, store, continuity)
	}()
	defer func() {
		cancelStream()
		<-streamDone
	}()

	sampleTicker := time.NewTicker(settings.Interval)
	defer sampleTicker.Stop()
	maintenanceTicker := time.NewTicker(5 * time.Second)
	defer maintenanceTicker.Stop()
	settingsTicker := time.NewTicker(time.Second)
	defer settingsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.updateStatus(func(status *CollectorStatus) { status.Collecting = false })
			return ctx.Err()
		case <-settingsTicker.C:
			current, err := c.settings.Load(ctx)
			if err != nil {
				c.recordCycleError(err)
				continue
			}
			if current.Revision != settings.Revision || !current.Enabled {
				return errSettingsChanged
			}
		case now := <-maintenanceTicker.C:
			if err := store.Maintain(ctx, now.UTC()); err != nil {
				c.updateStatus(func(status *CollectorStatus) {
					status.StoragePaused = true
					status.Problem = err.Error()
				})
			} else {
				c.updateStatus(func(status *CollectorStatus) {
					if status.StoragePaused {
						status.Problem = ""
					}
					status.StoragePaused = false
				})
			}
			if err := c.refreshEvictions(ctx, store); err != nil {
				c.recordCycleError(err)
			}
		case <-sampleTicker.C:
			if c.Status().StoragePaused {
				previousCycleOK = false
				continue
			}
			if err := c.collectCycle(ctx, settings, store, continuity, samplers, &previousCycleOK, &previousGeneration); err != nil {
				c.recordCycleError(err)
			}
		}
	}
}

func (c *Collector) collectCycle(ctx context.Context, settings Settings, store *ManagedStore, continuity *eventContinuity, samplers map[string]*Sampler, previousCycleOK *bool, previousGeneration *uint64) error {
	snapshot, err := c.docker.Snapshot(ctx)
	if err != nil {
		*previousCycleOK = false
		return fmt.Errorf("list Docker containers: %w", err)
	}
	eligibleProjects := make(map[string]bool)
	var scopeErr error
	if settings.Scope == ScopeActivated {
		projects, err := c.projects.Projects(ctx)
		if err != nil {
			*previousCycleOK = false
			return fmt.Errorf("read activated projects: %w", err)
		}
		for _, project := range projects {
			if !project.Active || !project.Available || project.ResolutionError != "" {
				continue
			}
			name, err := discovery.ResolveProjectName(ctx, project.Root, project.Path)
			if err != nil {
				c.logger.WarnContext(ctx, "skip unresolved activated metrics scope", "error", err)
				scopeErr = errors.Join(scopeErr, fmt.Errorf("resolve activated project %s: %w", project.Path, err))
				continue
			}
			eligibleProjects[name] = true
		}
		if scopeErr != nil {
			*previousCycleOK = false
		}
	}

	containers := make([]runtime.Container, 0, len(snapshot.Containers))
	eligible := make(map[string]bool)
	seen := make(map[string]bool, len(snapshot.Containers))
	cycleErr := scopeErr
	for _, container := range snapshot.Containers {
		inScope := settings.Scope == ScopeAll && container.State == "running" ||
			settings.Scope == ScopeActivated && eligibleProjects[container.Project]
		if !inScope {
			continue
		}
		if seen[container.ID] {
			cycleErr = errors.Join(cycleErr, fmt.Errorf("Docker snapshot repeats container ID %s", container.ID))
			continue
		}
		seen[container.ID] = true
		containers = append(containers, container)
		eligible[container.ID] = true
		if _, ok := samplers[container.ID]; !ok {
			sampler, err := NewSampler(c.docker)
			if err != nil {
				return err
			}
			samplers[container.ID] = sampler
		}
	}
	for id := range samplers {
		if !eligible[id] {
			delete(samplers, id)
		}
	}
	continuity.setEligible(eligible)
	connected, generation := continuity.snapshot()
	isContinuous := connected && *previousCycleOK && generation == *previousGeneration
	states := make([]State, 0, len(containers))
	samples := make([]Sample, 0, len(containers))
	if len(containers) > 0 {
		type observationResult struct {
			id     string
			state  State
			sample *Sample
			err    error
		}
		workers := min(collectorWorkers, len(containers))
		jobs := make(chan runtime.Container)
		results := make(chan observationResult)
		var workersDone sync.WaitGroup
		for range workers {
			workersDone.Add(1)
			go func() {
				defer workersDone.Done()
				for container := range jobs {
					state, sample, observeErr := samplers[container.ID].Observe(ctx, container.ID, time.Now().UTC(), isContinuous)
					select {
					case results <- observationResult{id: container.ID, state: state, sample: sample, err: observeErr}:
					case <-ctx.Done():
						return
					}
				}
			}()
		}
		go func() {
			defer close(jobs)
			for _, container := range containers {
				select {
				case jobs <- container:
				case <-ctx.Done():
					return
				}
			}
		}()
		go func() {
			workersDone.Wait()
			close(results)
		}()
		for result := range results {
			if result.state.ContainerID != "" {
				states = append(states, result.state)
			}
			if result.err != nil {
				cycleErr = errors.Join(cycleErr, fmt.Errorf("observe container %s: %w", result.id, result.err))
				continue
			}
			if result.sample != nil {
				samples = append(samples, *result.sample)
			}
		}
		if err := ctx.Err(); err != nil {
			cycleErr = errors.Join(cycleErr, err)
		}
	}
	connectedAfter, generationAfter := continuity.snapshot()
	if !connectedAfter || generationAfter != generation {
		for i := range samples {
			observation := &samples[i].Observation
			observation.CPUPercent = nil
			observation.NetworkRxBytesPerSec = nil
			observation.NetworkTxBytesPerSec = nil
			observation.BlockReadBytesPerSec = nil
			observation.BlockWriteBytesPerSec = nil
		}
	}
	now := time.Now().UTC()
	var stateErr error
	if len(states) != 0 {
		if stateErr = store.InsertStates(ctx, now, states); stateErr != nil {
			stateErr = c.reconcileWrite(ctx, store, stateErr)
			cycleErr = errors.Join(cycleErr, fmt.Errorf("persist container states: %w", stateErr))
		}
	}
	if len(samples) != 0 && stateErr == nil {
		if err := store.Insert(ctx, now, samples); err != nil {
			err = c.reconcileWrite(ctx, store, err)
			cycleErr = errors.Join(cycleErr, fmt.Errorf("persist metric samples: %w", err))
		}
	}
	if len(states) == 0 && len(samples) == 0 && cycleErr == nil {
		if err := store.Maintain(ctx, now); err != nil {
			cycleErr = fmt.Errorf("maintain idle metrics store: %w", err)
		}
	}
	if err := c.refreshEvictions(ctx, store); err != nil {
		cycleErr = errors.Join(cycleErr, err)
	}
	*previousCycleOK = cycleErr == nil
	*previousGeneration = generationAfter
	c.updateStatus(func(status *CollectorStatus) {
		status.Enabled = true
		status.Collecting = cycleErr == nil
		status.Containers = len(containers)
		status.LastCycle = now
		if cycleErr == nil {
			status.Problem = ""
		}
	})
	return cycleErr
}

func (c *Collector) refreshEvictions(ctx context.Context, store *ManagedStore) error {
	status, err := store.Status(ctx)
	if err != nil {
		return fmt.Errorf("read metrics storage status: %w", err)
	}
	c.updateStatus(func(current *CollectorStatus) {
		current.Evictions = status.Evictions
	})
	return nil
}

func (c *Collector) watchEvents(ctx context.Context, store *ManagedStore, continuity *eventContinuity) {
	for ctx.Err() == nil {
		err := c.docker.WatchContainerEventsWithConnect(ctx, func() {
			continuity.setConnected(true)
			c.updateStatus(func(status *CollectorStatus) {
				status.EventConnected = true
				if status.Problem == "Docker event stream disconnected" {
					status.Problem = ""
				}
			})
		}, func(event runtime.ContainerEvent) error {
			if !continuity.contains(event.ID) || event.Time.Before(time.Now().Add(-time.Hour)) {
				return nil
			}
			converted, ok := convertEvent(event)
			if !ok {
				return nil
			}
			err := store.InsertEvents(ctx, time.Now().UTC(), []Event{converted})
			return c.reconcileWrite(ctx, store, err)
		})
		continuity.setConnected(false)
		if ctx.Err() != nil {
			return
		}
		c.updateStatus(func(status *CollectorStatus) {
			status.EventConnected = false
			status.Problem = "Docker event stream disconnected"
		})
		c.logger.WarnContext(ctx, "Docker metrics event stream disconnected", "error", err)
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func convertEvent(event runtime.ContainerEvent) (Event, bool) {
	converted := Event{ContainerID: event.ID, Time: event.Time}
	switch event.Action {
	case "start":
		converted.Kind = EventStart
	case "stop":
		converted.Kind = EventStop
	case "die":
		converted.Kind = EventExit
	case "restart":
		converted.Kind = EventRestart
	case "oom":
		converted.Kind = EventOOM
	case "health_status":
		converted.Kind = EventHealth
		converted.Health = Health(event.Health)
	default:
		return Event{}, false
	}
	return converted, true
}

func (c *Collector) reconcileWrite(ctx context.Context, store *ManagedStore, writeErr error) error {
	if writeErr == nil || !errors.Is(writeErr, ErrWriteOutcomeUnknown) && !errors.Is(writeErr, ErrReconcileWrite) {
		return writeErr
	}
	pending, err := store.ReconcilePendingWrite(ctx)
	if err != nil {
		return errors.Join(writeErr, fmt.Errorf("reconcile uncertain metrics write: %w", err))
	}
	if pending != nil {
		c.logger.WarnContext(ctx, "reconciled uncertain metrics batch without replay",
			"kind", pending.Kind, "started_at", pending.CreatedAt, "committed", pending.Committed)
	}
	return writeErr
}

func (c *Collector) recordCycleError(err error) {
	c.updateStatus(func(status *CollectorStatus) {
		status.Collecting = false
		status.StoragePaused = errors.Is(err, ErrMetricsPaused) || errors.Is(err, ErrWriteCommitted)
		status.Problem = err.Error()
	})
	c.logger.Warn("metrics collection cycle failed", "error", err)
}
