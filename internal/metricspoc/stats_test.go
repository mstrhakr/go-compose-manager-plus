package metricspoc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nakabonne/tstorage"
)

type counters struct {
	Total uint64 `json:"total_usage"`
}

type cpuStats struct {
	Usage  counters `json:"cpu_usage"`
	System uint64   `json:"system_cpu_usage"`
	Online uint64   `json:"online_cpus"`
}

type dockerStats struct {
	Read   time.Time `json:"read"`
	CPU    cpuStats  `json:"cpu_stats"`
	Prev   cpuStats  `json:"precpu_stats"`
	Memory struct {
		Usage *uint64 `json:"usage"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		Rx uint64 `json:"rx_bytes"`
		Tx uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlockIO struct {
		Entries []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
}

type point struct {
	metric string
	value  float64
}

func samplePoints(s dockerStats) ([]point, error) {
	if s.Read.IsZero() {
		return nil, errors.New("stats response has no timestamp")
	}
	var result []point
	if s.Prev.System > 0 && s.CPU.Usage.Total >= s.Prev.Usage.Total && s.CPU.System > s.Prev.System && s.CPU.Online > 0 {
		cpu := float64(s.CPU.Usage.Total-s.Prev.Usage.Total) / float64(s.CPU.System-s.Prev.System) * float64(s.CPU.Online) * 100
		result = append(result, point{"cpu_percent", cpu})
	}
	if s.Memory.Usage != nil {
		result = append(result, point{"memory_usage_bytes", float64(*s.Memory.Usage)})
	}
	if s.Networks != nil {
		var rx, tx uint64
		for _, network := range s.Networks {
			rx += network.Rx
			tx += network.Tx
		}
		result = append(result, point{"network_rx_bytes", float64(rx)}, point{"network_tx_bytes", float64(tx)})
	}
	if s.BlockIO.Entries != nil {
		var read, write uint64
		var hasRead, hasWrite bool
		for _, entry := range s.BlockIO.Entries {
			switch strings.ToLower(entry.Op) {
			case "read":
				read += entry.Value
				hasRead = true
			case "write":
				write += entry.Value
				hasWrite = true
			}
		}
		if hasRead {
			result = append(result, point{"block_read_bytes", float64(read)})
		}
		if hasWrite {
			result = append(result, point{"block_write_bytes", float64(write)})
		}
	}
	return result, nil
}

func record(storage tstorage.Storage, s dockerStats) (int, error) {
	points, err := samplePoints(s)
	if err != nil {
		return 0, err
	}
	rows := make([]tstorage.Row, 0, len(points))
	for _, p := range points {
		rows = append(rows, tstorage.Row{Metric: p.metric, DataPoint: tstorage.DataPoint{Timestamp: s.Read.UnixNano(), Value: p.value}})
	}
	if len(rows) == 0 {
		return 0, errors.New("stats response has no supported observations")
	}
	if err := storage.InsertRows(rows); err != nil {
		return 0, fmt.Errorf("store samples: %w", err)
	}
	return len(rows), nil
}

func openStorage(dir string) (tstorage.Storage, error) {
	return tstorage.NewStorage(tstorage.WithDataPath(dir), tstorage.WithWALBufferedSize(0))
}

func TestStatsPointsAndStorageRestart(t *testing.T) {
	const payload = `{"read":"2026-11-22T21:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":500,"online_cpus":2},"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":400},"memory_stats":{"usage":1024},"networks":{"eth0":{"rx_bytes":12,"tx_bytes":34}},"blkio_stats":{"io_service_bytes_recursive":[{"op":"Read","value":56},{"op":"Write","value":78}]}}`
	var stats dockerStats
	if err := json.Unmarshal([]byte(payload), &stats); err != nil {
		t.Fatal(err)
	}
	points, err := samplePoints(stats)
	if err != nil {
		t.Fatal(err)
	}
	want := []point{{"cpu_percent", 400}, {"memory_usage_bytes", 1024}, {"network_rx_bytes", 12}, {"network_tx_bytes", 34}, {"block_read_bytes", 56}, {"block_write_bytes", 78}}
	if fmt.Sprint(points) != fmt.Sprint(want) {
		t.Fatalf("points = %v, want %v", points, want)
	}
	dir := t.TempDir()
	store, err := openStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := record(store, stats); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = openStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, p := range want {
		saved, err := store.Select(p.metric, nil, stats.Read.UnixNano()-1, stats.Read.UnixNano()+1)
		if err != nil || len(saved) != 1 || saved[0].Value != p.value {
			t.Fatalf("%s after restart: %v, %v", p.metric, saved, err)
		}
	}
}

func TestMissingStatsAreNotZero(t *testing.T) {
	stats := dockerStats{Read: time.Now()}
	points, err := samplePoints(stats)
	if err != nil || len(points) != 0 {
		t.Fatalf("missing observations: %v, %v", points, err)
	}
	if _, err := samplePoints(dockerStats{}); err == nil {
		t.Fatal("missing timestamp accepted")
	}
	stats.CPU = cpuStats{Usage: counters{Total: 10}, System: 20, Online: 1}
	points, err = samplePoints(stats)
	if err != nil || len(points) != 0 {
		t.Fatalf("CPU without previous system sample: %v, %v", points, err)
	}
	if err := json.Unmarshal([]byte(`{"read":"2026-11-22T21:00:00Z","blkio_stats":{"io_service_bytes_recursive":[{"op":"Total","value":12}]}}`), &stats); err != nil {
		t.Fatal(err)
	}
	points, err = samplePoints(stats)
	if err != nil || len(points) != 0 {
		t.Fatalf("block I/O without read/write counters: %v, %v", points, err)
	}
}

func TestLiveStatsStorage(t *testing.T) {
	socket, container := os.Getenv("CMP_METRICS_SOCKET"), os.Getenv("CMP_METRICS_CONTAINER")
	if socket == "" && container == "" {
		t.Skip("set CMP_METRICS_SOCKET and CMP_METRICS_CONTAINER to explicitly select a Docker target")
	}
	if !filepath.IsAbs(socket) || !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`).MatchString(container) {
		t.Fatal("specify an absolute socket and a single container name or ID")
	}
	dir := t.TempDir()
	store, err := openStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	open := true
	t.Cleanup(func() {
		if open {
			if err := store.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Docker API redirect refused")
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/v1.40/containers/"+container+"/stats?stream=true", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Docker stats returned HTTP %d", response.StatusCode)
	}
	start := time.Now()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16<<20))
	var count, rows int
	metrics := map[string]int{}
	var lastTimestamp int64
	for {
		var stats dockerStats
		if err := decoder.Decode(&stats); err != nil {
			if errors.Is(err, context.DeadlineExceeded) && count > 0 {
				break
			}
			t.Fatalf("read stats after %d samples: %v", count, err)
		}
		points, err := samplePoints(stats)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range points {
			metrics[p.metric]++
		}
		if timestamp := stats.Read.UnixNano(); timestamp > lastTimestamp {
			lastTimestamp = timestamp
		}
		n, err := record(store, stats)
		if err != nil {
			t.Fatal(err)
		}
		count++
		rows += n
	}
	runtime.ReadMemStats(&after)
	open = false
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var bytes int64
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			bytes += info.Size()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	store, err = openStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	open = true
	for metric, expected := range metrics {
		saved, err := store.Select(metric, nil, 1, lastTimestamp+1)
		if err != nil || len(saved) != expected {
			t.Fatalf("reopen %s: got %d points, want %d: %v", metric, len(saved), expected, err)
		}
	}
	open = false
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("duration=%s samples=%d points=%d storage_bytes=%d go_allocated_bytes=%d observed_metrics=%v", time.Since(start).Round(time.Millisecond), count, rows, bytes, after.TotalAlloc-before.TotalAlloc, metrics)
}
