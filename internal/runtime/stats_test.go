package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func decodeStats(t *testing.T, data string) Stats {
	t.Helper()
	var stats Stats
	if err := json.Unmarshal([]byte(data), &stats); err != nil {
		t.Fatal(err)
	}
	return stats
}

func TestObserveStatsRatesAndMissingValues(t *testing.T) {
	first := decodeStats(t, `{
		"read":"2026-11-22T20:00:00Z",
		"cpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":400},
		"memory_stats":{"usage":0,"limit":1024},
		"networks":{"eth0":{"rx_bytes":100,"tx_bytes":200},"eth1":{"rx_bytes":50,"tx_bytes":50}},
		"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"Read","value":100},{"major":8,"minor":0,"op":"Write","value":200}]}
	}`)
	second := decodeStats(t, `{
		"read":"2026-11-22T20:00:02Z",
		"cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":500,"online_cpus":2},
		"memory_stats":{"usage":100,"limit":1024},
		"networks":{"eth0":{"rx_bytes":120,"tx_bytes":210},"eth1":{"rx_bytes":60,"tx_bytes":90}},
		"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"read","value":140},{"major":8,"minor":0,"op":"WRITE","value":210}]}
	}`)
	initial, err := ObserveStats(first, nil)
	if err != nil || initial.CPUPercent != nil || initial.NetworkRxBytesPerSec != nil || initial.BlockReadBytesPerSec != nil || initial.MemoryUsageBytes == nil || *initial.MemoryUsageBytes != 0 {
		t.Fatalf("initial observation = %#v, error = %v", initial, err)
	}
	got, err := ObserveStats(second, &first)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]struct {
		got  *float64
		want float64
	}{
		"CPU": {got.CPUPercent, 400}, "network receive": {got.NetworkRxBytesPerSec, 15},
		"network transmit": {got.NetworkTxBytesPerSec, 25},
		"block read":       {got.BlockReadBytesPerSec, 20}, "block write": {got.BlockWriteBytesPerSec, 5},
	} {
		if value.got == nil || *value.got != value.want {
			t.Errorf("%s = %v, want %v", name, value.got, value.want)
		}
	}
	if got.MemoryUsageBytes == nil || *got.MemoryUsageBytes != 100 || got.MemoryLimitBytes == nil || *got.MemoryLimitBytes != 1024 {
		t.Fatalf("memory = %#v", got)
	}
}

