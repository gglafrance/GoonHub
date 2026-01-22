package main

import (
	"goonhub/internal/wire"
	"log"
	"os"
)

func main() {
	// Initialize Server using Wire
	// Config path can be set via environment variable or use default
	configPath := ""
	if path := os.Getenv("GOONHUB_CONFIG"); path != "" {
		configPath = path
	}
	srv, err := wire.InitializeServer(configPath)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start Server
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
