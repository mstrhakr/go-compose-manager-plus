package web

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

const chartBucket = 30 * time.Second
const timelineEventLimit = 200

var chartColors = []string{"blue", "amber", "purple", "green", "red"}

type dashboardMetricsCollector interface {
	Status() metrics.CollectorStatus
	SelectChartSeries(context.Context, time.Time, time.Time, time.Duration) ([]metrics.ContainerChartSeries, error)
	SelectTimelineEvents(context.Context, string, time.Time, time.Time, int) ([]metrics.Event, error)
	SelectObservedContainers(context.Context, time.Time, time.Time) ([]string, error)
}

type dashboardChartData struct {
	Min          float64           `json:"min"`
	Max          float64           `json:"max"`
	XMin         int64             `json:"xMin"`
	XMax         int64             `json:"xMax"`
	SelectedTime int64             `json:"selectedTime"`
	XLabel       string            `json:"xLabel"`
	YLabel       string            `json:"yLabel"`
	Series       []dashboardSeries `json:"series"`
}

type dashboardSeries struct {
	Label  string               `json:"label"`
	Color  string               `json:"color"`
	Points []dashboardDataPoint `json:"points"`
}

type dashboardDataPoint struct {
	X int64    `json:"x"`
	Y *float64 `json:"y"`
}

type dashboardAggregateMetric struct {
	Label string
	Color string
	Value func(metrics.ChartPoint) *float64
}

func (server *authServer) dashboardCharts(ctx context.Context) ([]chartPanel, string) {
	start, end := dashboardTimeRange()
	return server.dashboardChartsForRange(ctx, start, end, true, "")
}

func dashboardTimeRange() (time.Time, time.Time) {
	end := time.Now().UTC().Truncate(time.Second)
	start := end.Add(-time.Hour)
	return start, end
}