func TestObserveStatsCounterResetsAndIdentityChanges(t *testing.T) {
	base := decodeStats(t, `{"read":"2026-11-22T20:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":20},"system_cpu_usage":100,"online_cpus":1},"networks":{"eth0":{"rx_bytes":100,"tx_bytes":100}},"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"Read","value":100},{"major":8,"minor":0,"op":"Write","value":100}]}}`)
	tests := []struct {
		name string
		next string
	}{
		{"reset", `{"networks":{"eth0":{"rx_bytes":1,"tx_bytes":1}},"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"Read","value":1},{"major":8,"minor":0,"op":"Write","value":1}]}}`},
		{"changed identity", `{"networks":{"eth1":{"rx_bytes":200,"tx_bytes":200}},"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":1,"op":"Read","value":200},{"major":8,"minor":1,"op":"Write","value":200}]}}`},
		{"missing values", `{"networks":{"eth0":{}},"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"Read"},{"major":8,"minor":0,"op":"Write"}]}}`},
		{"duplicate device", `{"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"Read","value":200},{"major":8,"minor":0,"op":"Read","value":200}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := decodeStats(t, tt.next)
			next.Read = base.Read.Add(time.Second)
			got, err := ObserveStats(next, &base)
			if err != nil || got.CPUPercent != nil || got.NetworkRxBytesPerSec != nil || got.NetworkTxBytesPerSec != nil || got.BlockReadBytesPerSec != nil || got.BlockWriteBytesPerSec != nil {
				t.Fatalf("observation = %#v, error = %v", got, err)
			}
		})
	}
	if _, err := ObserveStats(base, &base); err == nil {
		t.Fatal("equal timestamps accepted")
	}
	if _, err := ObserveStats(Stats{}, nil); err == nil {
		t.Fatal("missing timestamp accepted")
	}
}

func TestObserveStatsCPUFallbackAndPartialReset(t *testing.T) {
	base := decodeStats(t, `{"read":"2026-11-22T20:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":10},"system_cpu_usage":100},"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}}}`)
	next := decodeStats(t, `{"read":"2026-11-22T20:00:01Z","cpu_stats":{"cpu_usage":{"total_usage":20,"percpu_usage":[1,2]},"system_cpu_usage":200},"networks":{"eth0":{"rx_bytes":0,"tx_bytes":25}}}`)
	got, err := ObserveStats(next, &base)
	if err != nil || got.CPUPercent == nil || *got.CPUPercent != 20 || got.NetworkRxBytesPerSec != nil || got.NetworkTxBytesPerSec == nil || *got.NetworkTxBytesPerSec != 5 {
		t.Fatalf("observation = %#v, error = %v", got, err)
	}
}

func TestObserveStatsRejectsInvalidDeltas(t *testing.T) {
	first := decodeStats(t, `{"read":"2026-11-22T20:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":10},"system_cpu_usage":100,"online_cpus":1},"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}},"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"Read","value":10},{"major":8,"minor":1,"op":"Read","value":100}]}}`)
	second := decodeStats(t, `{"read":"2026-11-22T20:00:01Z","cpu_stats":{"cpu_usage":{"total_usage":9},"system_cpu_usage":200,"online_cpus":1},"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}},"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"Read","value":0},{"major":8,"minor":1,"op":"Read","value":200}]}}`)
	got, err := ObserveStats(second, &first)
	if err != nil || got.CPUPercent != nil || got.BlockReadBytesPerSec != nil {
		t.Fatalf("reset must be missing, got %#v, error %v", got, err)
	}
	if got.NetworkRxBytesPerSec == nil || *got.NetworkRxBytesPerSec != 0 {
		t.Fatalf("unchanged counter must be observed zero: %#v", got.NetworkRxBytesPerSec)
	}
	second.Read = first.Read.Add(-time.Second)
	if _, err := ObserveStats(second, &first); err == nil {
		t.Fatal("out-of-order sample accepted")
	}
}

func TestContainerStatsRequest(t *testing.T) {
	id := strings.Repeat("a", 64)
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.40/containers/"+id+"/stats" || request.URL.Query().Get("stream") != "false" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"read":"2026-11-22T20:00:00Z","memory_stats":{"usage":0}}`))}, nil
	})}}
	got, err := client.ContainerStats(context.Background(), id)
	if err != nil || got.Memory.Usage == nil || *got.Memory.Usage != 0 {
		t.Fatalf("stats = %#v, error = %v", got, err)
	}
	for _, invalid := range []string{"../containers/json", strings.Repeat("z", 64), ""} {
		if _, err := client.ContainerStats(context.Background(), invalid); err == nil {
			t.Fatalf("accepted invalid ID %q", invalid)
		}
	}
}

func TestContainerStatsToObservations(t *testing.T) {
	bodies := []string{
		`{"read":"2026-11-22T20:00:00Z","networks":{"eth0":{"rx_bytes":100,"tx_bytes":0}}}`,
		`{"read":"2026-11-22T20:00:02Z","networks":{"eth0":{"rx_bytes":160,"tx_bytes":10}}}`,
	}
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		if len(bodies) == 0 {
			t.Fatal("unexpected extra Docker request")
		}
		body := bodies[0]
		bodies = bodies[1:]
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	id := strings.Repeat("a", 64)
	first, err := client.ContainerStats(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.ContainerStats(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ObserveStats(second, &first)
	if err != nil || got.NetworkRxBytesPerSec == nil || *got.NetworkRxBytesPerSec != 30 || got.NetworkTxBytesPerSec == nil || *got.NetworkTxBytesPerSec != 5 {
		t.Fatalf("observation = %#v, error = %v", got, err)
	}
}

func TestContainerStatsFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{"connection", 0, "", errors.New("offline")},
		{"forbidden", http.StatusForbidden, "", nil},
		{"invalid JSON", http.StatusOK, "{", nil},
		{"missing timestamp", http.StatusOK, "{}", nil},
		{"too large", http.StatusOK, strings.Repeat(" ", maxResponseBytes+1), nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				if tt.err != nil {
					return nil, tt.err
				}
				return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})}}
			if _, err := client.ContainerStats(context.Background(), strings.Repeat("a", 64)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
