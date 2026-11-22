package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/config"
)

func TestCheckHealth(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/healthz" {
				t.Errorf("unexpected health path: %s", r.URL.Path)
			}
			w.WriteHeader(status)
		}))
		err := checkHealth(server.Listener.Addr().String())
		server.Close()
		if (err == nil) != (status == http.StatusOK) {
			t.Errorf("status %d: error = %v", status, err)
		}
	}
	if err := checkHealth("invalid"); err == nil {
		t.Error("expected invalid address error")
	}
}

func TestRunShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, testConfig(t, "127.0.0.1:0"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
}

func TestRunListenFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := run(context.Background(), testConfig(t, listener.Addr().String()), slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Fatal("expected listen error")
	}
}

func testConfig(t *testing.T, addr string) config.Config {
	t.Helper()
	return config.Config{
		Addr:              addr,
		DataDir:           filepath.Join(t.TempDir(), "state"),
		BootstrapUsername: "operator",
		BootstrapPassword: "bootstrap-password-123",
		PublicURL:         mustTestURL(t, "http://127.0.0.1:8080"),
	}
}

func mustTestURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
