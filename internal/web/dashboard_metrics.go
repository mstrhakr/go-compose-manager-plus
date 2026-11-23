package web

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
)

const chartBucket = 30 * time.Second

var chartColors = []string{"blue", "amber", "purple", "green", "red"}

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

func (server *authServer) dashboardCharts(ctx context.Context) ([]chartPanel, string) {
	end := time.Now().UTC().Truncate(time.Second)
	start := end.Add(-time.Hour)
	panels := emptyDashboardCharts("Collection disabled", "No metrics are being collected. Enable opt-in collection in Settings to populate these charts.")
	settings, err := server.metricsSettings.Load(ctx)
	if err != nil {
		server.logger.ErrorContext(ctx, "load dashboard metrics settings", "error", err)
		return emptyDashboardCharts("Unavailable", "Metrics settings could not be loaded."), "Unavailable"
	}
	if !settings.Enabled {
		return panels, "Disabled"
	}
	status := server.collector.Status()
	if status.Problem != "" {
		return emptyDashboardCharts("Collection paused", "Metrics collection is reporting a problem. Check Settings for collection status."), "Collection paused"
	}
	if !status.Collecting {
		return emptyDashboardCharts("Collector starting", "The metrics collector is starting; no dashboard history is available yet."), "Starting"
	}
	series, err := server.collector.SelectChartSeries(ctx, start, end, chartBucket)
	if err != nil {
		server.logger.ErrorContext(ctx, "read dashboard metrics", "error", err)
		return emptyDashboardCharts("Unavailable", "Stored metrics could not be loaded."), "Unavailable"
	}
	if len(series) == 0 {
		return emptyDashboardCharts("Waiting for observations", "No observations were recorded during the last hour."), "Collecting"
	}
	countStatus := fmt.Sprintf("Last hour · %d containers", len(series))
	builders := []func() (chartPanel, error){
		func() (chartPanel, error) {
			return buildDashboardChart("dashboard-cpu", "CPU by container", "CPU (%)", series, start, end, func(point metrics.ChartPoint) *float64 { return point.CPUPercent })
		},
		func() (chartPanel, error) {
			return buildDashboardChart("dashboard-memory", "Docker raw memory usage by container", "MiB", series, start, end, func(point metrics.ChartPoint) *float64 {
				if point.MemoryUsageBytes == nil {
					return nil
				}
				value := *point.MemoryUsageBytes / (1024 * 1024)
				return &value
			})
		},
		func() (chartPanel, error) {
			return buildDashboardChart("dashboard-network-rx", "Network receive by container", "MiB/s", series, start, end, func(point metrics.ChartPoint) *float64 {
				if point.NetworkRxBytesPerSec == nil {
					return nil
				}
				value := *point.NetworkRxBytesPerSec / (1024 * 1024)
				return &value
			})
		},
		func() (chartPanel, error) {
			return buildDashboardChart("dashboard-network-tx", "Network transmit by container", "MiB/s", series, start, end, func(point metrics.ChartPoint) *float64 {
				if point.NetworkTxBytesPerSec == nil {
					return nil
				}
				value := *point.NetworkTxBytesPerSec / (1024 * 1024)
				return &value
			})
		},
	}
	panelIDs := []string{"dashboard-cpu", "dashboard-memory", "dashboard-network-rx", "dashboard-network-tx"}
	panelTitles := []string{"CPU by container", "Docker raw memory usage by container", "Network receive by container", "Network transmit by container"}
	panels = make([]chartPanel, 0, len(builders))
	for i, build := range builders {
		panel, err := build()
		if err != nil {
			server.logger.ErrorContext(ctx, "encode dashboard chart", "error", err)
			panel = chartPanel{ID: panelIDs[i], Title: panelTitles[i], Status: "Unavailable", EmptyTitle: "Chart unavailable", EmptyDetail: "Stored chart data could not be displayed."}
		}
		panels = append(panels, panel)
	}
	for i := range panels {
		if panels[i].Status != "Unavailable" {
			panels[i].Status = countStatus
		}
	}
	return panels, "Last hour"
}

func emptyDashboardCharts(status, detail string) []chartPanel {
	return []chartPanel{
		{ID: "dashboard-cpu", Title: "CPU by container", Status: status, EmptyTitle: "CPU unavailable", EmptyDetail: detail},
		{ID: "dashboard-memory", Title: "Docker raw memory usage by container", Status: status, EmptyTitle: "Memory unavailable", EmptyDetail: detail},
		{ID: "dashboard-network-rx", Title: "Network receive by container", Status: status, EmptyTitle: "Network receive unavailable", EmptyDetail: detail},
		{ID: "dashboard-network-tx", Title: "Network transmit by container", Status: status, EmptyTitle: "Network transmit unavailable", EmptyDetail: detail},
	}
}

func buildDashboardChart(id, title, yLabel string, containers []metrics.ContainerChartSeries, start, end time.Time, value func(metrics.ChartPoint) *float64) (chartPanel, error) {
	input := dashboardChartData{
		Min: 0, Max: 1, XMin: start.UnixMilli(), XMax: end.UnixMilli(),
		SelectedTime: end.UnixMilli(), XLabel: "Time", YLabel: yLabel,
		Series: make([]dashboardSeries, 0, len(containers)),
	}
	maxValue := 0.0
	hasValue := false
	for index, container := range containers {
		series := dashboardSeries{
			Label:  "Container " + shortContainerID(container.ContainerID),
			Color:  chartColors[index%len(chartColors)],
			Points: make([]dashboardDataPoint, 0, len(container.Points)),
		}
		for _, point := range container.Points {
			y := value(point)
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

func shortContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
