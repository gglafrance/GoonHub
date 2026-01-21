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

	"github.com/gin-gonic/gin"
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
		provideVideoProcessingService,
		provideVideoService,

		// API
		provideVideoHandler,
		api.NewRouter,

		// Server
		provideServer,
	)
	return &server.Server{}, nil
}

// Providers adapters

func provideVideoRepository(db *gorm.DB) data.VideoRepository {
	return data.NewSQLiteVideoRepository(db)
}

func provideVideoService(repo data.VideoRepository, cfg *config.Config, processingService *core.VideoProcessingService, logger *logging.Logger) *core.VideoService {
	dataPath := "./data"
	return core.NewVideoService(repo, dataPath, processingService, logger.Logger)
}

func provideVideoProcessingService(repo data.VideoRepository, cfg *config.Config, logger *logging.Logger) *core.VideoProcessingService {
	return core.NewVideoProcessingService(repo, cfg.Processing, logger.Logger)
}

func provideVideoHandler(service *core.VideoService, processingService *core.VideoProcessingService) *handler.VideoHandler {
	return handler.NewVideoHandler(service, processingService)
}

func provideServer(router *gin.Engine, logger *logging.Logger, cfg *config.Config, processingService *core.VideoProcessingService) *server.Server {
	return server.NewHTTPServer(router, logger, cfg, processingService)
}
