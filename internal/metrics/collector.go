package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
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
	Enabled         bool
	Collecting      bool
	EventConnected  bool
	StoragePaused   bool
	Containers      int
	LastCycle       time.Time
	Problem         string
	Evictions       EvictedSamples
	LogsEnabled     bool
	LogsCollecting  bool
	LogsContainers  int
	LogsLastCycle   time.Time
	LogsProblem     string
	LogEvictions    EvictedLogs
	StateEnabled    bool
	StateCollecting bool
	StateProblem    string
}

type Collector struct {
	dataDir              string
	settings             *SettingsStore
	projects             projectReader
	docker               *runtime.Client
	logger               *slog.Logger
	stateRefreshInterval time.Duration
	storeMu              sync.RWMutex
	store                *ManagedStore
	logStoreMu           sync.RWMutex
	logStore             *ManagedStore
	statePersistMu       sync.Mutex
	stateMu              sync.Mutex
	stateCache           map[string]State
	stateWatcherProblem  string
	stateControlProblem  string
	stateStorageProblem  string
	stateWatcherHealthy  bool
	mu                   sync.RWMutex
	status               CollectorStatus
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
	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		c.runLogCapture(ctx)
	}()
	stateDone := make(chan struct{})
	go func() {
		defer close(stateDone)
		c.runStateCapture(ctx)
	}()
	defer func() {
		<-logsDone
		<-stateDone
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		settings, sources, err := c.settings.LoadConfiguration(ctx)
		if err != nil {
			c.problem(err)
			if err := wait(ctx, ticker.C); err != nil {
				return err
			}
			continue
		}
		if !settings.Enabled {
			c.updateStatus(func(status *CollectorStatus) {
				eventConnected := status.EventConnected
				logsEnabled, logsCollecting := status.LogsEnabled, status.LogsCollecting
				logContainers, logCycle, logProblem, logEvictions := status.LogsContainers, status.LogsLastCycle, status.LogsProblem, status.LogEvictions
				stateEnabled, stateCollecting, stateProblem := status.StateEnabled, status.StateCollecting, status.StateProblem
				*status = CollectorStatus{
					EventConnected: eventConnected,
					LogsEnabled:    logsEnabled, LogsCollecting: logsCollecting, LogsContainers: logContainers,
					LogsLastCycle: logCycle, LogsProblem: logProblem, LogEvictions: logEvictions,
					StateEnabled: stateEnabled, StateCollecting: stateCollecting, StateProblem: stateProblem,
				}
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
		if err := c.collect(ctx, settings, sources); err != nil && !errors.Is(err, errSettingsChanged) && ctx.Err() == nil {
			c.problem(err)
			c.logger.ErrorContext(ctx, "metrics collection paused", "error", err)
			if err := wait(ctx, ticker.C); err != nil {
				return err
			}
		}
	}
}

func (c *Collector) LogsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status.LogsEnabled
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

func (c *Collector) collect(ctx context.Context, settings Settings, sources SourceSettings) error {
	var err error
	store, err := OpenManaged(ctx, c.dataDir, settings.StorageLimit)
	if err != nil {
		return fmt.Errorf("open managed metrics store: %w", err)
	}
	c.storeMu.Lock()
	c.store = store
	c.storeMu.Unlock()
	defer func() {
		c.storeMu.Lock()
		c.store = nil
		if err := store.Close(); err != nil {
			c.logger.Error("close managed metrics store", "error", err)
		}
		c.storeMu.Unlock()
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
		logsEnabled, logsCollecting := status.LogsEnabled, status.LogsCollecting
		logContainers, logCycle, logProblem, logEvictions := status.LogsContainers, status.LogsLastCycle, status.LogsProblem, status.LogEvictions
		stateEnabled, stateCollecting, stateProblem := status.StateEnabled, status.StateCollecting, status.StateProblem
		*status = CollectorStatus{
			Enabled: true, Collecting: true,
			LogsEnabled: logsEnabled, LogsCollecting: logsCollecting, LogsContainers: logContainers,
			LogsLastCycle: logCycle, LogsProblem: logProblem, LogEvictions: logEvictions,
			StateEnabled: stateEnabled, StateCollecting: stateCollecting, StateProblem: stateProblem,
		}
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

	continuity := &eventContinuity{}
	samplers := make(map[string]*Sampler)
	c.stateMu.Lock()
	c.stateCache = make(map[string]State)
	c.stateMu.Unlock()
	previousCycleOK := false
	_, previousGeneration := continuity.snapshot()
	if !c.Status().StoragePaused {
		if err := c.collectCycleWithScopes(ctx, settings, sources.StateScope, store, continuity, samplers, &previousCycleOK, &previousGeneration, sources.StateEnabled); err != nil {
			c.recordCycleError(err)
		}
	}
	streamCtx, cancelStream := context.WithCancel(ctx)
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		c.watchEvents(streamCtx, store, continuity, sources.StateEnabled, sources.StateScope)
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
			current, currentSources, err := c.settings.LoadConfiguration(ctx)
			if err != nil {
				c.recordCycleError(err)
				continue
			}
			if current.Revision != settings.Revision || !current.Enabled ||
				currentSources.StateRevision != sources.StateRevision ||
				currentSources.CollectionRevision != sources.CollectionRevision {
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
				c.clearStateStorageProblem()
			}
			if err := c.refreshEvictions(ctx, store); err != nil {
				c.recordCycleError(err)
			}
		case <-sampleTicker.C:
			if c.Status().StoragePaused {
				previousCycleOK = false
				continue
			}
			if err := c.collectCycleWithScopes(ctx, settings, sources.StateScope, store, continuity, samplers, &previousCycleOK, &previousGeneration, sources.StateEnabled); err != nil {
				c.recordCycleError(err)
			}
		}
	}
}

func (c *Collector) SelectChartSeries(ctx context.Context, id string, start, end time.Time, bucket time.Duration) ([]ContainerChartSeries, error) {
	c.storeMu.RLock()
	defer c.storeMu.RUnlock()
	if c.store == nil {
		return nil, nil
	}
	return c.store.SelectChartSeries(ctx, id, start, end, bucket)
}

func (c *Collector) SelectTimelineEvents(ctx context.Context, id string, start, end time.Time, limit int) ([]Event, error) {
	c.storeMu.RLock()
	defer c.storeMu.RUnlock()
	if c.store == nil {
		return nil, nil
	}
	return c.store.SelectTimelineEvents(ctx, id, start, end, limit)
}

func (c *Collector) SelectStates(ctx context.Context, id string, start, end time.Time, limit int) ([]State, error) {
	c.storeMu.RLock()
	defer c.storeMu.RUnlock()
	if c.store == nil {
		return nil, nil
	}
	return c.store.SelectStates(ctx, id, start, end, limit)
}

func (c *Collector) SelectStateHistory(ctx context.Context, id string, start, end time.Time, limit int) (StateHistory, error) {
	c.storeMu.RLock()
	defer c.storeMu.RUnlock()
	if c.store == nil {
		return StateHistory{}, nil
	}
	return c.store.SelectStateHistory(ctx, id, start, end, limit)
}

func (c *Collector) SelectObservedContainers(ctx context.Context, start, end time.Time) ([]string, error) {
	c.storeMu.RLock()
	defer c.storeMu.RUnlock()
	if c.store == nil {
		return nil, nil
	}
	return c.store.SelectObservedContainers(ctx, start, end)
}

func (c *Collector) collectCycle(ctx context.Context, settings Settings, store *ManagedStore, continuity *eventContinuity, samplers map[string]*Sampler, previousCycleOK *bool, previousGeneration *uint64) error {
	return c.collectCycleWithState(ctx, settings, store, continuity, samplers, previousCycleOK, previousGeneration, true)
}

func (c *Collector) collectCycleWithState(ctx context.Context, settings Settings, store *ManagedStore, continuity *eventContinuity, samplers map[string]*Sampler, previousCycleOK *bool, previousGeneration *uint64, persistStates bool) error {
	return c.collectCycleWithScopes(ctx, settings, settings.Scope, store, continuity, samplers, previousCycleOK, previousGeneration, persistStates)
}

func (c *Collector) collectCycleWithScopes(ctx context.Context, settings Settings, stateScope CollectionScope, store *ManagedStore, continuity *eventContinuity, samplers map[string]*Sampler, previousCycleOK *bool, previousGeneration *uint64, persistStates bool) error {
	snapshot, err := c.docker.Snapshot(ctx)
	if err != nil {
		*previousCycleOK = false
		if persistStates {
			if gapErr := store.StartGlobalStateGap(ctx, time.Now().UTC()); gapErr != nil {
				err = errors.Join(err, fmt.Errorf("record unavailable state observation: %w", gapErr))
			}
		}
		return fmt.Errorf("list Docker containers: %w", err)
	}
	metricsProjects, scopeErr := c.eligibleProjects(ctx, settings.Scope)
	if scopeErr != nil {
		c.logger.WarnContext(ctx, "partially resolved metrics scope", "error", scopeErr)
		*previousCycleOK = false
	}
	stateProjects := metricsProjects
	var stateScopeErr error
	if persistStates && stateScope == settings.Scope {
		stateScopeErr = scopeErr
	}
	if persistStates && stateScope != settings.Scope {
		stateProjects, stateScopeErr = c.eligibleProjects(ctx, stateScope)
		if stateScopeErr != nil {
			c.logger.WarnContext(ctx, "partially resolved state collection scope", "error", stateScopeErr)
			scopeErr = errors.Join(scopeErr, stateScopeErr)
		}
	}

	type target struct {
		container    runtime.Container
		collectStats bool
		collectState bool
	}
	targets := make([]target, 0, len(snapshot.Containers))
	metricsEligible := make(map[string]bool)
	stateEligible := make(map[string]bool)
	seen := make(map[string]bool, len(snapshot.Containers))
	cycleErr := scopeErr
	stateReconciliationOK := persistStates && stateScopeErr == nil
	for _, container := range snapshot.Containers {
		collectStats := containerInScope(container, settings.Scope, metricsProjects) &&
			(settings.Scope != ScopeAll || container.State == "running")
		collectState := persistStates && containerInScope(container, stateScope, stateProjects)
		if !collectStats && !collectState {
			continue
		}
		if seen[container.ID] {
			cycleErr = errors.Join(cycleErr, fmt.Errorf("Docker snapshot repeats container ID %s", container.ID))
			if collectState {
				stateReconciliationOK = false
			}
			continue
		}
		seen[container.ID] = true
		targets = append(targets, target{container: container, collectStats: collectStats, collectState: collectState})
		if collectStats {
			metricsEligible[container.ID] = true
		}
		if collectState {
			stateEligible[container.ID] = true
		}
		if collectStats {
			if _, ok := samplers[container.ID]; ok {
				continue
			}
			sampler, err := NewSampler(c.docker)
			if err != nil {
				return err
			}
			samplers[container.ID] = sampler
		}
	}
	for id := range samplers {
		if !metricsEligible[id] {
			delete(samplers, id)
		}
	}
	connected, generation := continuity.snapshot()
	isContinuous := connected && *previousCycleOK && generation == *previousGeneration
	states := make([]State, 0, len(stateEligible))
	samples := make([]Sample, 0, len(metricsEligible))
	unavailableStateIDs := make([]string, 0)
	if len(targets) > 0 {
		type observationResult struct {
			id          string
			state       State
			sample      *Sample
			stateTarget bool
			err         error
		}
		workers := min(collectorWorkers, len(targets))
		jobs := make(chan target)
		results := make(chan observationResult)
		var workersDone sync.WaitGroup
		for range workers {
			workersDone.Add(1)
			go func() {
				defer workersDone.Done()
				for item := range jobs {
					container := item.container
					var state State
					var sample *Sample
					var observeErr error
					if item.collectStats {
						state, sample, observeErr = samplers[container.ID].Observe(ctx, container.ID, time.Now().UTC(), isContinuous)
					} else if item.collectState {
						var inspection runtime.ContainerInspection
						inspection, observeErr = c.docker.InspectContainer(ctx, container.ID)
						if observeErr == nil {
							state = stateFromInspection(inspection, time.Now().UTC())
							observeErr = validateObservedState(container.ID, state)
						}
					}
					select {
					case results <- observationResult{id: container.ID, state: state, sample: sample, stateTarget: item.collectState, err: observeErr}:
					case <-ctx.Done():
						return
					}
				}
			}()
		}
		go func() {
			defer close(jobs)
			for _, item := range targets {
				select {
				case jobs <- item:
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
			if result.stateTarget && result.state.ContainerID != "" {
				states = append(states, result.state)
			} else if result.stateTarget && result.err != nil {
				unavailableStateIDs = append(unavailableStateIDs, result.id)
				stateReconciliationOK = false
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
	if persistStates {
		c.statePersistMu.Lock()
		for _, id := range unavailableStateIDs {
			if err := startStateGapIfPreviouslyObserved(ctx, store, nil, id, now); err != nil {
				stateErr = errors.Join(stateErr, fmt.Errorf("record unavailable state observation for %s: %w", id, err))
			}
		}
		if len(states) != 0 {
			changed, err := c.changedStates(ctx, store, states)
			if err != nil {
				stateErr = errors.Join(stateErr, err)
			} else if len(changed) != 0 {
				writeErr := store.InsertStates(ctx, now, changed)
				if writeErr == nil {
					for _, state := range changed {
						gapOpen, err := store.HasOpenStateGap(ctx, state.ContainerID)
						if err != nil {
							writeErr = errors.Join(writeErr, fmt.Errorf("read state observation gap for %s: %w", state.ContainerID, err))
							continue
						}
						if gapOpen {
							if err := store.EndStateGap(ctx, state.ContainerID, state.Time); err != nil {
								writeErr = errors.Join(writeErr, fmt.Errorf("end state observation gap for %s: %w", state.ContainerID, err))
							}
						}
					}
				}
				if writeErr == nil {
					c.rememberStates(changed)
					c.clearStateStorageProblem()
				}
				stateErr = errors.Join(stateErr, writeErr)
			}
		}
		if stateReconciliationOK && stateErr == nil {
			globalGapOpen, err := store.HasOpenGlobalStateGap(ctx)
			if err != nil {
				stateErr = errors.Join(stateErr, fmt.Errorf("read shared state observation gap: %w", err))
			} else if globalGapOpen {
				if err := store.EndGlobalStateGap(ctx, now); err != nil {
					stateErr = errors.Join(stateErr, fmt.Errorf("end shared state observation gap: %w", err))
				}
			}
		}
		c.statePersistMu.Unlock()
		if stateErr != nil {
			stateErr = c.reconcileWrite(ctx, store, stateErr)
			cycleErr = errors.Join(cycleErr, fmt.Errorf("persist container states: %w", stateErr))
		}
	}
	if len(samples) != 0 {
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
		status.Containers = len(metricsEligible)
		status.LastCycle = now
		if cycleErr == nil {
			status.Problem = ""
		}
	})
	return cycleErr
}

func (c *Collector) changedStates(ctx context.Context, store *ManagedStore, states []State) ([]State, error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.stateCache == nil {
		c.stateCache = make(map[string]State)
	}
	globalGapStart, globalGapOpen, err := store.OpenGlobalStateGap(ctx)
	if err != nil {
		return nil, fmt.Errorf("read shared state observation gap: %w", err)
	}
	changed := make([]State, 0, len(states))
	for i := range states {
		current := states[i]
		previous, known := c.stateCache[current.ContainerID]
		if !known {
			latest, err := store.SelectLatestState(ctx, current.ContainerID)
			if err != nil {
				return nil, fmt.Errorf("read previous state for %s: %w", current.ContainerID, err)
			}
			if latest != nil {
				previous, known = *latest, true
				c.stateCache[current.ContainerID] = previous
			}
		}
		gapStart, gapOpen, err := store.OpenStateGap(ctx, current.ContainerID)
		if err != nil {
			return nil, fmt.Errorf("read open state gap for %s: %w", current.ContainerID, err)
		}
		if globalGapOpen {
			current.ReconnectObserved = true
			states[i] = current
		}
		if known && previous.State == current.State && previous.Health == current.Health {
			if !gapOpen && !globalGapOpen {
				continue
			}
			relevantGapStart := gapStart
			if globalGapStart.After(relevantGapStart) {
				relevantGapStart = globalGapStart
			}
			if previous.ReconnectObserved && !previous.Time.Before(relevantGapStart) {
				if gapOpen {
					if err := store.EndStateGap(ctx, current.ContainerID, previous.Time); err != nil {
						return nil, fmt.Errorf("end state observation gap for %s: %w", current.ContainerID, err)
					}
				}
				continue
			}
		}
		changed = append(changed, current)
	}
	return changed, nil
}

func (c *Collector) cachedStateIDs() []string {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	ids := make([]string, 0, len(c.stateCache))
	for id := range c.stateCache {
		ids = append(ids, id)
	}
	return ids
}

func validateObservedState(id string, state State) error {
	if state.ContainerID != id {
		return errors.New("inspected container ID does not match request")
	}
	_, err := validateState(state)
	return err
}

func (c *Collector) rememberStates(states []State) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.stateCache == nil {
		c.stateCache = make(map[string]State)
	}
	for _, state := range states {
		c.stateCache[state.ContainerID] = state
	}
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

func (c *Collector) watchEvents(ctx context.Context, store *ManagedStore, continuity *eventContinuity, persistEvents bool, scope CollectionScope) {
	known := make(map[string]State)
	var stateWatchMu sync.Mutex
	projectsByName := make(map[string]bool)
	var connected atomic.Bool
	refresh := func() error {
		projects, scopeErr := c.eligibleProjects(ctx, scope)
		if scopeErr != nil {
			err := fmt.Errorf("partially resolved state collection scope: %w", scopeErr)
			c.setStateWatcherProblem(err)
			return err
		}
		stateWatchMu.Lock()
		defer stateWatchMu.Unlock()
		projectsByName = projects
		c.statePersistMu.Lock()
		err := c.reconcileStateSnapshot(ctx, store, scope, projectsByName, known)
		c.statePersistMu.Unlock()
		if err != nil {
			c.setStateWatcherProblem(err)
			return err
		}
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
				if persistEvents && scope == ScopeActivated && connected.Load() {
					_ = refresh()
				}
			}
		}
	}()
	defer func() { <-refreshDone }()
	for ctx.Err() == nil {
		err := c.docker.WatchContainerEventsWithConnect(ctx, func() {
			continuity.setConnected(true)
			connected.Store(true)
			c.updateStatus(func(status *CollectorStatus) {
				status.EventConnected = true
				if status.Problem == "Docker event stream disconnected" {
					status.Problem = ""
				}
			})
			if persistEvents {
				_ = refresh()
			}
		}, func(event runtime.ContainerEvent) error {
			if event.Time.Before(time.Now().Add(-time.Hour)) {
				return nil
			}
			if !persistEvents {
				return nil
			}
			stateWatchMu.Lock()
			defer stateWatchMu.Unlock()
			if !containerInScope(runtime.Container{ID: event.ID, Project: event.Project}, scope, projectsByName) {
				return nil
			}
			converted, ok := convertEvent(event)
			if !ok {
				return nil
			}
			err := c.reconcileWrite(ctx, store, store.InsertEvents(ctx, time.Now().UTC(), []Event{converted}))
			if err != nil {
				c.setStateStorageProblem(err)
				return err
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
					return nil
				}
				c.statePersistMu.Lock()
				err = c.persistObservedState(ctx, store, known, stateFromInspection(inspection, time.Now().UTC()))
				c.statePersistMu.Unlock()
				if err != nil {
					c.setStateStorageProblem(err)
					return err
				}
			}
			return nil
		})
		connected.Store(false)
		continuity.setConnected(false)
		if ctx.Err() != nil {
			return
		}
		var gapErr error
		if persistEvents {
			gapErr = store.StartGlobalStateGap(ctx, time.Now().UTC())
			if gapErr != nil {
				c.logger.ErrorContext(ctx, "record shared state-source disconnect", "error", gapErr)
				c.setStateStorageProblem(gapErr)
			}
		}
		c.updateStatus(func(status *CollectorStatus) {
			status.EventConnected = false
			status.Problem = "Docker event stream disconnected"
			if persistEvents {
				status.StateCollecting = false
			}
		})
		if persistEvents {
			c.setStateWatcherProblem(errors.New("Docker event stream disconnected; state history may have gaps."))
		}
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

func (c *Collector) stateRefreshEvery() time.Duration {
	if c.stateRefreshInterval > 0 {
		return c.stateRefreshInterval
	}
	return stateScopeRefreshInterval
}

func convertEvent(event runtime.ContainerEvent) (Event, bool) {
	converted := Event{ContainerID: event.ID, Time: event.Time}
	switch event.Action {
	case "create":
		converted.Kind = EventCreate
	case "start":
		converted.Kind = EventStart
	case "stop":
		converted.Kind = EventStop
	case "die":
		converted.Kind = EventExit
	case "destroy":
		converted.Kind = EventDestroy
	case "restart":
		converted.Kind = EventRestart
	case "pause":
		converted.Kind = EventPause
	case "unpause":
		converted.Kind = EventUnpause
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
