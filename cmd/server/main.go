package main

import (
	"goonhub/internal/wire"
	"log"
)

func main() {
	// Initialize Server using Wire
	// Empty config path for now (uses defaults + env)
	srv, err := wire.InitializeServer("")
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start Server
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