func (server *authServer) dashboardChartsForRange(ctx context.Context, start, end time.Time, aggregate bool, containerID string) ([]chartPanel, string) {
	panels := emptyDashboardCharts("Collection disabled", "No metrics are being collected. Enable opt-in collection in Settings to populate these charts.", aggregate)
	settings, err := server.metricsSettings.Load(ctx)
	if err != nil {
		server.logger.ErrorContext(ctx, "load dashboard metrics settings", "error", err)
		return emptyDashboardCharts("Unavailable", "Metrics settings could not be loaded.", aggregate), "Unavailable"
	}
	if !settings.Enabled {
		return panels, "Disabled"
	}
	if !aggregate && containerID == "" {
		return emptyDashboardCharts("No container selected", "Select a container to view its last-hour history.", false), "No selection"
	}
	status := server.collector.Status()
	series, err := server.collector.SelectChartSeries(ctx, start, end, chartBucket)
	if err != nil {
		server.logger.ErrorContext(ctx, "read dashboard metrics", "error", err)
		return emptyDashboardCharts("Unavailable", "Stored metrics could not be loaded.", aggregate), "Unavailable"
	}
	if !aggregate {
		selected := []metrics.ContainerChartSeries{}
		for _, container := range series {
			if container.ContainerID == containerID {
				selected = append(selected, container)
				break
			}
		}
		series = selected
	}
	if len(series) == 0 {
		if status.Problem != "" {
			return emptyDashboardCharts("Collection degraded", "Metrics collection is reporting a problem. Check Settings for collection status; no retained history is available to display yet.", aggregate), "Collection degraded"
		}
		if !status.Collecting {
			return emptyDashboardCharts("Collector starting", "The metrics collector is starting; no dashboard history is available yet.", aggregate), "Starting"
		}
		return emptyDashboardCharts("Waiting for observations", "No observations were recorded during the last hour.", aggregate), "Collecting"
	}
	containerCount := fmt.Sprintf("%d containers", len(series))
	if len(series) == 1 {
		containerCount = "1 container"
	}
	countStatus := "Last hour · " + containerCount
	if aggregate {
		countStatus = "Last hour aggregate · " + containerCount
	}
	metricsStatus := "Last hour"
	if status.Problem != "" {
		countStatus = "Collection degraded · " + containerCount + " with retained history"
		if aggregate {
			countStatus = "Collection degraded · aggregate of " + containerCount + " with retained history"
		}
		metricsStatus = "Collection degraded"
	} else if !status.Collecting {
		countStatus = "Collector starting · " + containerCount + " with retained history"
		if aggregate {
			countStatus = "Collector starting · aggregate of " + containerCount + " with retained history"
		}
		metricsStatus = "Starting"
	}
	memoryMiB := func(point metrics.ChartPoint) *float64 {
		if point.MemoryUsageBytes == nil {
			return nil
		}
		value := float64(*point.MemoryUsageBytes) / (1024 * 1024)
		return &value
	}
	rateMiB := func(value *float64) *float64 {
		if value == nil {
			return nil
		}
		converted := *value / (1024 * 1024)
		return &converted
	}
	type chartDefinition struct {
		id             string
		containerTitle string
		aggregateTitle string
		yLabel         string
		aggregate      []dashboardAggregateMetric
	}
	definitions := []chartDefinition{
		{
			id: "dashboard-cpu", containerTitle: "CPU usage", aggregateTitle: "CPU across collected containers", yLabel: "CPU (%)",
			aggregate: []dashboardAggregateMetric{{Label: "Collected container total", Color: "blue", Value: func(point metrics.ChartPoint) *float64 { return point.CPUPercent }}},
		},
		{
			id: "dashboard-memory", containerTitle: "Memory usage (Docker raw)", aggregateTitle: "Raw memory across collected containers", yLabel: "MiB",
			aggregate: []dashboardAggregateMetric{{Label: "Collected container total", Color: "amber", Value: memoryMiB}},
		},
		{
			id: "dashboard-network-rx", containerTitle: "Network throughput", aggregateTitle: "Network throughput across collected containers", yLabel: "MiB/s",
			aggregate: []dashboardAggregateMetric{
				{Label: "Receive", Color: "blue", Value: func(point metrics.ChartPoint) *float64 { return rateMiB(point.NetworkRxBytesPerSec) }},
				{Label: "Transmit", Color: "amber", Value: func(point metrics.ChartPoint) *float64 { return rateMiB(point.NetworkTxBytesPerSec) }},
			},
		},
	}
	panels = make([]chartPanel, 0, len(definitions))
	for _, definition := range definitions {
		title := definition.containerTitle
		var panel chartPanel
		var err error
		if aggregate {
			title = definition.aggregateTitle
			panel, err = buildAggregateDashboardChart(definition.id, title, definition.yLabel, series, start, end, chartBucket, definition.aggregate)
		} else {
			panel, err = buildDashboardChart(definition.id, title, definition.yLabel, series, start, end, definition.aggregate)
		}
		if err != nil {
			server.logger.ErrorContext(ctx, "encode dashboard chart", "error", err)
			panel = chartPanel{ID: definition.id, Title: title, Status: "Unavailable", EmptyTitle: "Chart unavailable", EmptyDetail: "Stored chart data could not be displayed."}
		}
		panels = append(panels, panel)
	}
	for i := range panels {
		if panels[i].Status != "Unavailable" {
			panels[i].Status = countStatus
		}
	}
	return panels, metricsStatus
}

type dashboardTimeline struct {
	Start        int64
	End          int64
	SelectedTime int64
	StartLabel   string
	EndLabel     string
	Status       string
	Message      string
	Events       []dashboardTimelineEvent
	RecentEvents []dashboardTimelineEvent
	Truncated    bool
}

type dashboardTimelineEvent struct {
	Time        int64
	TimeLabel   string
	Position    float64
	Container   string
	Description string
	Kind        string
}

