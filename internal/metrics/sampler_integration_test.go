package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	docker "github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestSamplerMockDockerToManagedStorage(t *testing.T) {
	listener, socket := newDockerSocket(t)
	id := strings.Repeat("a", 64)
	base := time.Now().UTC().Truncate(time.Second)
	var requests atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{},"State":{"Status":"running","StartedAt":%q}}`, id, base.Add(-time.Minute).Format(time.RFC3339Nano))
		case "/v1.40/containers/" + id + "/stats":
			if r.URL.Query().Get("stream") != "false" {
				t.Error("unexpected stats request")
			}
			n := requests.Add(1)
			fmt.Fprintf(w, `{"read":%q,"cpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":1},"memory_stats":{"usage":0,"limit":1024}}`,
				base.Add(time.Duration(n-1)*time.Second).Format(time.RFC3339Nano), n*10, n*100)
		default:
			t.Errorf("unexpected Docker request %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	sampler, err := NewSampler(docker.NewClient(socket))
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenManaged(context.Background(), t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	ctx := context.Background()
	for i := range 2 {
		at := base.Add(time.Duration(i) * time.Second)
		state, sample, err := sampler.Observe(ctx, id, at, i > 0)
		if err != nil || sample == nil {
			t.Fatalf("state=%#v sample=%#v error=%v", state, sample, err)
		}
		if err := store.InsertStates(ctx, at, []State{state}); err != nil {
			t.Fatal(err)
		}
		if err := store.Insert(ctx, at, []Sample{*sample}); err != nil {
			t.Fatal(err)
		}
	}
	samples, err := store.Select(ctx, id, base, base.Add(2*time.Second), 2)
	if err != nil || len(samples) != 2 || samples[0].Observation.CPUPercent != nil ||
		samples[1].Observation.CPUPercent == nil || *samples[1].Observation.CPUPercent != 10 ||
		samples[0].Observation.MemoryUsageBytes == nil || *samples[0].Observation.MemoryUsageBytes != 0 {
		t.Fatalf("samples=%#v error=%v", samples, err)
	}
	states, err := store.SelectStates(ctx, id, base, base.Add(2*time.Second), 2)
	if err != nil || len(states) != 2 || states[0].Health != HealthNone {
		t.Fatalf("states=%#v error=%v", states, err)
	}
}
