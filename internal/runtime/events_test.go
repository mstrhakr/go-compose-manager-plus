package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWatchContainerEvents(t *testing.T) {
	id := strings.Repeat("a", 64)
	body := fmt.Sprintf(
		"{\"Type\":\"network\",\"Action\":\"create\"}\n"+
			"{\"Type\":\"container\",\"Action\":\"attach\"}\n"+
			"{\"Type\":\"container\",\"Action\":\"start\",\"Actor\":{\"ID\":%q,\"Attributes\":{\"com.docker.compose.project\":\"shared\",\"com.docker.compose.service\":\"web\"}},\"timeNano\":1000000001}\n"+
			"{\"Type\":\"container\",\"Action\":\"health_status: healthy\",\"Actor\":{\"ID\":%q},\"timeNano\":2000000000}\n", id, id)
	client := &Client{stream: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.40/events" ||
			request.URL.Query().Get("filters") != `{"type":["container"]}` {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	var got []ContainerEvent
	err := client.WatchContainerEvents(context.Background(), func(event ContainerEvent) error {
		got = append(got, event)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "ended unexpectedly") {
		t.Fatalf("EOF error = %v", err)
	}
	want := []ContainerEvent{
		{ID: id, Project: "shared", Service: "web", Action: "start", Time: time.Unix(1, 1).UTC()},
		{ID: id, Action: "health_status", Health: "healthy", Time: time.Unix(2, 0).UTC()},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v", got)
	}
}

func TestWatchContainerEventFailures(t *testing.T) {
	id := strings.Repeat("a", 64)
	payload := func(action, eventID string, timestamp int) string {
		return fmt.Sprintf(`{"Type":"container","Action":%q,"Actor":{"ID":%q},"timeNano":%d}`, action, eventID, timestamp)
	}
	callbackError := errors.New("consumer failed")
	for _, tt := range []struct {
		name string
		body string
		code int
		fn   func(ContainerEvent) error
		want string
	}{
		{"status", "", http.StatusForbidden, nil, "HTTP 403"},
		{"malformed", "{", http.StatusOK, nil, "decode"},
		{"oversized", strings.Repeat("x", maxEventBytes+1), http.StatusOK, nil, "read"},
		{"invalid ID", payload("start", "bad", 1), http.StatusOK, nil, "container ID"},
		{"missing time", payload("start", id, 0), http.StatusOK, nil, "timestamp"},
		{"invalid health", payload("health_status: odd", id, 1), http.StatusOK, nil, "health"},
		{"bare health", payload("health_status", id, 1), http.StatusOK, nil, "health"},
		{"callback", payload("start", id, 1), http.StatusOK, func(ContainerEvent) error { return callbackError }, "consumer failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{stream: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tt.code, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})}}
			fn := tt.fn
			if fn == nil {
				fn = func(ContainerEvent) error { return nil }
			}
			err := client.WatchContainerEvents(context.Background(), fn)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestWatchContainerEventsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{stream: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		cancel()
		return nil, &url.Error{Op: "Get", URL: request.URL.String(), Err: request.Context().Err()}
	})}}
	err := client.WatchContainerEvents(ctx, func(ContainerEvent) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
}

func TestWatchContainerEventsInterruptsIdleStream(t *testing.T) {
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(ready)
		<-r.Context().Done()
	}))
	defer server.Close()
	destination, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{stream: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		copy := request.Clone(request.Context())
		copy.URL.Scheme, copy.URL.Host = destination.Scheme, destination.Host
		return http.DefaultTransport.RoundTrip(copy)
	})}}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- client.WatchContainerEvents(ctx, func(ContainerEvent) error { return nil })
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("event stream did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("idle event stream did not stop")
	}
}
