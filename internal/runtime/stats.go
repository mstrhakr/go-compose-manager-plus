package runtime

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Stats is one Docker observation. Counter fields retain their presence so a
// missing value is not confused with an observed zero.
type Stats struct {
	Read time.Time `json:"read"`
	CPU  struct {
		Usage struct {
			Total  *uint64  `json:"total_usage"`
			PerCPU []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		System *uint64 `json:"system_cpu_usage"`
		Online *uint64 `json:"online_cpus"`
	} `json:"cpu_stats"`
	Memory struct {
		Usage *uint64 `json:"usage"`
		Limit *uint64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		Rx *uint64 `json:"rx_bytes"`
		Tx *uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlockIO struct {
		Entries []struct {
			Major *uint64 `json:"major"`
			Minor *uint64 `json:"minor"`
			Op    string  `json:"op"`
			Value *uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
}

// Observations uses nil for unavailable values. Memory usage is Docker's raw
// usage in bytes, not the cache-adjusted docker stats CLI value.
type Observations struct {
	Time                  time.Time
	CPUPercent            *float64
	MemoryUsageBytes      *uint64
	MemoryLimitBytes      *uint64
	NetworkRxBytesPerSec  *float64
	NetworkTxBytesPerSec  *float64
	BlockReadBytesPerSec  *float64
	BlockWriteBytesPerSec *float64
}

// ContainerStats requests a single read-only Docker stats observation by full
// container ID. Collection scheduling, history and retries belong to its caller.
func (c *Client) ContainerStats(ctx context.Context, id string) (Stats, error) {
	if len(id) != 64 {
		return Stats{}, errors.New("Docker container ID must be 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return Stats{}, fmt.Errorf("invalid Docker container ID: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/v1.40/containers/"+id+"/stats?stream=false", nil)
	if err != nil {
		return Stats{}, fmt.Errorf("create Docker stats request: %w", err)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return Stats{}, fmt.Errorf("request Docker stats: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Stats{}, fmt.Errorf("Docker stats returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Stats{}, fmt.Errorf("read Docker stats: %w", err)
	}
	if len(body) > maxResponseBytes {
		return Stats{}, errors.New("Docker stats exceed size limit")
	}
	var stats Stats
	if err := json.Unmarshal(body, &stats); err != nil {
		return Stats{}, fmt.Errorf("decode Docker stats: %w", err)
	}
	if stats.Read.IsZero() {
		return Stats{}, errors.New("Docker stats have no timestamp")
	}
	return stats, nil
}

// ObserveStats compares consecutive samples from the same uninterrupted
// container instance. Pass nil after a disconnect or identity change.
func ObserveStats(current Stats, previous *Stats) (Observations, error) {
	if current.Read.IsZero() {
		return Observations{}, errors.New("Docker stats have no timestamp")
	}
	result := Observations{Time: current.Read, MemoryUsageBytes: current.Memory.Usage, MemoryLimitBytes: current.Memory.Limit}
	if previous == nil {
		return result, nil
	}
	if previous.Read.IsZero() || !current.Read.After(previous.Read) {
		return Observations{}, errors.New("Docker stats timestamps must be strictly increasing")
	}
	elapsed := current.Read.Sub(previous.Read).Seconds()
	cpu, prevCPU := current.CPU.Usage.Total, previous.CPU.Usage.Total
	system, prevSystem := current.CPU.System, previous.CPU.System
	var cpus uint64
	if current.CPU.Online != nil {
		cpus = *current.CPU.Online
	}
	if cpus == 0 {
		cpus = uint64(len(current.CPU.Usage.PerCPU))
	}
	if cpu != nil && prevCPU != nil && system != nil && prevSystem != nil && *cpu >= *prevCPU && *system > *prevSystem && cpus > 0 {
		value := float64(*cpu-*prevCPU) / float64(*system-*prevSystem) * float64(cpus) * 100
		result.CPUPercent = &value
	}
	result.NetworkRxBytesPerSec = networkRate(current, *previous, elapsed, true)
	result.NetworkTxBytesPerSec = networkRate(current, *previous, elapsed, false)
	result.BlockReadBytesPerSec = blockRate(current, *previous, elapsed, "read")
	result.BlockWriteBytesPerSec = blockRate(current, *previous, elapsed, "write")
	return result, nil
}

func networkRate(current, previous Stats, elapsed float64, rx bool) *float64 {
	if len(current.Networks) == 0 || len(current.Networks) != len(previous.Networks) {
		return nil
	}
	var total float64
	for name, next := range current.Networks {
		old, ok := previous.Networks[name]
		if !ok {
			return nil
		}
		nextValue, oldValue := next.Tx, old.Tx
		if rx {
			nextValue, oldValue = next.Rx, old.Rx
		}
		if nextValue == nil || oldValue == nil || *nextValue < *oldValue {
			return nil
		}
		total += float64(*nextValue - *oldValue)
	}
	value := total / elapsed
	return &value
}

type blockDevice struct {
	major, minor uint64
}

func blockCounters(stats Stats, operation string) map[blockDevice]uint64 {
	result := make(map[blockDevice]uint64)
	for _, entry := range stats.BlockIO.Entries {
		if !strings.EqualFold(entry.Op, operation) {
			continue
		}
		if entry.Major == nil || entry.Minor == nil || entry.Value == nil {
			return nil
		}
		device := blockDevice{*entry.Major, *entry.Minor}
		if _, exists := result[device]; exists {
			return nil
		}
		result[device] = *entry.Value
	}
	return result
}

func blockRate(current, previous Stats, elapsed float64, operation string) *float64 {
	next, old := blockCounters(current, operation), blockCounters(previous, operation)
	if len(next) == 0 || len(next) != len(old) {
		return nil
	}
	var total float64
	for device, value := range next {
		prev, ok := old[device]
		if !ok || value < prev {
			return nil
		}
		total += float64(value - prev)
	}
	value := total / elapsed
	return &value
}
