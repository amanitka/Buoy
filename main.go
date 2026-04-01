package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	config := LoadConfig()
	setupLogger()
	client := initializeClient(config)
	ctx := setupContext()

	slog.Info("🚢 Buoy is leaving the dock: Starting informers...")
	if err := client.Start(ctx); err != nil {
		slog.Error("❌ Failed to start informers", "error", err)
		os.Exit(1)
	}
	slog.Info("⚓ Buoy is now watching the cluster for changes")

	updater := NewUpdater(client.clientset)
	scheduler := NewScheduler(client, client.registry, updater)
	scheduler.Start(ctx)

	server, err := NewServer(client.registry, updater)
	if err != nil {
		slog.Error("❌ Failed to create HTTP server", "error", err)
		os.Exit(1)
	}

	go func() {
		if err := server.Start(ctx, ":8080"); err != nil {
			slog.Error("❌ HTTP server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("🛑 Buoy is shutting down gracefully")
}

func setupLogger() {
	var level slog.Level
	if os.Getenv("DEBUG") == "true" {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{
		Level: level,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}

func initializeClient(config *Config) *Client {
	client, err := NewClient(config)
	if err != nil {
		slog.Error("❌ Fatal Error: Could not drop anchor", "error", err)
		os.Exit(1)
	}
	return client
}

func setupContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		slog.Info("🌊 Received shutdown signal")
		cancel()
	}()
	return ctx
}
