package metrics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestSyntheticHourSQLiteCapacity(t *testing.T) {
	if os.Getenv("CMP_METRICS_SQLITE_CAPACITY_TEST") != "1" {
		t.Skip("set CMP_METRICS_SQLITE_CAPACITY_TEST=1 to run the synthetic SQLite measurement")
	}
	ctx := context.Background()
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	for _, containers := range []int{1, 5, 25, 50} {
		t.Run(fmt.Sprint(containers), func(t *testing.T) {
			dir := t.TempDir()
			store, err := Open(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			open := true
			defer func() {
				if open {
					if err := store.Close(); err != nil {
						t.Error(err)
					}
				}
			}()
			ids := make([]string, containers)
			for i := range ids {
				ids[i] = fmt.Sprintf("%064x", i+1)
			}
			start := time.Now()
			for minute := 0; minute < 60; minute++ {
				samples := make([]Sample, 0, containers*60)
				for offset := 0; offset < 60; offset++ {
					second := minute*60 + offset
					for i, id := range ids {
						samples = append(samples, capacitySample(id, i, second, base))
					}
				}
				if err := store.Insert(ctx, samples); err != nil {
					t.Fatal(err)
				}
			}
			elapsed := time.Since(start)
			before := fileSizes(t, dir)
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			open = false
			after := fileSizes(t, dir)
			store, err = Open(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			open = true
			for i, id := range ids {
				samples, err := store.Select(ctx, id, base, base.Add(time.Hour), 3600)
				if err != nil || len(samples) != 3600 {
					t.Fatalf("reopened samples for %s: %d, error %v", id, len(samples), err)
				}
				for second, sample := range samples {
					if !reflect.DeepEqual(sample, capacitySample(id, i, second, base)) {
						t.Fatalf("reopened sample %s at second %d differs: %#v", id, second, sample)
					}
				}
			}
			t.Logf("containers=%d samples=%d elapsed=%s at_ingest=%v after_close=%v",
				containers, containers*3600, elapsed.Round(time.Millisecond), before, after)
		})
	}
}

func capacitySample(id string, container, second int, base time.Time) Sample {
	cpu := float64((container*3+second)%200) / 2
	memory := uint64(100000000 + container*123456 + second*100)
	limit := uint64(1073741824 + container*4096)
	rx := float64((second*37 + container*13) % 100000)
	tx := float64((second*31 + container*7) % 100000)
	read := float64((second*17 + container*5) % 10000)
	write := float64((second*23 + container*11) % 10000)
	return Sample{ContainerID: id, Observation: runtime.Observations{
		Time: base.Add(time.Duration(second) * time.Second), CPUPercent: &cpu,
		MemoryUsageBytes: &memory, MemoryLimitBytes: &limit,
		NetworkRxBytesPerSec: &rx, NetworkTxBytesPerSec: &tx,
		BlockReadBytesPerSec: &read, BlockWriteBytesPerSec: &write,
	}}
}

func fileSizes(t *testing.T, dir string) map[string]int64 {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]int64, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			t.Fatalf("unexpected metrics storage entry %s", entry.Name())
		}
		info, err := os.Stat(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = info.Size()
	}
	return result
}
