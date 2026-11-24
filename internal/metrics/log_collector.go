package metrics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

const liveLogRefreshInterval = 10 * time.Second

type logFollowTask struct {
	cancel context.CancelFunc
	health *logFollowerHealth
}

type logFollowerHealth struct {
	mu      sync.RWMutex
	problem string
}

func (health *logFollowerHealth) set(problem string) {
	health.mu.Lock()
	health.problem = problem
	health.mu.Unlock()
}

func (health *logFollowerHealth) get() string {
	health.mu.RLock()
	defer health.mu.RUnlock()
	return health.problem
}

func (c *Collector) runLogCapture(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		_, sources, err := c.settings.LoadConfiguration(ctx)
		if err != nil {
			c.setLogProblem(err)
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		settings := sources.Logs
		if !settings.Configured || !settings.Enabled {
			c.updateStatus(func(status *CollectorStatus) {
				status.LogsEnabled = false
				status.LogsCollecting = false
				status.LogsContainers = 0
				status.LogsProblem = ""
			})
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		c.updateStatus(func(status *CollectorStatus) { status.LogsEnabled = true })
		if c.docker == nil {
			c.setLogProblem(errors.New("Docker socket is not configured"))
			if wait(ctx, ticker.C) != nil {
				return
			}
			continue
		}
		err = c.collectLogSession(ctx, settings)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errSettingsChanged) {
			continue
		}
		if err != nil {
			c.setLogProblem(err)
			c.logger.ErrorContext(ctx, "container log collection paused", "error", err)
			if wait(ctx, ticker.C) != nil {
				return
			}
		}
	}
}

func (c *Collector) setLogProblem(err error) {
	c.updateStatus(func(status *CollectorStatus) {
		status.LogsEnabled = true
		status.LogsCollecting = false
		status.LogsProblem = err.Error()
	})
}

func (c *Collector) collectLogSession(ctx context.Context, settings LogSettings) error {
	store, err := OpenManagedLogs(ctx, c.dataDir, settings.StorageLimit)
	if err != nil {
		return fmt.Errorf("open managed log store: %w", err)
	}
	c.logStoreMu.Lock()
	c.logStore = store
	c.logStoreMu.Unlock()
	defer func() {
		c.logStoreMu.Lock()
		c.logStore = nil
		c.logStoreMu.Unlock()
		if err := store.Close(); err != nil {
			c.logger.Error("close managed log store", "error", err)
		}
	}()
	pending, err := store.ReconcilePendingWrite(ctx)
	if err != nil {
		return fmt.Errorf("reconcile interrupted log write: %w", err)
	}
	if pending != nil {
		c.logger.WarnContext(ctx, "reconciled interrupted log batch without replay",
			"kind", pending.Kind, "started_at", pending.CreatedAt, "committed", pending.Committed)
	}
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		c.setLogProblem(err)
	}
	if err := c.refreshLogEvictions(ctx, store); err != nil {
		return err
	}

	sessionCtx, cancel := context.WithCancelCause(ctx)
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-ticker.C:
				_, current, err := c.settings.LoadConfiguration(sessionCtx)
				if err != nil || current.Logs.Revision != settings.Revision ||
					!current.Logs.Enabled || !current.Logs.Configured {
					cancel(errSettingsChanged)
					return
				}
			}
		}
	}()
	defer func() {
		cancel(context.Canceled)
		<-monitorDone
	}()

	if settings.DeliveryPreset == ProfileHigh {
		return c.collectLiveLogs(sessionCtx, settings, store)
	}
	return c.collectPeriodicLogs(sessionCtx, settings, store)
}

func (c *Collector) collectPeriodicLogs(ctx context.Context, settings LogSettings, store *ManagedStore) error {
	interval := 60 * time.Second
	if settings.DeliveryPreset == ProfileMedium {
		interval = 30 * time.Second
	}
	cursors := make(map[string]time.Time)
	maintenance := time.NewTicker(5 * time.Second)
	defer maintenance.Stop()
	collection := time.NewTicker(interval)
	defer collection.Stop()
	if err := c.collectLogBacklog(ctx, settings, store, cursors); err != nil {
		c.setLogProblem(err)
	}
	for {
		select {
		case <-ctx.Done():
			if errors.Is(context.Cause(ctx), errSettingsChanged) {
				return errSettingsChanged
			}
			return ctx.Err()
		case <-maintenance.C:
			if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
				c.setLogProblem(err)
			}
			if err := c.refreshLogEvictions(ctx, store); err != nil {
				c.logger.WarnContext(ctx, "read log storage status", "error", err)
			}
		case <-collection.C:
			if err := c.collectLogBacklog(ctx, settings, store, cursors); err != nil {
				c.setLogProblem(err)
			}
		}
	}
}

