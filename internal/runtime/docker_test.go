package runtime

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestNewClientBoundsResponseHeadersWithoutLimitingEventStreams(t *testing.T) {
	client := NewClient("unused")
	transport, ok := client.stream.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("stream transport type = %T", client.stream.Transport)
	}
	if transport.ResponseHeaderTimeout != dockerResponseHeaderTimeout {
		t.Fatalf("response header timeout = %s, want %s", transport.ResponseHeaderTimeout, dockerResponseHeaderTimeout)
	}
	if client.stream.Timeout != 0 {
		t.Fatalf("event stream client timeout = %s, want no whole-response timeout", client.stream.Timeout)
	}
	if client.http.Timeout != 5*time.Second {
		t.Fatalf("request client timeout = %s, want 5s", client.http.Timeout)
	}
}

func TestSnapshotGroupsAllContainerStatesByComposeName(t *testing.T) {
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.40/containers/json" || request.URL.Query().Get("all") != "1" {
			t.Fatalf("unexpected Docker request: %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`[
			{"Names":["/first"],"State":"running","Labels":{"com.docker.compose.project":"shared","com.docker.compose.service":"web"}},
			{"Names":["/second"],"State":"exited","Labels":{"com.docker.compose.project":"shared","com.docker.compose.service":"db"}},
			{"Names":["/other"],"State":"running","Labels":{"com.docker.compose.project":"other"}},
			{"Names":["/unmanaged"],"State":"running","Labels":{}}
		]`))}, nil
	})}}
	got, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Containers) != 4 || len(got.ByProject) != 2 || len(got.ByProject["shared"]) != 2 || got.ByProject["shared"][0].Name != "first" ||
		got.ByProject["shared"][0].Service != "web" || got.ByProject["shared"][0].State != "running" ||
		got.ByProject["shared"][1].State != "exited" || len(got.ByProject["other"]) != 1 || got.Containers[3].Project != "" {
		t.Fatalf("unexpected snapshot: %#v", got)
	}
}

func TestSnapshotFailureIsNotAnEmptyResult(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{name: "daemon error", err: errors.New("connection failed")},
		{name: "API error", status: http.StatusForbidden, body: "denied"},
		{name: "malformed JSON", status: http.StatusOK, body: "["},
		{name: "null", status: http.StatusOK, body: "null"},
		{name: "too large", status: http.StatusOK, body: strings.Repeat(" ", maxResponseBytes+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				if tt.err != nil {
					return nil, tt.err
				}
				return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})}}
			got, err := client.Snapshot(context.Background())
			if err == nil || got != nil {
				t.Fatalf("snapshot = %#v, error = %v", got, err)
			}
		})
	}
}

func TestSnapshotEmptyArray(t *testing.T) {
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("[]"))}, nil
	})}}
	got, err := client.Snapshot(context.Background())
	if err != nil || len(got.Containers) != 0 || len(got.ByProject) != 0 {
		t.Fatalf("snapshot = %#v, error = %v", got, err)
	}
}
