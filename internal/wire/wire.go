//go:build wireinject
// +build wireinject

package wire

import (
	"fmt"
	"time"

	"goonhub/internal/api"
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/logging"
	"goonhub/internal/infrastructure/meilisearch"
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
		provideJobHistoryRepository,
		providePoolConfigRepository,
		provideProcessingConfigRepository,
		provideTriggerConfigRepository,

		provideTagRepository,
		provideInteractionRepository,
		provideWatchHistoryRepository,
		provideStoragePathRepository,
		provideScanHistoryRepository,
		provideDLQRepository,
		provideRetryConfigRepository,

		// Infrastructure
		provideMeilisearchClient,

		// Core
		provideEventBus,
		provideSearchService,
		provideJobHistoryService,
		provideVideoProcessingService,
		provideVideoService,
		provideAuthService,
		provideUserService,
		provideSettingsService,
		provideRBACService,
		provideAdminService,
		provideTagService,
		provideInteractionService,
		provideWatchHistoryService,
		provideTriggerScheduler,
		provideStoragePathService,
		provideScanService,
		provideRetryScheduler,
		provideDLQService,

		// API Middleware
		provideRateLimiter,

		// API
		provideVideoHandler,
		provideAuthHandler,
		provideSettingsHandler,
		provideAdminHandler,
		provideJobHandler,
		provideSSEHandler,
		provideTagHandler,
		provideInteractionHandler,
		provideSearchHandler,
		provideWatchHistoryHandler,
		provideStoragePathHandler,
		provideScanHandler,
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

func provideEventBus(logger *logging.Logger) *core.EventBus {
	return core.NewEventBus(logger.Logger)
}

func provideVideoService(repo data.VideoRepository, cfg *config.Config, processingService *core.VideoProcessingService, eventBus *core.EventBus, logger *logging.Logger) *core.VideoService {
	videoPath := "./data/videos"
	metadataPath := "./data/metadata"
	return core.NewVideoService(repo, videoPath, metadataPath, processingService, eventBus, logger.Logger)
}

func provideJobHistoryRepository(db *gorm.DB) data.JobHistoryRepository {
	return data.NewJobHistoryRepository(db)
}

func provideJobHistoryService(repo data.JobHistoryRepository, cfg *config.Config, logger *logging.Logger) *core.JobHistoryService {
	return core.NewJobHistoryService(repo, cfg.Processing, logger.Logger)
}

func provideVideoProcessingService(repo data.VideoRepository, cfg *config.Config, logger *logging.Logger, eventBus *core.EventBus, jobHistory *core.JobHistoryService, poolConfigRepo data.PoolConfigRepository, processingConfigRepo data.ProcessingConfigRepository, triggerConfigRepo data.TriggerConfigRepository) *core.VideoProcessingService {
	return core.NewVideoProcessingService(repo, cfg.Processing, logger.Logger, eventBus, jobHistory, poolConfigRepo, processingConfigRepo, triggerConfigRepo)
}

func provideJobHandler(jobHistoryService *core.JobHistoryService, processingService *core.VideoProcessingService, poolConfigRepo data.PoolConfigRepository, processingConfigRepo data.ProcessingConfigRepository, triggerConfigRepo data.TriggerConfigRepository, triggerScheduler *core.TriggerScheduler, dlqService *core.DLQService, retryConfigRepo data.RetryConfigRepository, retryScheduler *core.RetryScheduler) *handler.JobHandler {
	return handler.NewJobHandler(jobHistoryService, processingService, poolConfigRepo, processingConfigRepo, triggerConfigRepo, triggerScheduler, dlqService, retryConfigRepo, retryScheduler)
}

func providePoolConfigRepository(db *gorm.DB) data.PoolConfigRepository {
	return data.NewPoolConfigRepository(db)
}

func provideProcessingConfigRepository(db *gorm.DB) data.ProcessingConfigRepository {
	return data.NewProcessingConfigRepository(db)
}

func provideTriggerConfigRepository(db *gorm.DB) data.TriggerConfigRepository {
	return data.NewTriggerConfigRepository(db)
}

func provideTriggerScheduler(triggerConfigRepo data.TriggerConfigRepository, videoRepo data.VideoRepository, processingService *core.VideoProcessingService, logger *logging.Logger) *core.TriggerScheduler {
	return core.NewTriggerScheduler(triggerConfigRepo, videoRepo, processingService, logger.Logger)
}

func provideSSEHandler(eventBus *core.EventBus, authService *core.AuthService, logger *logging.Logger) *handler.SSEHandler {
	return handler.NewSSEHandler(eventBus, authService, logger.Logger)
}

func provideAuthService(userRepo data.UserRepository, revokedRepo data.RevokedTokenRepository, cfg *config.Config, logger *logging.Logger) *core.AuthService {
	return core.NewAuthService(userRepo, revokedRepo, cfg.Auth.PasetoSecret, cfg.Auth.TokenDuration, logger.Logger)
}

func provideUserService(userRepo data.UserRepository, logger *logging.Logger) *core.UserService {
	return core.NewUserService(userRepo, logger.Logger)
}

func provideVideoHandler(service *core.VideoService, processingService *core.VideoProcessingService, tagService *core.TagService, searchService *core.SearchService) *handler.VideoHandler {
	return handler.NewVideoHandler(service, processingService, tagService, searchService)
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

func provideTagRepository(db *gorm.DB) data.TagRepository {
	return data.NewTagRepository(db)
}

func provideTagService(tagRepo data.TagRepository, videoRepo data.VideoRepository, logger *logging.Logger) *core.TagService {
	return core.NewTagService(tagRepo, videoRepo, logger.Logger)
}

func provideTagHandler(tagService *core.TagService) *handler.TagHandler {
	return handler.NewTagHandler(tagService)
}

func provideInteractionRepository(db *gorm.DB) data.InteractionRepository {
	return data.NewInteractionRepository(db)
}

func provideMeilisearchClient(cfg *config.Config, logger *logging.Logger) (*meilisearch.Client, error) {
	client, err := meilisearch.NewClient(
		cfg.Meilisearch.Host,
		cfg.Meilisearch.APIKey,
		cfg.Meilisearch.IndexName,
		logger.Logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to meilisearch: %w", err)
	}

	return client, nil
}

func provideSearchService(meiliClient *meilisearch.Client, videoRepo data.VideoRepository, interactionRepo data.InteractionRepository, tagRepo data.TagRepository, logger *logging.Logger) *core.SearchService {
	return core.NewSearchService(meiliClient, videoRepo, interactionRepo, tagRepo, logger.Logger)
}

func provideInteractionService(repo data.InteractionRepository, logger *logging.Logger) *core.InteractionService {
	return core.NewInteractionService(repo, logger.Logger)
}

func provideInteractionHandler(service *core.InteractionService) *handler.InteractionHandler {
	return handler.NewInteractionHandler(service)
}

func provideSearchHandler(searchService *core.SearchService) *handler.SearchHandler {
	return handler.NewSearchHandler(searchService)
}

func provideWatchHistoryRepository(db *gorm.DB) data.WatchHistoryRepository {
	return data.NewWatchHistoryRepository(db)
}

func provideWatchHistoryService(repo data.WatchHistoryRepository, videoRepo data.VideoRepository, logger *logging.Logger) *core.WatchHistoryService {
	return core.NewWatchHistoryService(repo, videoRepo, logger.Logger)
}

func provideWatchHistoryHandler(service *core.WatchHistoryService) *handler.WatchHistoryHandler {
	return handler.NewWatchHistoryHandler(service)
}

func provideRouter(logger *logging.Logger, cfg *config.Config, videoHandler *handler.VideoHandler, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, adminHandler *handler.AdminHandler, jobHandler *handler.JobHandler, sseHandler *handler.SSEHandler, tagHandler *handler.TagHandler, interactionHandler *handler.InteractionHandler, searchHandler *handler.SearchHandler, watchHistoryHandler *handler.WatchHistoryHandler, storagePathHandler *handler.StoragePathHandler, scanHandler *handler.ScanHandler, authService *core.AuthService, rbacService *core.RBACService, rateLimiter *middleware.IPRateLimiter) *gin.Engine {
	return api.NewRouter(logger, cfg, videoHandler, authHandler, settingsHandler, adminHandler, jobHandler, sseHandler, tagHandler, interactionHandler, searchHandler, watchHistoryHandler, storagePathHandler, scanHandler, authService, rbacService, rateLimiter)
}

func provideServer(router *gin.Engine, logger *logging.Logger, cfg *config.Config, processingService *core.VideoProcessingService, userService *core.UserService, jobHistoryService *core.JobHistoryService, triggerScheduler *core.TriggerScheduler, videoService *core.VideoService, tagService *core.TagService, searchService *core.SearchService, scanService *core.ScanService, retryScheduler *core.RetryScheduler, dlqService *core.DLQService) *server.Server {
	return server.NewHTTPServer(router, logger, cfg, processingService, userService, jobHistoryService, triggerScheduler, videoService, tagService, searchService, scanService, retryScheduler, dlqService)
}

func provideStoragePathRepository(db *gorm.DB) data.StoragePathRepository {
	return data.NewStoragePathRepository(db)
}

func provideStoragePathService(repo data.StoragePathRepository, logger *logging.Logger) *core.StoragePathService {
	return core.NewStoragePathService(repo, logger.Logger)
}

func provideStoragePathHandler(service *core.StoragePathService) *handler.StoragePathHandler {
	return handler.NewStoragePathHandler(service)
}

func provideScanHistoryRepository(db *gorm.DB) data.ScanHistoryRepository {
	return data.NewScanHistoryRepository(db)
}

func provideScanService(storagePathService *core.StoragePathService, videoRepo data.VideoRepository, scanHistoryRepo data.ScanHistoryRepository, processingService *core.VideoProcessingService, eventBus *core.EventBus, logger *logging.Logger) *core.ScanService {
	return core.NewScanService(storagePathService, videoRepo, scanHistoryRepo, processingService, eventBus, logger.Logger)
}

func provideScanHandler(scanService *core.ScanService) *handler.ScanHandler {
	return handler.NewScanHandler(scanService)
}

func provideDLQRepository(db *gorm.DB) data.DLQRepository {
	return data.NewDLQRepository(db)
}

func provideRetryConfigRepository(db *gorm.DB) data.RetryConfigRepository {
	return data.NewRetryConfigRepository(db)
}

func provideRetryScheduler(jobHistoryRepo data.JobHistoryRepository, dlqRepo data.DLQRepository, retryConfigRepo data.RetryConfigRepository, videoRepo data.VideoRepository, eventBus *core.EventBus, logger *logging.Logger) *core.RetryScheduler {
	return core.NewRetryScheduler(jobHistoryRepo, dlqRepo, retryConfigRepo, videoRepo, eventBus, logger.Logger)
}

func provideDLQService(dlqRepo data.DLQRepository, jobHistoryRepo data.JobHistoryRepository, videoRepo data.VideoRepository, eventBus *core.EventBus, logger *logging.Logger) *core.DLQService {
	return core.NewDLQService(dlqRepo, jobHistoryRepo, videoRepo, eventBus, logger.Logger)
}