func (c *Collector) collectLogBacklog(ctx context.Context, settings LogSettings, store *ManagedStore, cursors map[string]time.Time) error {
	snapshot, err := c.docker.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("list containers for log collection: %w", err)
	}
	projects, scopeErr := c.eligibleProjects(ctx, settings.Scope)
	now := time.Now().UTC()
	containers := make([]runtime.Container, 0, len(snapshot.Containers))
	seen := make(map[string]bool, len(snapshot.Containers))
	for _, container := range snapshot.Containers {
		if !containerInScope(container, settings.Scope, projects) {
			continue
		}
		if seen[container.ID] {
			scopeErr = errors.Join(scopeErr, fmt.Errorf("Docker snapshot repeats container ID %s", container.ID))
			continue
		}
		seen[container.ID] = true
		containers = append(containers, container)
	}
	for _, container := range containers {
		start := cursors[container.ID]
		if start.IsZero() {
			start = now.Add(-time.Hour)
		}
		if start.Before(now.Add(-time.Hour)) {
			start = now.Add(-time.Hour)
		}
		if !start.Before(now) {
			continue
		}
		if err := c.readAndPersistLogs(ctx, store, container.ID, runtime.LogOptions{Since: start, Until: now}, nil,
			func(at time.Time) {
				if cursors[container.ID].Before(at) {
					cursors[container.ID] = at
				}
			}); err != nil {
			scopeErr = errors.Join(scopeErr, fmt.Errorf("collect logs for container %s: %w", container.ID, err))
			continue
		}
		cursors[container.ID] = now
	}
	c.updateStatus(func(status *CollectorStatus) {
		status.LogsEnabled = true
		status.LogsCollecting = scopeErr == nil
		status.LogsContainers = len(containers)
		status.LogsLastCycle = now
		if scopeErr == nil {
			status.LogsProblem = ""
		} else {
			status.LogsProblem = scopeErr.Error()
		}
	})
	if err := c.refreshLogEvictions(ctx, store); err != nil {
		return errors.Join(scopeErr, err)
	}
	return scopeErr
}

func (c *Collector) collectLiveLogs(ctx context.Context, settings LogSettings, store *ManagedStore) error {
	tasks := make(map[string]logFollowTask)
	stoppedRead := make(map[string]bool)
	cursors := &logCursorMap{times: make(map[string]time.Time)}
	var tasksMu sync.Mutex
	var workers sync.WaitGroup
	stopTasks := func() {
		tasksMu.Lock()
		for _, task := range tasks {
			task.cancel()
		}
		tasksMu.Unlock()
		workers.Wait()
	}
	defer stopTasks()

	refresh := time.NewTicker(liveLogRefreshInterval)
	defer refresh.Stop()
	maintenance := time.NewTicker(5 * time.Second)
	defer maintenance.Stop()
	if err := c.refreshLiveLogTargets(ctx, settings, store, tasks, stoppedRead, cursors, &tasksMu, &workers); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			if errors.Is(context.Cause(ctx), errSettingsChanged) {
				return errSettingsChanged
			}
			return ctx.Err()
		case <-refresh.C:
			if err := c.refreshLiveLogTargets(ctx, settings, store, tasks, stoppedRead, cursors, &tasksMu, &workers); err != nil {
				c.setLogProblem(err)
			}
		case <-maintenance.C:
			if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
				c.setLogProblem(err)
			} else if err := c.updateLiveLogStatus(ctx, store, c.Status().LogsContainers, nil, tasks, &tasksMu, time.Now().UTC()); err != nil {
				c.setLogProblem(err)
			}
			if err := c.refreshLogEvictions(ctx, store); err != nil {
				c.logger.WarnContext(ctx, "read log storage status", "error", err)
			}
		}
	}
}

