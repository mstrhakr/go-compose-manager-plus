package web

import (
	"context"
	"encoding/json"
	"html/template"
	"io"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
)

type dashboardMetricsCollectorStub struct {
	status metrics.CollectorStatus
	series []metrics.ContainerChartSeries
	events []metrics.Event
}

func (collector dashboardMetricsCollectorStub) Status() metrics.CollectorStatus {
	return collector.status
}

func (collector dashboardMetricsCollectorStub) SelectChartSeries(context.Context, time.Time, time.Time, time.Duration) ([]metrics.ContainerChartSeries, error) {
	return collector.series, nil
}

func (collector dashboardMetricsCollectorStub) SelectTimelineEvents(context.Context, time.Time, time.Time, int) ([]metrics.Event, error) {
	return collector.events, nil
}

func TestBuildDashboardChartSerializesRecordedContainerValuesAndGaps(t *testing.T) {
	start := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	cpu1, cpu2 := 25.0, 75.0
	containers := []metrics.ContainerChartSeries{
		{ContainerID: "aaaaaaaaaaaa1111", Points: []metrics.ChartPoint{
			{Time: start.Add(time.Minute), CPUPercent: &cpu1},
			{Time: start.Add(2 * time.Minute)},
		}},
		{ContainerID: "bbbbbbbbbbbb2222", Points: []metrics.ChartPoint{
			{Time: start.Add(time.Minute), CPUPercent: &cpu2},
			{Time: start.Add(2 * time.Minute)},
		}},
	}
	panel, err := buildDashboardChart("cpu", "CPU by container", "CPU (%)", containers, start, end,
		func(point metrics.ChartPoint) *float64 { return point.CPUPercent })
	if err != nil {
		t.Fatal(err)
	}
	var got dashboardChartData
	if err := json.Unmarshal([]byte(panel.Series), &got); err != nil {
		t.Fatal(err)
	}
	if got.Min != 0 || got.Max != 82.5 || got.XMin != start.UnixMilli() || got.XMax != end.UnixMilli() ||
		got.SelectedTime != end.UnixMilli() || len(got.Series) != 2 {
		t.Fatalf("chart data = %#v", got)
	}
	if got.Series[0].Label != "Container aaaaaaaaaaaa" || got.Series[0].Color != "blue" ||
		got.Series[0].Points[0].Y == nil || *got.Series[0].Points[0].Y != cpu1 ||
		got.Series[0].Points[1].Y != nil || got.Series[1].Points[0].Y == nil ||
		*got.Series[1].Points[0].Y != cpu2 || got.Series[1].Color != "amber" {
		t.Fatalf("container samples were combined or gaps changed: %#v", got.Series)
	}
	if !reflect.DeepEqual(got.Series[0].Points[1], dashboardDataPoint{X: start.Add(2 * time.Minute).UnixMilli()}) {
		t.Fatalf("missing sample representation = %#v", got.Series[0].Points[1])
	}
}

func TestDashboardChartsKeepRetainedHistoryWhenCollectionIsDegraded(t *testing.T) {
	ctx := context.Background()
	database, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.SQL.Close(); err != nil {
			t.Error(err)
		}
	})
	settings := metrics.NewSettingsStore(database.SQL)
	if _, err := settings.Save(ctx, metrics.Settings{
		Enabled: true, Profile: metrics.ProfileHigh, Scope: metrics.ScopeAll,
		Interval: time.Second, StorageLimit: 10 << 20,
	}); err != nil {
		t.Fatal(err)
	}

	cpu, memory, receive, transmit := 25.0, 1024.0, 2.0, 3.0
	collector := dashboardMetricsCollectorStub{
		status: metrics.CollectorStatus{Enabled: true, Problem: "Docker stats failed"},
		series: []metrics.ContainerChartSeries{{
			ContainerID: strings.Repeat("a", 64),
			Points: []metrics.ChartPoint{{
				Time: time.Now().UTC(), CPUPercent: &cpu, MemoryUsageBytes: &memory,
				NetworkRxBytesPerSec: &receive, NetworkTxBytesPerSec: &transmit,
			}},
		}},
	}
	server := &authServer{
		metricsSettings: settings, collector: collector,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	panels, status := server.dashboardCharts(ctx)
	if status != "Collection degraded" || len(panels) != 3 {
		t.Fatalf("dashboard status = %q, panels = %d", status, len(panels))
	}
	for _, panel := range panels {
		if panel.Status != "Collection degraded · aggregate of 1 container with retained history" || panel.Series == "" {
			t.Fatalf("degraded chart panel = %#v", panel)
		}
		var data dashboardChartData
		if err := json.Unmarshal([]byte(panel.Series), &data); err != nil {
			t.Fatal(err)
		}
		wantSeriesCount := 1
		if panel.ID == "dashboard-network-rx" {
			wantSeriesCount = 2
		}
		if len(data.Series) != wantSeriesCount || len(data.Series[0].Points) != 1 || data.Series[0].Points[0].Y == nil {
			t.Fatalf("retained chart data = %#v", data)
		}
	}
	start, end := dashboardTimeRange()
	observabilityPanels, _ := server.dashboardChartsForRange(ctx, start, end, false)
	if len(observabilityPanels) != 4 {
		t.Fatalf("observability chart count = %d, want four per-container charts", len(observabilityPanels))
	}
	var observabilityCPU dashboardChartData
	if err := json.Unmarshal([]byte(observabilityPanels[0].Series), &observabilityCPU); err != nil {
		t.Fatal(err)
	}
	if len(observabilityCPU.Series) != 1 || observabilityCPU.Series[0].Label != "Container aaaaaaaaaaaa" {
		t.Fatalf("observability CPU chart was aggregated: %#v", observabilityCPU.Series)
	}
}

