package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestInspectContainerReadOnlyAndStates(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, tt := range []struct {
		name, body, state, health string
	}{
		{"healthy", `"Status":"running","StartedAt":"2026-11-22T20:00:00Z","Health":{"Status":"healthy"}`, "running", "healthy"},
		{"unknown absent config", `"Status":"exited","StartedAt":"0001-01-01T00:00:00Z"`, "exited", "unknown"},
		{"unknown health", `"Status":"running","Health":{"Status":"unexpected"}`, "running", "unknown"},
		{"unknown state", `"Status":"unexpected"`, "unknown", "unknown"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1.40/containers/"+id+"/json" {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL)
				}
				body := fmt.Sprintf(`{"Id":%q,"State":{%s}}`, id, tt.body)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}}
			got, err := client.InspectContainer(context.Background(), id)
			if err != nil || got.ID != id || got.State != tt.state || got.Health != tt.health {
				t.Fatalf("inspection=%#v error=%v", got, err)
			}
		})
	}
}

func TestInspectContainerRejectsInvalidResponse(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, body := range []string{
		`{`, `{"Id":"other","State":{"Status":"running"}}`,
		fmt.Sprintf(`{"Id":%q}`, id),
		fmt.Sprintf(`{"Id":%q,"State":{"Status":"running","StartedAt":"not-a-time"}}`, id),
	} {
		client := &Client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}}
		if _, err := client.InspectContainer(context.Background(), id); err == nil {
			t.Fatalf("accepted response %q", body)
		}

	}
	client := &Client{}
	for _, invalid := range []string{"", "../json", strings.Repeat("z", 64)} {
		if _, err := client.InspectContainer(context.Background(), invalid); err == nil {
			t.Fatalf("accepted ID %q", invalid)
		}
	}
}
