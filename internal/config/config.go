package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Addr               string
	LogLevel           slog.Level
	DataDir            string
	DockerSocket       string
	PublicURL          *url.URL
	TrustedProxyRanges []netip.Prefix
	BootstrapUsername  string
	BootstrapPassword  string
}

func Load() (Config, error) {
	cfg := Config{Addr: "127.0.0.1:8080", LogLevel: slog.LevelInfo, DataDir: "./data"}
	if value := os.Getenv("APP_ADDR"); value != "" {
		cfg.Addr = value
	}
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return Config{}, fmt.Errorf("APP_ADDR must be host:port: %w", err)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, fmt.Errorf("APP_ADDR port must be between 1 and 65535")
	}
	if value := os.Getenv("APP_LOG_LEVEL"); value != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(value)); err != nil {
			return Config{}, fmt.Errorf("invalid APP_LOG_LEVEL: %w", err)
		}
	}
	if value := os.Getenv("APP_DATA_DIR"); value != "" {
		cfg.DataDir = value
	}
	if value := os.Getenv("APP_DOCKER_SOCKET"); value != "" {
		if !filepath.IsAbs(value) {
			return Config{}, fmt.Errorf("APP_DOCKER_SOCKET must be an absolute filesystem path")
		}
		cfg.DockerSocket = filepath.Clean(value)
	}
	publicURL := os.Getenv("APP_PUBLIC_URL")
	if publicURL == "" {
		if host == "" {
			host = "127.0.0.1"
		}
		publicURL = "http://" + net.JoinHostPort(host, port)
	}
	parsedURL, err := url.Parse(publicURL)
	if err != nil || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || (parsedURL.Path != "" && parsedURL.Path != "/") {
		return Config{}, fmt.Errorf("APP_PUBLIC_URL must be an origin without credentials, path, query or fragment")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return Config{}, fmt.Errorf("APP_PUBLIC_URL must use http or https")
	}
	if parsedURL.Scheme == "http" && !isLoopbackHost(parsedURL.Hostname()) {
		return Config{}, fmt.Errorf("APP_PUBLIC_URL requires https except for loopback development")
	}
	parsedURL.Path = ""
	cfg.PublicURL = parsedURL
	cfg.BootstrapUsername = os.Getenv("APP_BOOTSTRAP_USERNAME")
	cfg.BootstrapPassword = os.Getenv("APP_BOOTSTRAP_PASSWORD")
	if value := os.Getenv("APP_TRUSTED_PROXIES"); value != "" {
		for _, rawPrefix := range strings.Split(value, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(rawPrefix))
			if err != nil {
				return Config{}, fmt.Errorf("invalid APP_TRUSTED_PROXIES entry")
			}
			cfg.TrustedProxyRanges = append(cfg.TrustedProxyRanges, prefix.Masked())
		}
	}
	return cfg, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}
