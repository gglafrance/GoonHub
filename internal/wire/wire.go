//go:build wireinject
// +build wireinject

package wire

import (
	"goonhub/internal/api"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/logging"
	"goonhub/internal/infrastructure/persistence/sqlite"
	"goonhub/internal/infrastructure/server"

	"github.com/google/wire"
	"gorm.io/gorm"
)

func InitializeServer(cfgPath string) (*server.Server, error) {
	wire.Build(
		// Config
		config.Load,

		// Infrastructure
		logging.New,
		sqlite.NewDB,

		// Data
		provideVideoRepository,

		// Core
		provideVideoService,

		// API
		handler.NewVideoHandler,
		api.NewRouter,

		// Server
		server.NewHTTPServer,
	)
	return &server.Server{}, nil
}

// Providers adapters

func provideVideoRepository(db *gorm.DB) data.VideoRepository {
	return data.NewSQLiteVideoRepository(db)
}

func provideVideoService(repo data.VideoRepository, cfg *config.Config) *core.VideoService {
	// Extract data path from config.
	// In config.go we used SetDefault("database.source") but didn't explicitly map a "data_path".
	// Let's assume it's in the root or add it to config.
	dataPath := "./data" // Default
	// Check env var directly or add to config. Let's use a hardcoded default relative to config for now to match current behavior
	return core.NewVideoService(repo, dataPath)
}
