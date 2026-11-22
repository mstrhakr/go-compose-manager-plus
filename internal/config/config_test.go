package config

import (
	"log/slog"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tt := range []struct {
		name, addr, level, dataDir, publicURL, trustedProxies string
		wantAddr                                              string
		wantLevel                                             slog.Level
		wantDataDir                                           string
		wantPublicURL                                         string
		wantErr                                               bool
	}{
		{name: "defaults", wantAddr: "127.0.0.1:8080", wantLevel: slog.LevelInfo, wantDataDir: "./data", wantPublicURL: "http://127.0.0.1:8080"},
		{name: "overrides", addr: "127.0.0.1:9000", level: "debug", dataDir: "/tmp/app", publicURL: "http://localhost:9000", trustedProxies: "127.0.0.0/8", wantAddr: "127.0.0.1:9000", wantLevel: slog.LevelDebug, wantDataDir: "/tmp/app", wantPublicURL: "http://localhost:9000"},
		{name: "derived public origin", addr: "127.0.0.1:9000", wantAddr: "127.0.0.1:9000", wantLevel: slog.LevelInfo, wantDataDir: "./data", wantPublicURL: "http://127.0.0.1:9000"},
		{name: "HTTPS public origin", publicURL: "https://manager.example.com", wantAddr: "127.0.0.1:8080", wantLevel: slog.LevelInfo, wantDataDir: "./data", wantPublicURL: "https://manager.example.com"},
		{name: "IPv6", addr: "[::1]:8080", wantAddr: "[::1]:8080", wantLevel: slog.LevelInfo, wantDataDir: "./data", wantPublicURL: "http://[::1]:8080"},
		{name: "missing port", addr: "localhost", wantErr: true},
		{name: "invalid port", addr: ":http", wantErr: true},
		{name: "zero port", addr: ":0", wantErr: true},
		{name: "large port", addr: ":65536", wantErr: true},
		{name: "negative port", addr: ":-1", wantErr: true},
		{name: "invalid level", level: "verbose", wantErr: true},
		{name: "remote HTTP origin", publicURL: "http://manager.example.com", wantErr: true},
		{name: "origin with path", publicURL: "https://manager.example.com/app", wantErr: true},
		{name: "invalid trusted proxy", trustedProxies: "not-a-cidr", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_ADDR", tt.addr)
			t.Setenv("APP_LOG_LEVEL", tt.level)
			t.Setenv("APP_DATA_DIR", tt.dataDir)
			t.Setenv("APP_DOCKER_SOCKET", "")
			t.Setenv("APP_PUBLIC_URL", tt.publicURL)
			t.Setenv("APP_TRUSTED_PROXIES", tt.trustedProxies)
			t.Setenv("APP_BOOTSTRAP_USERNAME", "")
			t.Setenv("APP_BOOTSTRAP_PASSWORD", "")
			got, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && (got.Addr != tt.wantAddr || got.LogLevel != tt.wantLevel || got.DataDir != tt.wantDataDir || got.PublicURL == nil || got.PublicURL.String() != tt.wantPublicURL) {
				t.Fatalf("Load() = %+v", got)
			}

		})
	}
}

func TestDockerSocketConfiguration(t *testing.T) {
	t.Setenv("APP_ADDR", "")
	t.Setenv("APP_PUBLIC_URL", "")
	t.Setenv("APP_LOG_LEVEL", "")
	t.Setenv("APP_TRUSTED_PROXIES", "")
	t.Setenv("APP_DOCKER_SOCKET", "relative.sock")
	if _, err := Load(); err == nil {
		t.Fatal("expected relative socket path to be rejected")
	}
	socket := filepath.Join(t.TempDir(), "docker.sock")
	t.Setenv("APP_DOCKER_SOCKET", socket)
	got, err := Load()
	if err != nil || got.DockerSocket != socket {
		t.Fatalf("Load socket = %q, error = %v", got.DockerSocket, err)
	}
}