func (server *authServer) dashboardTimeline(ctx context.Context, start, end time.Time, containerID string) dashboardTimeline {
	timeline := dashboardTimeline{
		Start: start.UnixMilli(), End: end.UnixMilli(), SelectedTime: end.UnixMilli(),
		StartLabel: start.Format("Jan 2 · 15:04 MST"), EndLabel: end.Format("Jan 2 · 15:04 MST"),
		Status: "Unavailable", Message: "Timeline data could not be loaded.",
		Events: []dashboardTimelineEvent{}, RecentEvents: []dashboardTimelineEvent{},
	}
	settings, err := server.metricsSettings.Load(ctx)
	if err != nil {
		server.logger.ErrorContext(ctx, "load observability timeline settings", "error", err)
		return timeline
	}
	if !settings.Enabled {
		timeline.Status = "Disabled"
		timeline.Message = "Metrics collection is disabled. Enable opt-in collection in Settings to populate this timeline."
		return timeline
	}
	if containerID == "" {
		timeline.Status = "No selection"
		timeline.Message = "Select a container to view its last-hour history."
		return timeline
	}
	events, err := server.collector.SelectTimelineEvents(ctx, containerID, start, end, timelineEventLimit+1)
	if err != nil {
		server.logger.ErrorContext(ctx, "read observability timeline events", "error", err)
		return timeline
	}
	if len(events) > timelineEventLimit {
		timeline.Truncated = true
		events = events[len(events)-timelineEventLimit:]
	}
	for _, event := range events {
		description := timelineEventDescription(event)
		timeline.Events = append(timeline.Events, dashboardTimelineEvent{
			Time:        event.Time.UnixMilli(),
			TimeLabel:   event.Time.UTC().Format("Jan 2 · 15:04:05 UTC"),
			Position:    float64(event.Time.Sub(start)) / float64(end.Sub(start)) * 100,
			Container:   "Container " + shortContainerID(event.ContainerID),
			Description: description,
			Kind:        string(event.Kind),
		})
	}
	for i := len(timeline.Events) - 1; i >= 0; i-- {
		timeline.RecentEvents = append(timeline.RecentEvents, timeline.Events[i])
	}
	status := server.collector.Status()
	switch {
	case status.Problem != "":
		timeline.Status = "Collection degraded"
	case !status.Collecting:
		timeline.Status = "Collector starting"
	case !status.EventConnected:
		timeline.Status = "Event stream disconnected"
	default:
		timeline.Status = fmt.Sprintf("%d observed events", len(timeline.Events))
		if len(timeline.Events) == 1 {
			timeline.Status = "1 observed event"
		}
	}
	if timeline.Truncated {
		timeline.Message = fmt.Sprintf("Showing the newest %d recorded events. Docker event history may be incomplete.", timelineEventLimit)
	} else if status.Problem != "" {
		timeline.Message = "Collection is reporting a problem; stored events are shown, but recent history may be incomplete."
	} else if !status.Collecting {
		timeline.Message = "The metrics collector is starting. Previously recorded events may still appear below."
	} else if !status.EventConnected {
		timeline.Message = "The Docker event stream is disconnected. Recorded events may be incomplete."
	} else if len(events) == 0 {
		timeline.Message = "No lifecycle events were recorded in the last hour. Docker event history may be incomplete."
	} else {
		timeline.Message = "Observed lifecycle events only; Docker may not provide a complete event history."
	}
	return timeline
}

type observedContainer struct {
	ID      string
	Label   string
	State   string
	Href    string
	Current bool
}

