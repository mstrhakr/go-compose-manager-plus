package metricspoc

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nakabonne/tstorage"
)

type capacityLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *capacityLogger) Printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

func (l *capacityLogger) check(t *testing.T) {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.messages) != 0 {
		t.Errorf("storage reported: %v", l.messages)
	}
}

func TestSyntheticHourCapacity(t *testing.T) {
	if os.Getenv("CMP_METRICS_CAPACITY_TEST") != "1" {
		t.Skip("set CMP_METRICS_CAPACITY_TEST=1 to run the synthetic storage measurement")
	}
	const seconds = 3600
	metrics := []string{"cpu_percent", "memory_usage_bytes", "network_rx_bytes", "network_tx_bytes", "block_read_bytes", "block_write_bytes"}
	for _, containers := range []int{1, 5, 25, 50} {
		t.Run(fmt.Sprintf("%d_containers", containers), func(t *testing.T) {
			dir := t.TempDir()
			logger := &capacityLogger{}
			store, err := tstorage.NewStorage(
				tstorage.WithDataPath(dir),
				tstorage.WithPartitionDuration(5*time.Minute),
				tstorage.WithRetention(24*time.Hour),
				tstorage.WithWALBufferedSize(0),
				tstorage.WithLogger(logger),
			)
			if err != nil {
				t.Fatal(err)
			}
			open := true
			t.Cleanup(func() {
				if open {
					if err := store.Close(); err != nil {
						t.Errorf("close synthetic store: %v", err)
					}
				}
			})
			start := time.Now().UTC().Truncate(time.Second).Add(-seconds * time.Second)
			labels := make([][]tstorage.Label, containers)
			for i := range labels {
				labels[i] = []tstorage.Label{{Name: "container", Value: fmt.Sprintf("%064x", i+1)}}
			}
			began := time.Now()
			for second := range seconds {
				rows := make([]tstorage.Row, 0, containers*len(metrics))
				for container := range containers {
					for metric, name := range metrics {
						value := float64((second*(metric+1)*(container+1))%10000) + float64(second%7)/10
						rows = append(rows, tstorage.Row{
							Metric: name, Labels: labels[container],
							DataPoint: tstorage.DataPoint{Timestamp: start.Add(time.Duration(second) * time.Second).UnixNano(), Value: value},
						})
					}
				}
				if err := store.InsertRows(rows); err != nil {
					t.Fatalf("insert at second %d: %v", second, err)
				}
			}
			liveBytes := directoryBytes(t, dir)
			open = false
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			closedBytes := directoryBytes(t, dir)
			store, err = tstorage.NewStorage(tstorage.WithDataPath(dir), tstorage.WithRetention(24*time.Hour), tstorage.WithLogger(logger))
			if err != nil {
				t.Fatal(err)
			}
			open = true
			for container := range containers {
				for _, metric := range metrics {
					points, err := store.Select(metric, labels[container], start.UnixNano(), start.Add(seconds*time.Second).UnixNano())
					if err != nil || len(points) != seconds {
						t.Errorf("container %d %s: reopened samples = %d, error = %v; want %d", container, metric, len(points), err, seconds)
					}
				}
			}
			open = false
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			logger.check(t)
			t.Logf("containers=%d points=%d live_bytes=%d closed_bytes=%d duration=%s",
				containers, containers*seconds*len(metrics), liveBytes, closedBytes, time.Since(began).Round(time.Millisecond))
		})
	}
}

func directoryBytes(t *testing.T, dir string) int64 {
	t.Helper()
	var total int64
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return total
}
