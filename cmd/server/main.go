package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/config"
	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
	"github.com/mstrhakr/go-compose-manager-plus/internal/web"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check the running service and exit")
	showVersion := flag.Bool("version", false, "print build version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s commit=%s built=%s\n", version, commit, date)
		return
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	if *healthcheck {
		err = checkHealth(cfg.Addr)
	} else {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err = run(ctx, cfg, logger)
	}
	if err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if ctx.Err() != nil {
		return nil
	}
	database, err := auth.OpenDatabase(ctx, cfg.DataDir)
	if err != nil {
		return fmt.Errorf("open application state: %w", err)
	}
	defer database.SQL.Close()
	authManager, err := auth.NewManager(ctx, database, cfg.PublicURL.Scheme == "https", cfg.TrustedProxyRanges)
	if err != nil {
		return fmt.Errorf("initialize authentication: %w", err)
	}
	if err := authManager.Bootstrap(ctx, cfg.BootstrapUsername, cfg.BootstrapPassword); err != nil {
		return fmt.Errorf("initialize administrator: %w", err)
	}
	var docker *runtime.Client
	if cfg.DockerSocket != "" {
		docker = runtime.NewClient(cfg.DockerSocket)
	}
	metricsSettings := metrics.NewSettingsStore(database.SQL)
	collector, err := metrics.NewCollector(cfg.DataDir, metricsSettings, discovery.NewStore(database.SQL), docker, logger)
	if err != nil {
		return fmt.Errorf("initialize metrics collector: %w", err)
	}
	collectorCtx, cancelCollector := context.WithCancel(ctx)
	collectorDone := make(chan error, 1)
	go func() {
		collectorDone <- collector.Run(collectorCtx)
	}()
	defer func() {
		cancelCollector()
		if err := <-collectorDone; err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("metrics collector stopped", "error", err)
		}
	}()
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           web.Handler(authManager, discovery.NewStore(database.SQL), metricsSettings, collector, cfg.PublicURL, cfg.TrustedProxyRanges, logger, docker),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	logger.Info("server starting", "addr", cfg.Addr)
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return fmt.Errorf("shutdown: %w", err)
		}
		if err := <-result; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		logger.Info("server stopped")
		return nil
	}
}

func checkHealth(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned HTTP %d", response.StatusCode)
	}
	return nil
}