func (server *authServer) observabilityContainers(ctx context.Context, start, end time.Time, snapshot []runtime.Container, query url.Values) ([]observedContainer, string, inventoryFilters, error) {
	settings, err := server.metricsSettings.Load(ctx)
	if err != nil {
		return nil, "", inventoryFilters{}, fmt.Errorf("load observability settings: %w", err)
	}
	var ids []string
	if settings.Enabled {
		ids, err = server.collector.SelectObservedContainers(ctx, start, end)
		if err != nil {
			return nil, "", inventoryFilters{}, fmt.Errorf("load observed containers: %w", err)
		}
	}
	inventory := append([]runtime.Container{}, snapshot...)
	seen := make(map[string]bool, len(inventory))
	for _, container := range inventory {
		if container.ID == "" {
			return nil, "", inventoryFilters{}, fmt.Errorf("snapshot container %q has no identity", container.Name)
		}
		seen[container.ID] = true
	}
	for _, id := range ids {
		if !seen[id] {
			inventory = append(inventory, runtime.Container{ID: id, Name: "Container " + shortContainerID(id), State: "History only"})
		}
	}
	filters, _, inventory := filterInventory("containers", query, nil, inventory)
	filters.Path = "/containers/observability"
	selected := query.Get("container")
	if selected == "" && len(inventory) > 0 {
		selected = inventory[0].ID
	}
	containers := make([]observedContainer, 0, len(inventory))
	found := selected == ""
	linkQuery := url.Values{}
	for _, key := range []string{"q", "sort", "state", "group", "project"} {
		linkQuery[key] = query[key]
	}
	for _, container := range inventory {
		current := container.ID == selected
		found = found || current
		label := container.Name
		if label == "" {
			label = "Container " + shortContainerID(container.ID)
		}
		linkQuery.Set("container", container.ID)
		containers = append(containers, observedContainer{
			ID: container.ID, Label: label, State: container.State,
			Href: "/containers/observability?" + linkQuery.Encode(), Current: current,
		})
	}
	if !found {
		return containers, "", filters, nil
	}
	return containers, selected, filters, nil
}

func timelineEventDescription(event metrics.Event) string {
	switch event.Kind {
	case metrics.EventStart:
		return "Started"
	case metrics.EventStop:
		return "Stopped"
	case metrics.EventExit:
		return "Exited"
	case metrics.EventRestart:
		return "Restarted"
	case metrics.EventOOM:
		return "Out of memory"
	case metrics.EventHealth:
		return "Health: " + string(event.Health)
	default:
		return "Container event"
	}
}

func emptyDashboardCharts(status, detail string, aggregate bool) []chartPanel {
	if aggregate {
		return []chartPanel{
			{ID: "dashboard-cpu", Title: "CPU across collected containers", Status: status, EmptyTitle: "Container CPU unavailable", EmptyDetail: detail},
			{ID: "dashboard-memory", Title: "Raw memory across collected containers", Status: status, EmptyTitle: "Container memory unavailable", EmptyDetail: detail},
			{ID: "dashboard-network-rx", Title: "Network throughput across collected containers", Status: status, EmptyTitle: "Container network unavailable", EmptyDetail: detail},
		}
	}
	return []chartPanel{
		{ID: "dashboard-cpu", Title: "CPU usage", Status: status, EmptyTitle: "CPU unavailable", EmptyDetail: detail},
		{ID: "dashboard-memory", Title: "Memory usage (Docker raw)", Status: status, EmptyTitle: "Memory unavailable", EmptyDetail: detail},
		{ID: "dashboard-network-rx", Title: "Network throughput", Status: status, EmptyTitle: "Network unavailable", EmptyDetail: detail},
	}
}

