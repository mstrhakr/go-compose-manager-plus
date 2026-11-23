package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
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
}

func (collector dashboardMetricsCollectorStub) Status() metrics.CollectorStatus {
	return collector.status
}

func (collector dashboardMetricsCollectorStub) SelectChartSeries(context.Context, time.Time, time.Time, time.Duration) ([]metrics.ContainerChartSeries, error) {
	return collector.series, nil
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
	if status != "Collection degraded" || len(panels) != 4 {
		t.Fatalf("dashboard status = %q, panels = %d", status, len(panels))
	}
	for _, panel := range panels {
		if panel.Status != "Collection degraded · 1 container with retained history" || panel.Series == "" {
			t.Fatalf("degraded chart panel = %#v", panel)
		}
		var data dashboardChartData
		if err := json.Unmarshal([]byte(panel.Series), &data); err != nil {
			t.Fatal(err)
		}
		if len(data.Series) != 1 || len(data.Series[0].Points) != 1 || data.Series[0].Points[0].Y == nil {
			t.Fatalf("retained chart data = %#v", data)
		}
	}
}
