package main

import (
	"context"
	"log/slog"
	"os"
)

func main() {
	// 1. Initialize the logger
	// Using NewTextHandler makes the emojis look great in your terminal
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 2. Client Setup
	client, err := NewClient()
	if err != nil {
		slog.Error("❌ Fatal Error: Could not drop anchor", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// 3. Scan Cluster
	slog.Info("🚢 Buoy is leaving the dock: Scanning cluster...")

	resources, err := client.GetAllWatchableResources(ctx, "")
	if err != nil {
		slog.Error("🌊 Storm detected: Failed to fetch resources", "error", err)
		return
	}

	// 4. Log the results with your anchor
	slog.Info("📋 Scan complete", "total_found", len(resources))

	for _, r := range resources {
		// We put the anchor in the message string, and the data in the attributes
		slog.Info("⚓ Watching resource",
			"name", r.Name,
			"namespace", r.Namespace,
			"kind", r.Kind,
		)
	}

}
