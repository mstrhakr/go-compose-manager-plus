package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestInspectConfiguredHealthDistinguishesNoneFromUnknown(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, tt := range []struct {
		config string
		want   string
	}{
		{`{}`, "none"},
		{`{"Healthcheck":{"Test":["NONE"]}}`, "none"},
		{`{"Healthcheck":{"Test":["CMD","true"]}}`, "unknown"},
	} {
		client := &Client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			body := fmt.Sprintf(`{"Id":%q,"Config":%s,"State":{"Status":"exited"}}`, id, tt.config)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}}
		got, err := client.InspectContainer(context.Background(), id)
		if err != nil || got.Health != tt.want {
			t.Fatalf("config=%s inspection=%#v error=%v", tt.config, got, err)
		}
	}
}
