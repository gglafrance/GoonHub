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
		provideUserRepository,

		// Core
		provideVideoProcessingService,
		provideVideoService,
		provideAuthService,
		provideUserService,

		// API
		provideVideoHandler,
		provideAuthHandler,
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

func provideUserRepository(db *gorm.DB) data.UserRepository {
	return data.NewSQLiteUserRepository(db)
}

func provideVideoService(repo data.VideoRepository, cfg *config.Config, processingService *core.VideoProcessingService, logger *logging.Logger) *core.VideoService {
	dataPath := "./data"
	return core.NewVideoService(repo, dataPath, processingService, logger.Logger)
}

func provideVideoProcessingService(repo data.VideoRepository, cfg *config.Config, logger *logging.Logger) *core.VideoProcessingService {
	return core.NewVideoProcessingService(repo, cfg.Processing, logger.Logger)
}

func provideAuthService(userRepo data.UserRepository, cfg *config.Config, logger *logging.Logger) *core.AuthService {
	return core.NewAuthService(userRepo, cfg.Auth.PasetoSecret, cfg.Auth.TokenDuration, logger.Logger)
}

func provideUserService(userRepo data.UserRepository, logger *logging.Logger) *core.UserService {
	return core.NewUserService(userRepo, logger.Logger)
}

func provideVideoHandler(service *core.VideoService, processingService *core.VideoProcessingService) *handler.VideoHandler {
	return handler.NewVideoHandler(service, processingService)
}

func provideAuthHandler(authService *core.AuthService, userService *core.UserService) *handler.AuthHandler {
	return handler.NewAuthHandler(authService, userService)
}

func provideServer(router *gin.Engine, logger *logging.Logger, cfg *config.Config, processingService *core.VideoProcessingService, userService *core.UserService) *server.Server {
	return server.NewHTTPServer(router, logger, cfg, processingService, userService)
}