func (c *Collector) refreshLiveLogTargets(
	ctx context.Context,
	settings LogSettings,
	store *ManagedStore,
	tasks map[string]logFollowTask,
	stoppedRead map[string]bool,
	cursors *logCursorMap,
	tasksMu *sync.Mutex,
	workers *sync.WaitGroup,
) error {
	snapshot, err := c.docker.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("refresh live log scope: %w", err)
	}
	projects, scopeErr := c.eligibleProjects(ctx, settings.Scope)
	desired := make(map[string]runtime.Container)
	for _, container := range snapshot.Containers {
		if containerInScope(container, settings.Scope, projects) {
			desired[container.ID] = container
		}
	}
	tasksMu.Lock()
	for id, task := range tasks {
		container, exists := desired[id]
		if !exists || container.State != "running" {
			task.cancel()
			delete(tasks, id)
		}
	}
	tasksMu.Unlock()
	now := time.Now().UTC()
	for id, container := range desired {
		if container.State != "running" {
			if stoppedRead[id] {
				continue
			}
			start := cursors.get(id)
			if start.IsZero() {
				start = now.Add(-time.Hour)
			}
			if start.Before(now.Add(-time.Hour)) {
				start = now.Add(-time.Hour)
			}
			if err := c.readAndPersistLogs(ctx, store, id, runtime.LogOptions{Since: start, Until: now}, nil,
				func(at time.Time) { cursors.advance(id, at) }); err != nil {
				scopeErr = errors.Join(scopeErr, fmt.Errorf("read stopped container %s backlog: %w", id, err))
				continue
			}
			cursors.set(id, now)
			stoppedRead[id] = true
			continue
		}
		delete(stoppedRead, id)
		tasksMu.Lock()
		if _, exists := tasks[id]; exists {
			tasksMu.Unlock()
			continue
		}
		taskCtx, cancel := context.WithCancel(ctx)
		health := &logFollowerHealth{}
		tasks[id] = logFollowTask{cancel: cancel, health: health}
		workers.Add(1)
		tasksMu.Unlock()
		go func(containerID string) {
			defer workers.Done()
			c.followContainerLogs(taskCtx, store, containerID, cursors, health)
		}(id)
	}
	if err := c.updateLiveLogStatus(ctx, store, len(desired), scopeErr, tasks, tasksMu, now); err != nil {
		scopeErr = errors.Join(scopeErr, err)
	}
	if err := c.refreshLogEvictions(ctx, store); err != nil {
		return errors.Join(scopeErr, err)
	}
	return scopeErr
}

func (c *Collector) updateLiveLogStatus(ctx context.Context, store *ManagedStore, containerCount int, scopeErr error, tasks map[string]logFollowTask, tasksMu *sync.Mutex, now time.Time) error {
	problems := []error{scopeErr}
	tasksMu.Lock()
	for _, task := range tasks {
		if problem := task.health.get(); problem != "" {
			problems = append(problems, errors.New(problem))
		}
	}
	tasksMu.Unlock()
	storage, err := store.Status(ctx)
	if err != nil {
		problems = append(problems, fmt.Errorf("read log storage status: %w", err))
	} else if storage.Paused {
		if storage.Problem == nil {
			problems = append(problems, errors.New("log storage is paused"))
		} else {
			problems = append(problems, fmt.Errorf("log storage paused: %w", storage.Problem))
		}
	}
	problem := errors.Join(problems...)
	c.updateStatus(func(status *CollectorStatus) {
		status.LogsEnabled = true
		status.LogsCollecting = problem == nil
		status.LogsContainers = containerCount
		status.LogsLastCycle = now
		if problem == nil {
			status.LogsProblem = ""
		} else {
			status.LogsProblem = problem.Error()
		}
	})
	return problem
}

