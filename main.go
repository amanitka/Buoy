package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	setupLogger()
	client := initializeClient()
	ctx := setupContext()

	slog.Info("🚢 Buoy is leaving the dock: Starting informers...")
	if err := client.Start(ctx); err != nil {
		slog.Error("❌ Failed to start informers", "error", err)
		os.Exit(1)
	}
	slog.Info("⚓ Buoy is now watching the cluster for changes")

	<-ctx.Done()
	slog.Info("🛑 Buoy is shutting down gracefully")
}

func setupLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)
}

func initializeClient() *Client {
	client, err := NewClient()
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