func TestBuildAggregateDashboardChartSumsCompleteBucketsAndPreservesGaps(t *testing.T) {
	start := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	cpu1, cpu2 := 25.0, 15.0
	containers := []metrics.ContainerChartSeries{
		{ContainerID: strings.Repeat("a", 64), Points: []metrics.ChartPoint{
			{CPUPercent: &cpu1}, {CPUPercent: &cpu1},
		}},
		{ContainerID: strings.Repeat("b", 64), Points: []metrics.ChartPoint{
			{CPUPercent: &cpu2}, {},
		}},
	}
	panel, err := buildAggregateDashboardChart(
		"dashboard-cpu", "CPU across collected containers", "CPU (%)", containers, start, end, chartBucket,
		[]dashboardAggregateMetric{{Label: "Collected container total", Color: "blue", Value: func(point metrics.ChartPoint) *float64 {
			return point.CPUPercent
		}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	var got dashboardChartData
	if err := json.Unmarshal([]byte(panel.Series), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Series) != 1 || len(got.Series[0].Points) != 2 {
		t.Fatalf("aggregate chart shape = %#v", got)
	}
	if got.Series[0].Points[0].Y == nil || *got.Series[0].Points[0].Y != 40 {
		t.Fatalf("aggregate total = %#v", got.Series[0].Points[0])
	}
	if got.Series[0].Points[1].Y != nil {
		t.Fatalf("incomplete aggregate bucket should remain a gap: %#v", got.Series[0].Points[1])
	}
	if got.Series[0].Label != "Collected container total" || got.Series[0].Color != "blue" {
		t.Fatalf("aggregate series presentation = %#v", got.Series[0])
	}
}

func TestDashboardTimelineBoundsAndLabelsRecentObservedEvents(t *testing.T) {
	ctx := context.Background()
	database, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.SQL.Close(); err != nil {
			t.Error(err)
		}
	})
	settings := metrics.NewSettingsStore(database.SQL)
	if _, err := settings.Save(ctx, metrics.Settings{
		Enabled: true, Profile: metrics.ProfileLow, Scope: metrics.ScopeActivated,
		Interval: 15 * time.Second, StorageLimit: 10 << 20,
	}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	containerID := strings.Repeat("a", 64)
	events := make([]metrics.Event, timelineEventLimit+1)
	for i := range events {
		events[i] = metrics.Event{
			ContainerID: containerID, Time: start.Add(time.Duration(i) * time.Second),
			Kind: metrics.EventHealth, Health: metrics.HealthUnhealthy,
		}
	}

	collector := dashboardMetricsCollectorStub{
		status: metrics.CollectorStatus{Enabled: true, Collecting: true, EventConnected: true},
		events: events,
	}
	server := &authServer{
		metricsSettings: settings, collector: collector,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	got := server.dashboardTimeline(ctx, start, end)
	if !got.Truncated || len(got.Events) != timelineEventLimit || len(got.RecentEvents) != timelineEventLimit {
		t.Fatalf("timeline event bounds = truncated %v, events %d, recent %d", got.Truncated, len(got.Events), len(got.RecentEvents))
	}
	if got.Events[0].Time != start.Add(time.Second).UnixMilli() ||
		got.Events[len(got.Events)-1].Time != start.Add(time.Duration(timelineEventLimit)*time.Second).UnixMilli() ||
		got.RecentEvents[0].Time != got.Events[len(got.Events)-1].Time {
		t.Fatalf("timeline order or selection = %#v", got)
	}
	if got.Events[0].Description != "Health: unhealthy" || got.Events[0].Container != "Container aaaaaaaaaaaa" ||
		got.Events[0].Position <= 0 || got.Events[0].Position >= 100 {
		t.Fatalf("timeline event presentation = %#v", got.Events[0])
	}
	if !strings.Contains(got.Message, "newest 200") {
		t.Fatalf("truncation message = %q", got.Message)
	}
}

func TestObservabilityTemplateRendersTimelineMarkersAndEventList(t *testing.T) {
	pages := template.Must(template.ParseFS(content, "templates/*.html", "templates/partials/*.html"))
	server := &authServer{pages: pages}
	event := dashboardTimelineEvent{
		Time: 2000, TimeLabel: "Nov 23 · 12:30:00 UTC", Position: 50,
		Container: "Container aaaaaaaaaaaa", Description: "Out of memory", Kind: string(metrics.EventOOM),
	}
	data := pageData{
		Title: "Container observability", View: "observability", MetricsStatus: "Collecting",
		SubviewNav: viewNavigation{Label: "Container views", Tabs: []viewTab{
			{Label: "Containers", Href: "/containers"},
			{Label: "Observability", Href: "/containers/observability", Current: true},
		}},
		Timeline: dashboardTimeline{
			Start: 1000, End: 3000, SelectedTime: 3000, StartLabel: "Start", EndLabel: "End",
			Status: "1 observed event", Message: "Observed lifecycle events only.",
			Events: []dashboardTimelineEvent{event}, RecentEvents: []dashboardTimelineEvent{event},
		},
		Charts: emptyDashboardCharts("Collecting", "No recorded values.", false),
	}
	response := httptest.NewRecorder()
	server.render(response, data)
	body := response.Body.String()
	for _, expected := range []string{
		`data-timeline-select="2000"`, `data-position="50.000"`,
		`data-kind="oom"`, "Out of memory", "Container aaaaaaaaaaaa",
		`aria-current="page"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("observability page does not contain %q", expected)
		}
	}
}