func buildDashboardChart(id, title, yLabel string, containers []metrics.ContainerChartSeries, start, end time.Time, values []dashboardAggregateMetric) (chartPanel, error) {
	input := dashboardChartData{
		Min: 0, Max: 1, XMin: start.UnixMilli(), XMax: end.UnixMilli(),
		SelectedTime: end.UnixMilli(), XLabel: "Time", YLabel: yLabel,
		Series: make([]dashboardSeries, 0, len(containers)*len(values)),
	}
	maxValue := 0.0
	hasValue := false
	for index, container := range containers {
		for _, metric := range values {
			series := dashboardSeries{
				Label:  "Container " + shortContainerID(container.ContainerID),
				Color:  chartColors[index%len(chartColors)],
				Points: make([]dashboardDataPoint, 0, len(container.Points)),
			}
			if len(containers) == 1 {
				series.Color = metric.Color
			}
			if len(values) > 1 {
				series.Label = metric.Label
			}
			for _, point := range container.Points {
				y := metric.Value(point)
				if y != nil {
					if math.IsNaN(*y) || math.IsInf(*y, 0) || *y < 0 {
						return chartPanel{}, fmt.Errorf("invalid %s observation for container %s", yLabel, container.ContainerID)
					}
					hasValue = true
					maxValue = math.Max(maxValue, *y)
				}
				series.Points = append(series.Points, dashboardDataPoint{X: point.Time.UnixMilli(), Y: y})
			}
			input.Series = append(input.Series, series)
		}
	}
	if hasValue {
		input.Max = math.Max(1, maxValue*1.1)
		if math.IsInf(input.Max, 0) {
			input.Max = maxValue
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return chartPanel{}, fmt.Errorf("encode %s dashboard chart: %w", title, err)
	}
	return chartPanel{
		ID: id, Title: title, Status: "Last hour", EmptyTitle: title + " unavailable",
		EmptyDetail: "No recorded values for this metric.", Series: string(encoded),
	}, nil
}

func buildAggregateDashboardChart(id, title, yLabel string, containers []metrics.ContainerChartSeries, start, end time.Time, bucket time.Duration, aggregateMetrics []dashboardAggregateMetric) (chartPanel, error) {
	if len(containers) == 0 || len(aggregateMetrics) == 0 || bucket <= 0 {
		return chartPanel{}, fmt.Errorf("invalid %s aggregate chart input", title)
	}
	bucketCount := len(containers[0].Points)
	for _, container := range containers[1:] {
		if len(container.Points) != bucketCount {
			return chartPanel{}, fmt.Errorf("inconsistent %s aggregate chart buckets", title)
		}
	}
	input := dashboardChartData{
		Min: 0, Max: 1, XMin: start.UnixMilli(), XMax: end.UnixMilli(),
		SelectedTime: end.UnixMilli(), XLabel: "Time", YLabel: yLabel,
		Series: make([]dashboardSeries, 0, len(aggregateMetrics)),
	}
	maxValue := 0.0
	hasValue := false
	for _, metric := range aggregateMetrics {
		if metric.Value == nil {
			return chartPanel{}, fmt.Errorf("invalid %s aggregate metric", title)
		}
		series := dashboardSeries{
			Label: metric.Label, Color: metric.Color,
			Points: make([]dashboardDataPoint, bucketCount),
		}
		for index := 0; index < bucketCount; index++ {
			sum, complete := 0.0, true
			for _, container := range containers {
				value := metric.Value(container.Points[index])
				if value == nil {
					complete = false
					continue
				}
				if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
					return chartPanel{}, fmt.Errorf("invalid %s observation for container %s", yLabel, container.ContainerID)
				}
				sum += *value
			}
			var total *float64
			if complete {
				if math.IsNaN(sum) || math.IsInf(sum, 0) {
					return chartPanel{}, fmt.Errorf("invalid %s aggregate", yLabel)
				}
				total = &sum
				hasValue = true
				maxValue = math.Max(maxValue, sum)
			}
			series.Points[index] = dashboardDataPoint{
				X: start.Add(time.Duration(index) * bucket).UnixMilli(),
				Y: total,
			}
		}
		input.Series = append(input.Series, series)
	}
	if hasValue {
		input.Max = math.Max(1, maxValue*1.1)
		if math.IsInf(input.Max, 0) {
			input.Max = maxValue
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return chartPanel{}, fmt.Errorf("encode %s dashboard chart: %w", title, err)
	}
	return chartPanel{
		ID: id, Title: title, Status: "Last hour", EmptyTitle: title + " unavailable",
		EmptyDetail: "No complete aggregate observations for this metric.", Series: string(encoded),
	}, nil
}

func shortContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
