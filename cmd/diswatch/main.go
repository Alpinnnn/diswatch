package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"diswatch/internal/app"
	"diswatch/internal/config"
	"diswatch/internal/web"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	dataDir := env("DISWATCH_DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		logger.Error("failed to create data directory", "err", err)
		os.Exit(1)
	}

	store, err := config.Open(filepath.Clean(dataDir), logger)
	if err != nil {
		logger.Error("failed to open config store", "err", err)
		os.Exit(1)
	}

	runtime := app.New(store, logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go runtime.Run(ctx)

	addr := env("DISWATCH_ADDR", ":8080")
	server := &http.Server{
		Addr:              addr,
		Handler:           web.NewServer(runtime, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("diswatch listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown did not finish cleanly", "err", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
