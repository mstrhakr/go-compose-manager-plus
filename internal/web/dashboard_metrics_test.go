package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

type dashboardMetricsCollectorStub struct {
	status   metrics.CollectorStatus
	series   []metrics.ContainerChartSeries
	events   []metrics.Event
	chartIDs *[]string
}

func (collector dashboardMetricsCollectorStub) Status() metrics.CollectorStatus {
	return collector.status
}

func (collector dashboardMetricsCollectorStub) SelectChartSeries(_ context.Context, id string, _, _ time.Time, _ time.Duration) ([]metrics.ContainerChartSeries, error) {
	if collector.chartIDs != nil {
		*collector.chartIDs = append(*collector.chartIDs, id)
	}
	if id == "" {
		return collector.series, nil
	}
	for _, series := range collector.series {
		if series.ContainerID == id {
			return []metrics.ContainerChartSeries{series}, nil
		}
	}
	return nil, nil
}

func (collector dashboardMetricsCollectorStub) SelectTimelineEvents(_ context.Context, id string, _, _ time.Time, limit int) ([]metrics.Event, error) {
	var events []metrics.Event
	for _, event := range collector.events {
		if event.ContainerID == id {
			events = append(events, event)
		}
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events, nil
}

func (collector dashboardMetricsCollectorStub) SelectObservedContainers(context.Context, time.Time, time.Time) ([]string, error) {
	seen := map[string]bool{}
	for _, series := range collector.series {
		seen[series.ContainerID] = true
	}
	for _, event := range collector.events {
		seen[event.ContainerID] = true
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func TestSyntheticDashboardPreview(t *testing.T) {
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
	start, end := dashboardTimeRange()
	containers := make([]metrics.ContainerChartSeries, 4)
	for container := range containers {
		containers[container].ContainerID = strings.Repeat(string(rune('a'+container)), 64)
		for bucket := 0; bucket < 120; bucket++ {
			phase := float64(bucket)/7 + float64(container)
			cpu := 12 + 9*math.Sin(phase) + float64(container)*3
			receive := (1.2 + math.Sin(phase)*.8) * 1024 * 1024
			transmit := (.4 + math.Cos(phase)*.25) * 1024 * 1024
			memory := (180 + float64(container)*100 + 25*math.Sin(phase/3)) * 1024 * 1024
			if bucket >= 72 && bucket < 86 {
				cpu += 22
				receive *= 3
			}
			point := metrics.ChartPoint{
				Time:       start.Add(time.Duration(bucket)*chartBucket + 5*time.Second),
				CPUPercent: &cpu, MemoryUsageBytes: &memory,
				NetworkRxBytesPerSec: &receive, NetworkTxBytesPerSec: &transmit,
			}
			if container == 2 && bucket >= 40 && bucket <= 43 {
				point.CPUPercent = nil
				point.NetworkRxBytesPerSec = nil
			}
			containers[container].Points = append(containers[container].Points, point)
		}
	}
	server := &authServer{
		metricsSettings: settings,
		collector: dashboardMetricsCollectorStub{
			status: metrics.CollectorStatus{Enabled: true, Collecting: true, EventConnected: true}, series: containers,
			events: []metrics.Event{
				{ContainerID: containers[0].ContainerID, Time: start.Add(20 * time.Minute), Kind: metrics.EventStart},
				{ContainerID: containers[1].ContainerID, Time: start.Add(25 * time.Minute), Kind: metrics.EventOOM},
				{ContainerID: containers[0].ContainerID, Time: start.Add(42*time.Minute + 750*time.Millisecond), Kind: metrics.EventRestart},
				{ContainerID: containers[1].ContainerID, Time: start.Add(50 * time.Minute), Kind: metrics.EventStop},
			},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		pages:  template.Must(template.ParseFS(content, "templates/*.html", "templates/partials/*.html")),
	}
	panels, _ := server.dashboardChartsForRange(ctx, start, end, true, "")
	if len(panels) != 3 {
		t.Fatalf("preview panels = %d", len(panels))
	}
	var cpu dashboardChartData
	if err := json.Unmarshal([]byte(panels[0].Series), &cpu); err != nil {
		t.Fatal(err)
	}
	if cpu.XMax != end.UnixMilli() || end.Sub(start) != time.Hour ||
		len(cpu.Series) != 1 || len(cpu.Series[0].Points) != 120 ||
		cpu.Series[0].Points[43].Y != nil || cpu.Series[0].Points[119].Y == nil {
		t.Fatalf("preview must contain current last-hour history with a gap: %#v", cpu)
	}
	response := httptest.NewRecorder()
	server.render(response, pageData{
		Title: "Synthetic dashboard preview", View: "dashboard", Username: "Preview", Initial: "P",
		Charts: panels, MetricsStatus: "Synthetic preview",
		Metrics: []metricPanel{
			{ID: "restart-title", Title: "Container restart & OOM rate", Status: "Unavailable", Value: "--", Detail: "No runtime data"},
			{ID: "health-title", Title: "Stack health & readiness", Status: "Unavailable", Value: "--", Detail: "No runtime data"},
		},
		Notice:       "Synthetic preview data only. Generated for the current last hour; no Docker connection or production data.",
		RuntimeState: "Not connected", RuntimeMessage: "Synthetic preview; Docker is not connected.",
		SubviewNav: viewNavigation{Label: "Overview views", Tabs: []viewTab{
			{Label: "Dashboard", Href: "/", Current: true},
		}},
	})
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Synthetic preview data only") {
		t.Fatalf("preview rendering failed: %s", response.Body.String())
	}
	if directory := os.Getenv("CMP_UI_PREVIEW_DIR"); directory != "" {
		if err := os.WriteFile(filepath.Join(directory, "dashboard.html"), response.Body.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		settingsResponse := httptest.NewRecorder()
		server.render(settingsResponse, pageData{
			Title: "Synthetic settings preview", View: "settings", Username: "Preview", Initial: "P",
			MetricsSettings: metricsSettingsForm{
				Enabled: true, Profile: "high", Scope: "all", Interval: "1s",
				Limit: "10.48576", LimitUnit: "MB", Revision: "1",
			},
			MetricsPresets: metricsProfilePresets[:], CollectionStatusText: "Synthetic preview only; no Docker collection.",
			Notice: "Synthetic preview data only. No production preferences are changed.",
		})
		if settingsResponse.Code != http.StatusOK {
			t.Fatalf("settings preview rendering failed: %s", settingsResponse.Body.String())
		}
		if err := os.WriteFile(filepath.Join(directory, "settings.html"), settingsResponse.Body.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := []runtime.Container{
		{ID: containers[0].ContainerID, Name: "api", State: "running", Project: "demo", Service: "api"},
		{ID: containers[1].ContainerID, Name: "database", State: "exited", Project: "demo", Service: "database"},
		{ID: containers[2].ContainerID, Name: "proxy", State: "running", Project: "edge", Service: "proxy"},
		{ID: containers[3].ContainerID, Name: "worker", State: "running", Project: "demo", Service: "worker"},
	}
	queries := []url.Values{
		{}, {"container": {containers[1].ContainerID}},
		{"container": {containers[2].ContainerID}}, {"container": {containers[3].ContainerID}},
		{"q": {"database"}}, {"q": {"missing"}},
	}
	for index, query := range queries {
		choices, selected, filters, err := server.observabilityContainers(ctx, start, end, snapshot, query)
		wantSelected := ""
		if index < 4 {
			wantSelected = containers[index].ContainerID
		} else if index == 4 {
			wantSelected = containers[1].ContainerID
		}
		if err != nil || selected != wantSelected {
			t.Fatalf("single-container selection = %q, choices %v, error %v", selected, choices, err)
		}
		charts, status := server.dashboardChartsForRange(ctx, start, end, false, selected)
		for _, panel := range charts {
			if selected == "" {
				if panel.Series != "" {
					t.Fatal("empty filtered view displays metrics")
				}
				continue
			}
			var chart dashboardChartData
			if err := json.Unmarshal([]byte(panel.Series), &chart); err != nil {
				t.Fatal(err)
			}
			if panel.ID == "dashboard-network-rx" {
				if len(chart.Series) != 2 || chart.Series[0].Label != "Receive" || chart.Series[1].Label != "Transmit" {
					t.Fatalf("network chart has unexpected streams: %#v", chart.Series)
				}
				var source metrics.ChartPoint
				for _, container := range containers {
					if container.ContainerID == selected {
						source = container.Points[0]
					}
				}
				if chart.Series[0].Points[0].Y == nil || *chart.Series[0].Points[0].Y != *source.NetworkRxBytesPerSec/(1024*1024) ||
					chart.Series[0].Points[0].X != source.Time.UnixMilli() {
					t.Fatalf("network throughput is not isolated to %s", selected)
				}
				continue
			}
			if len(chart.Series) != 1 || chart.Series[0].Label != "Container "+shortContainerID(selected) {
				t.Fatalf("selected chart leaks another container: %#v", chart.Series)
			}
		}
		timeline := server.dashboardTimeline(ctx, start, end, selected)
		for _, event := range timeline.Events {
			if event.Container != "Container "+shortContainerID(selected) {
				t.Fatalf("timeline leaks another container: %#v", event)
			}
		}
		response := httptest.NewRecorder()
		server.render(response, pageData{
			Title: "Synthetic observability preview", View: "observability", Username: "Preview", Initial: "P",
			ObservedContainers: choices, SelectedContainer: selected, Filters: filters, Charts: charts, MetricsStatus: status, Timeline: timeline,
			RuntimeMessage: "Synthetic read-only snapshot; no Docker connection.",
			Notice:         "Synthetic preview data only. No Docker connection or production data.",
			SubviewNav: viewNavigation{Label: "Container views", Tabs: []viewTab{
				{Label: "Containers", Href: "/containers"},
				{Label: "Observability", Href: "/containers/observability", Current: true},
			}},
		})
		if response.Code != 200 {
			t.Fatalf("observability preview rendering failed: %s", response.Body.String())
		}
		if directory := os.Getenv("CMP_UI_PREVIEW_DIR"); directory != "" {
			if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("observability-%d.html", index)), response.Body.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, requested := range []string{"all", strings.Repeat("f", 64)} {
		_, selected, _, err := server.observabilityContainers(ctx, start, end, snapshot, url.Values{"container": {requested}})
		if err != nil || selected != "" {
			t.Fatalf("unknown selection %q fell back to %q: %v", requested, selected, err)
		}
	}
	choices, selected, filters, err := server.observabilityContainers(ctx, start, end, snapshot, url.Values{"state": {"running"}, "project": {"demo"}})
	if err != nil || len(choices) != 2 || selected != containers[0].ContainerID || filters.Total != 4 || filters.Shown != 2 {
		t.Fatalf("container filters = %#v, selected %q, filters %#v, error %v", choices, selected, filters, err)
	}
	if !strings.Contains(choices[1].Href, "project=demo") || !strings.Contains(choices[1].Href, "state=running") {
		t.Fatalf("selection link lost filters: %s", choices[1].Href)
	}
	noSelection, _ := server.dashboardChartsForRange(ctx, start, end, false, "")
	for _, panel := range noSelection {
		if panel.Series != "" {
			t.Fatal("missing selection produced overlapping chart data")
		}
	}
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
		[]dashboardAggregateMetric{{Label: "CPU", Color: "blue", Value: func(point metrics.ChartPoint) *float64 { return point.CPUPercent }}})
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
	var chartIDs []string
	collector := dashboardMetricsCollectorStub{
		chartIDs: &chartIDs,
		status:   metrics.CollectorStatus{Enabled: true, Problem: "Docker stats failed"},
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
	observabilityPanels, _ := server.dashboardChartsForRange(ctx, start, end, false, collector.series[0].ContainerID)
	if len(observabilityPanels) != 3 {
		t.Fatalf("observability chart count = %d, want three single-container charts", len(observabilityPanels))
	}
	var observabilityCPU dashboardChartData
	if err := json.Unmarshal([]byte(observabilityPanels[0].Series), &observabilityCPU); err != nil {
		t.Fatal(err)
	}
	if len(observabilityCPU.Series) != 1 || observabilityCPU.Series[0].Label != "Container aaaaaaaaaaaa" {
		t.Fatalf("observability CPU chart was aggregated: %#v", observabilityCPU.Series)
	}
	if !reflect.DeepEqual(chartIDs, []string{"", collector.series[0].ContainerID}) {
		t.Fatalf("Overview and Observability query scopes = %v", chartIDs)
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
	got := server.dashboardTimeline(ctx, start, end, containerID)
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
