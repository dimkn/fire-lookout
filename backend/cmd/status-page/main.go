// Command status-page runs the local status-page reader: it applies the SQLite schema
// migrations and serves the HTTP API under /api.
//
// The frontend is not embedded yet — in development Vite serves it and proxies /api
// here (AGENTS.md §9). Embedding lands with the infra pillar.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fire-lookout/backend/internal/adapters/feed"
	"fire-lookout/backend/internal/adapters/httpapi"
	"fire-lookout/backend/internal/adapters/scheduler"
	"fire-lookout/backend/internal/adapters/storage"
	"fire-lookout/backend/internal/application"
)

// version is injected at build time from the root VERSION file (AGENTS.md §7):
//
//	go build -ldflags "-X main.version=$(cat ../VERSION)" ./cmd/status-page
var version = "dev"

const shutdownGrace = 10 * time.Second

func main() {
	addr := flag.String("addr", env("RSS_READER_ADDR", ":8080"), "address to listen on")
	dbPath := flag.String("db", env("RSS_READER_DB", "./data/fire-lookout.db"), "path to the SQLite database file")
	pollTick := flag.Duration("poll-tick", envDuration("RSS_READER_POLL_TICK", scheduler.DefaultTick),
		"how often to look for feeds that are due (each feed's own cadence still applies)")
	flag.Parse()

	if err := run(*addr, *dbPath, *pollTick); err != nil {
		slog.Error("status-page failed", "error", err)
		os.Exit(1)
	}
}

func run(addr, dbPath string, pollTick time.Duration) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	if err := storage.Migrate(ctx, db); err != nil {
		return err
	}

	repo := storage.NewRepository(db)
	fetcher := feed.NewFetcher(feed.DefaultTimeout, version)

	handler := httpapi.NewRouter(
		httpapi.NewServer(
			application.NewStatusService(repo, time.Now),
			application.NewSubscriptionService(repo, fetcher),
		),
		"/api",
	)

	// The poller shares the signal-derived context, so Ctrl-C stops it with the server.
	poller := application.NewPollService(repo, fetcher, time.Now)
	pollDone := make(chan struct{})
	go func() {
		defer close(pollDone)
		scheduler.Run(ctx, pollTick, poller)
	}()

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("listening", "version", version, "addr", addr, "db", dbPath)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- fmt.Errorf("serve: %w", err)
			return
		}
		errs <- nil
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		// Let an in-flight poll finish before the database handle closes.
		select {
		case <-pollDone:
		case <-shutdownCtx.Done():
			slog.Warn("poller did not stop within the shutdown grace period")
		}
		return nil
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envDuration reads a Go duration ("30s", "5m") from the environment, falling back when it is
// unset or unparsable.
func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("ignoring unparsable duration", "env", key, "value", raw, "error", err)
		return fallback
	}
	return parsed
}