func (c *Collector) followContainerLogs(ctx context.Context, store *ManagedStore, id string, cursors *logCursorMap, health *logFollowerHealth) {
	start := cursors.get(id)
	if start.IsZero() {
		start = time.Now().UTC().Add(-time.Hour)
	}
	for ctx.Err() == nil {
		if acknowledged := cursors.get(id); !acknowledged.IsZero() {
			start = acknowledged
		}
		err := c.readAndPersistLogs(ctx, store, id, runtime.LogOptions{Since: start, Follow: true},
			func() { health.set("") }, func(at time.Time) { cursors.advance(id, at) })
		if ctx.Err() != nil {
			return
		}
		health.set(fmt.Sprintf("container %s live log stream disconnected; replay is best-effort because Docker provides no stable log record IDs, so messages may be missing or repeated: %v", id, err))
		c.updateStatus(func(status *CollectorStatus) {
			status.LogsCollecting = false
			status.LogsProblem = health.get()
		})
		c.logger.WarnContext(ctx, "live Docker log stream disconnected", "container_id", id, "error", err)
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (c *Collector) readAndPersistLogs(ctx context.Context, store *ManagedStore, id string, options runtime.LogOptions, connected func(), acknowledged func(time.Time)) error {
	batch := make([]Log, 0, maxLogBatch)
	flushFailed := false
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		writeTime := time.Now().UTC()
		cutoff := writeTime.Add(-time.Hour)
		eligible := batch[:0]
		latest := time.Time{}
		for _, record := range batch {
			if record.Time.Before(cutoff) {
				continue
			}
			eligible = append(eligible, record)
			if record.Time.After(latest) {
				latest = record.Time
			}
		}
		if len(eligible) == 0 {
			batch = batch[:0]
			return nil
		}
		err := store.InsertLogs(ctx, writeTime, eligible)
		if err != nil {
			committed := errors.Is(err, ErrWriteCommitted)
			if committed {
				if _, reconcileErr := store.ReconcilePendingWrite(ctx); reconcileErr != nil {
					err = errors.Join(err, fmt.Errorf("reconcile committed log batch: %w", reconcileErr))
				}
			} else if errors.Is(err, ErrWriteOutcomeUnknown) {
				pending, reconcileErr := store.ReconcilePendingWrite(ctx)
				if reconcileErr != nil {
					err = errors.Join(err, fmt.Errorf("reconcile uncertain log batch: %w", reconcileErr))
				} else if pending != nil && pending.Kind == "logs" && pending.Committed {
					committed = true
				}
			} else if errors.Is(err, ErrReconcileWrite) {
				if _, reconcileErr := store.ReconcilePendingWrite(ctx); reconcileErr != nil {
					err = errors.Join(err, fmt.Errorf("reconcile prior log batch: %w", reconcileErr))
				}
			}
			if committed && acknowledged != nil {
				acknowledged(latest)
			}
			batch = batch[:0]
			flushFailed = true
			return err
		}
		if acknowledged != nil {
			acknowledged(latest)
		}
		batch = batch[:0]
		return nil
	}
	err := c.docker.ReadContainerLogsWithConnect(ctx, id, options, connected, func(item runtime.ContainerLog) error {
		batch = append(batch, Log{
			ContainerID: item.ID, Time: item.Time, ReceivedAt: item.ReceivedAt,
			Stream: item.Stream, Message: item.Message, Truncated: item.Truncated,
		})
		if options.Follow || len(batch) == maxLogBatch {
			return flush()
		}
		return nil
	})
	var flushErr error
	if !flushFailed {
		flushErr = flush()
	}
	if err != nil || flushErr != nil {
		return errors.Join(err, flushErr)
	}
	return nil
}

type logCursorMap struct {
	mu    sync.Mutex
	times map[string]time.Time
}

func (cursors *logCursorMap) get(id string) time.Time {
	cursors.mu.Lock()
	defer cursors.mu.Unlock()
	return cursors.times[id]
}

func (cursors *logCursorMap) set(id string, at time.Time) {
	cursors.mu.Lock()
	defer cursors.mu.Unlock()
	cursors.times[id] = at
}

func (cursors *logCursorMap) advance(id string, at time.Time) {
	cursors.mu.Lock()
	defer cursors.mu.Unlock()
	if cursors.times[id].Before(at) {
		cursors.times[id] = at
	}
}

func (c *Collector) refreshLogEvictions(ctx context.Context, store *ManagedStore) error {
	status, err := store.Status(ctx)
	if err != nil {
		return fmt.Errorf("read log storage status: %w", err)
	}
	c.updateStatus(func(current *CollectorStatus) {
		current.LogEvictions = status.LogEvictions
	})
	return nil
}

func (c *Collector) SelectLogs(ctx context.Context, id string, start, end time.Time, limit int) (LogPage, error) {
	c.logStoreMu.RLock()
	defer c.logStoreMu.RUnlock()
	if c.logStore == nil {
		return LogPage{Logs: []Log{}}, nil
	}
	return c.logStore.SelectLogs(ctx, id, start, end, limit)
}

func (c *Collector) SelectObservedContainersForSources(ctx context.Context, start, end time.Time, stats, states, logs bool) ([]string, error) {
	seen := make(map[string]bool)
	if stats || states {
		c.storeMu.RLock()
		if c.store != nil {
			ids, err := c.store.SelectObservedContainersForSources(ctx, start, end, stats, states, states, false)
			c.storeMu.RUnlock()
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				seen[id] = true
			}
		} else {
			c.storeMu.RUnlock()
		}
	}
	if logs {
		c.logStoreMu.RLock()
		if c.logStore != nil {
			ids, err := c.logStore.SelectObservedContainers(ctx, start, end)
			c.logStoreMu.RUnlock()
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				seen[id] = true
			}
		} else {
			c.logStoreMu.RUnlock()
		}
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}
