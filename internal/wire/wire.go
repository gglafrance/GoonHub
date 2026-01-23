//go:build wireinject
// +build wireinject

package wire

import (
	"time"

	"goonhub/internal/api"
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/logging"
	"goonhub/internal/infrastructure/persistence/postgres"
	"goonhub/internal/infrastructure/server"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

func InitializeServer(cfgPath string) (*server.Server, error) {
	wire.Build(
		// Config
		config.Load,

		// Infrastructure
		logging.New,
		postgres.NewDB,

		// Data
		provideVideoRepository,
		provideUserRepository,
		provideRevokedTokenRepository,
		provideUserSettingsRepository,
		provideRoleRepository,
		providePermissionRepository,

		// Core
		provideVideoProcessingService,
		provideVideoService,
		provideAuthService,
		provideUserService,
		provideSettingsService,
		provideRBACService,
		provideAdminService,

		// API Middleware
		provideRateLimiter,

		// API
		provideVideoHandler,
		provideAuthHandler,
		provideSettingsHandler,
		provideAdminHandler,
		provideRouter,

		// Server
		provideServer,
	)
	return &server.Server{}, nil
}

// Providers adapters

func provideRateLimiter(cfg *config.Config) *middleware.IPRateLimiter {
	rl := rate.Every(time.Minute / time.Duration(cfg.Auth.LoginRateLimit))
	return middleware.NewIPRateLimiter(rl, cfg.Auth.LoginRateBurst)
}

func provideVideoRepository(db *gorm.DB) data.VideoRepository {
	return data.NewVideoRepository(db)
}

func provideUserRepository(db *gorm.DB) data.UserRepository {
	return data.NewUserRepository(db)
}

func provideRevokedTokenRepository(db *gorm.DB) data.RevokedTokenRepository {
	return data.NewRevokedTokenRepository(db)
}

func provideVideoService(repo data.VideoRepository, cfg *config.Config, processingService *core.VideoProcessingService, logger *logging.Logger) *core.VideoService {
	dataPath := "./data"
	return core.NewVideoService(repo, dataPath, processingService, logger.Logger)
}

func provideVideoProcessingService(repo data.VideoRepository, cfg *config.Config, logger *logging.Logger) *core.VideoProcessingService {
	return core.NewVideoProcessingService(repo, cfg.Processing, logger.Logger)
}

func provideAuthService(userRepo data.UserRepository, revokedRepo data.RevokedTokenRepository, cfg *config.Config, logger *logging.Logger) *core.AuthService {
	return core.NewAuthService(userRepo, revokedRepo, cfg.Auth.PasetoSecret, cfg.Auth.TokenDuration, logger.Logger)
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

func provideUserSettingsRepository(db *gorm.DB) data.UserSettingsRepository {
	return data.NewUserSettingsRepository(db)
}

func provideSettingsService(settingsRepo data.UserSettingsRepository, userRepo data.UserRepository, logger *logging.Logger) *core.SettingsService {
	return core.NewSettingsService(settingsRepo, userRepo, logger.Logger)
}

func provideSettingsHandler(settingsService *core.SettingsService) *handler.SettingsHandler {
	return handler.NewSettingsHandler(settingsService)
}

func provideRoleRepository(db *gorm.DB) data.RoleRepository {
	return data.NewRoleRepository(db)
}

func providePermissionRepository(db *gorm.DB) data.PermissionRepository {
	return data.NewPermissionRepository(db)
}

func provideRBACService(roleRepo data.RoleRepository, permRepo data.PermissionRepository, logger *logging.Logger) *core.RBACService {
	svc, err := core.NewRBACService(roleRepo, permRepo, logger.Logger)
	if err != nil {
		panic(err)
	}
	return svc
}

func provideAdminService(userRepo data.UserRepository, roleRepo data.RoleRepository, rbac *core.RBACService, logger *logging.Logger) *core.AdminService {
	return core.NewAdminService(userRepo, roleRepo, rbac, logger.Logger)
}

func provideAdminHandler(adminService *core.AdminService, rbacService *core.RBACService) *handler.AdminHandler {
	return handler.NewAdminHandler(adminService, rbacService)
}

func provideRouter(logger *logging.Logger, cfg *config.Config, videoHandler *handler.VideoHandler, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, adminHandler *handler.AdminHandler, authService *core.AuthService, rbacService *core.RBACService, rateLimiter *middleware.IPRateLimiter) *gin.Engine {
	return api.NewRouter(logger, cfg, videoHandler, authHandler, settingsHandler, adminHandler, authService, rbacService, rateLimiter)
}

func provideServer(router *gin.Engine, logger *logging.Logger, cfg *config.Config, processingService *core.VideoProcessingService, userService *core.UserService) *server.Server {
	return server.NewHTTPServer(router, logger, cfg, processingService, userService)
}
