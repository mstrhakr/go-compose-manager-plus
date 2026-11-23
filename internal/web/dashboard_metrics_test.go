package web

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
)

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
