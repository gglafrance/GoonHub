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

// InitializeServer creates a fully wired server instance
func InitializeServer(cfgPath string) (*server.Server, error) {
	wire.Build(
		// ============================================================
		// CONFIGURATION & INFRASTRUCTURE
		// ============================================================
		config.Load,
		logging.New,
		postgres.NewDB,

		// ============================================================
		// DATA LAYER - REPOSITORIES
		// ============================================================

		// User & Auth Repositories
		provideUserRepository,
		provideRevokedTokenRepository,
		provideUserSettingsRepository,
		provideRoleRepository,
		providePermissionRepository,

		// Video Repositories
		provideVideoRepository,
		provideTagRepository,
		provideActorRepository,
		provideInteractionRepository,
		provideActorInteractionRepository,
		provideWatchHistoryRepository,

		// Job & Processing Repositories
		provideJobHistoryRepository,
		providePoolConfigRepository,
		provideProcessingConfigRepository,
		provideTriggerConfigRepository,
		provideDLQRepository,
		provideRetryConfigRepository,

		// Storage & Scan Repositories
		provideStoragePathRepository,
		provideScanHistoryRepository,
		provideExplorerRepository,

		// ============================================================
		// EXTERNAL SERVICES
		// ============================================================
		provideMeilisearchClient,

		// ============================================================
		// CORE SERVICES
		// ============================================================

		// Event Bus (used by many services)
		provideEventBus,

		// Auth & User Services
		provideAuthService,
		provideUserService,
		provideSettingsService,
		provideRBACService,
		provideAdminService,

		// Video & Content Services
		provideVideoService,
		provideTagService,
		provideActorService,
		provideInteractionService,
		provideActorInteractionService,
		provideSearchService,
		provideWatchHistoryService,

		// Processing & Job Services
		provideVideoProcessingService,
		provideJobHistoryService,
		provideTriggerScheduler,
		provideRetryScheduler,
		provideDLQService,

		// Storage & Scan Services
		provideStoragePathService,
		provideScanService,
		provideExplorerService,

		// External API Services
		providePornDBService,

		// ============================================================
		// API LAYER - MIDDLEWARE
		// ============================================================
		provideRateLimiter,

		// ============================================================
		// API LAYER - HANDLERS
		// ============================================================

		// Auth & User Handlers
		provideAuthHandler,
		provideAdminHandler,
		provideSettingsHandler,

		// Video & Content Handlers
		provideVideoHandler,
		provideTagHandler,
		provideActorHandler,
		provideInteractionHandler,
		provideActorInteractionHandler,
		provideSearchHandler,
		provideWatchHistoryHandler,

		// Job & Processing Handlers
		provideJobHandler,
		providePoolConfigHandler,
		provideProcessingConfigHandler,
		provideTriggerConfigHandler,
		provideDLQHandler,
		provideRetryConfigHandler,

		// Real-time & Storage Handlers
		provideSSEHandler,
		provideStoragePathHandler,
		provideScanHandler,
		provideExplorerHandler,

		// External API Handlers
		providePornDBHandler,

		// ============================================================
		// ROUTER & SERVER
		// ============================================================
		provideRouter,
		provideServer,
	)
	return &server.Server{}, nil
}

// ============================================================================
// DATA LAYER PROVIDERS - Repositories
// ============================================================================

// --- User & Auth Repositories ---

func provideUserRepository(db *gorm.DB) data.UserRepository {
	return data.NewUserRepository(db)
}

func provideRevokedTokenRepository(db *gorm.DB) data.RevokedTokenRepository {
	return data.NewRevokedTokenRepository(db)
}

func provideUserSettingsRepository(db *gorm.DB) data.UserSettingsRepository {
	return data.NewUserSettingsRepository(db)
}

func provideRoleRepository(db *gorm.DB) data.RoleRepository {
	return data.NewRoleRepository(db)
}

func providePermissionRepository(db *gorm.DB) data.PermissionRepository {
	return data.NewPermissionRepository(db)
}

// --- Video Repositories ---

func provideVideoRepository(db *gorm.DB) data.VideoRepository {
	return data.NewVideoRepository(db)
}

func provideTagRepository(db *gorm.DB) data.TagRepository {
	return data.NewTagRepository(db)
}

func provideActorRepository(db *gorm.DB) data.ActorRepository {
	return data.NewActorRepository(db)
}

func provideInteractionRepository(db *gorm.DB) data.InteractionRepository {
	return data.NewInteractionRepository(db)
}

func provideActorInteractionRepository(db *gorm.DB) data.ActorInteractionRepository {
	return data.NewActorInteractionRepository(db)
}

func provideWatchHistoryRepository(db *gorm.DB) data.WatchHistoryRepository {
	return data.NewWatchHistoryRepository(db)
}

// --- Job & Processing Repositories ---

func provideJobHistoryRepository(db *gorm.DB) data.JobHistoryRepository {
	return data.NewJobHistoryRepository(db)
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

func provideDLQRepository(db *gorm.DB) data.DLQRepository {
	return data.NewDLQRepository(db)
}

func provideRetryConfigRepository(db *gorm.DB) data.RetryConfigRepository {
	return data.NewRetryConfigRepository(db)
}

// --- Storage & Scan Repositories ---

func provideStoragePathRepository(db *gorm.DB) data.StoragePathRepository {
	return data.NewStoragePathRepository(db)
}

func provideScanHistoryRepository(db *gorm.DB) data.ScanHistoryRepository {
	return data.NewScanHistoryRepository(db)
}

func provideExplorerRepository(db *gorm.DB) data.ExplorerRepository {
	return data.NewExplorerRepository(db)
}

// ============================================================================
// EXTERNAL SERVICE PROVIDERS
// ============================================================================

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

// ============================================================================
// CORE SERVICE PROVIDERS
// ============================================================================

// --- Event Bus ---

func provideEventBus(logger *logging.Logger) *core.EventBus {
	return core.NewEventBus(logger.Logger)
}

// --- Auth & User Services ---

func provideAuthService(userRepo data.UserRepository, revokedRepo data.RevokedTokenRepository, cfg *config.Config, logger *logging.Logger) (*core.AuthService, error) {
	return core.NewAuthService(
		userRepo, revokedRepo,
		cfg.Auth.PasetoSecret, cfg.Auth.TokenDuration,
		cfg.Auth.LockoutThreshold, cfg.Auth.LockoutDuration,
		logger.Logger,
	)
}

func provideUserService(userRepo data.UserRepository, logger *logging.Logger) *core.UserService {
	return core.NewUserService(userRepo, logger.Logger)
}

func provideSettingsService(settingsRepo data.UserSettingsRepository, userRepo data.UserRepository, logger *logging.Logger) *core.SettingsService {
	return core.NewSettingsService(settingsRepo, userRepo, logger.Logger)
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

// --- Video & Content Services ---

func provideVideoService(repo data.VideoRepository, cfg *config.Config, processingService *core.VideoProcessingService, eventBus *core.EventBus, logger *logging.Logger) *core.VideoService {
	return core.NewVideoService(repo, cfg.Processing.VideoDir, cfg.Processing.MetadataDir, processingService, eventBus, logger.Logger)
}

func provideTagService(tagRepo data.TagRepository, videoRepo data.VideoRepository, logger *logging.Logger) *core.TagService {
	return core.NewTagService(tagRepo, videoRepo, logger.Logger)
}

func provideActorService(actorRepo data.ActorRepository, videoRepo data.VideoRepository, logger *logging.Logger) *core.ActorService {
	return core.NewActorService(actorRepo, videoRepo, logger.Logger)
}

func provideInteractionService(repo data.InteractionRepository, logger *logging.Logger) *core.InteractionService {
	return core.NewInteractionService(repo, logger.Logger)
}

func provideActorInteractionService(repo data.ActorInteractionRepository, logger *logging.Logger) *core.ActorInteractionService {
	return core.NewActorInteractionService(repo, logger.Logger)
}

func provideSearchService(meiliClient *meilisearch.Client, videoRepo data.VideoRepository, interactionRepo data.InteractionRepository, tagRepo data.TagRepository, logger *logging.Logger) *core.SearchService {
	return core.NewSearchService(meiliClient, videoRepo, interactionRepo, tagRepo, logger.Logger)
}

func provideWatchHistoryService(repo data.WatchHistoryRepository, videoRepo data.VideoRepository, logger *logging.Logger) *core.WatchHistoryService {
	return core.NewWatchHistoryService(repo, videoRepo, logger.Logger)
}

// --- Processing & Job Services ---

func provideVideoProcessingService(repo data.VideoRepository, cfg *config.Config, logger *logging.Logger, eventBus *core.EventBus, jobHistory *core.JobHistoryService, poolConfigRepo data.PoolConfigRepository, processingConfigRepo data.ProcessingConfigRepository, triggerConfigRepo data.TriggerConfigRepository) *core.VideoProcessingService {
	return core.NewVideoProcessingService(repo, cfg.Processing, logger.Logger, eventBus, jobHistory, poolConfigRepo, processingConfigRepo, triggerConfigRepo)
}

func provideJobHistoryService(repo data.JobHistoryRepository, cfg *config.Config, logger *logging.Logger) *core.JobHistoryService {
	return core.NewJobHistoryService(repo, cfg.Processing, logger.Logger)
}

func provideTriggerScheduler(triggerConfigRepo data.TriggerConfigRepository, videoRepo data.VideoRepository, processingService *core.VideoProcessingService, logger *logging.Logger) *core.TriggerScheduler {
	return core.NewTriggerScheduler(triggerConfigRepo, videoRepo, processingService, logger.Logger)
}

func provideRetryScheduler(jobHistoryRepo data.JobHistoryRepository, dlqRepo data.DLQRepository, retryConfigRepo data.RetryConfigRepository, videoRepo data.VideoRepository, eventBus *core.EventBus, logger *logging.Logger) *core.RetryScheduler {
	return core.NewRetryScheduler(jobHistoryRepo, dlqRepo, retryConfigRepo, videoRepo, eventBus, logger.Logger)
}

func provideDLQService(dlqRepo data.DLQRepository, jobHistoryRepo data.JobHistoryRepository, videoRepo data.VideoRepository, eventBus *core.EventBus, logger *logging.Logger) *core.DLQService {
	return core.NewDLQService(dlqRepo, jobHistoryRepo, videoRepo, eventBus, logger.Logger)
}

// --- Storage & Scan Services ---

func provideStoragePathService(repo data.StoragePathRepository, logger *logging.Logger) *core.StoragePathService {
	return core.NewStoragePathService(repo, logger.Logger)
}

func provideScanService(storagePathService *core.StoragePathService, videoRepo data.VideoRepository, scanHistoryRepo data.ScanHistoryRepository, processingService *core.VideoProcessingService, eventBus *core.EventBus, logger *logging.Logger) *core.ScanService {
	return core.NewScanService(storagePathService, videoRepo, scanHistoryRepo, processingService, eventBus, logger.Logger)
}

func provideExplorerService(explorerRepo data.ExplorerRepository, storagePathRepo data.StoragePathRepository, videoRepo data.VideoRepository, tagRepo data.TagRepository, actorRepo data.ActorRepository, eventBus *core.EventBus, logger *logging.Logger, cfg *config.Config) *core.ExplorerService {
	return core.NewExplorerService(explorerRepo, storagePathRepo, videoRepo, tagRepo, actorRepo, eventBus, logger.Logger, cfg.Processing.MetadataDir)
}

// --- External API Services ---

func providePornDBService(cfg *config.Config, logger *logging.Logger) *core.PornDBService {
	return core.NewPornDBService(cfg.PornDB.APIKey, logger.Logger)
}

// ============================================================================
// API MIDDLEWARE PROVIDERS
// ============================================================================

func provideRateLimiter(cfg *config.Config) *middleware.IPRateLimiter {
	rl := rate.Every(time.Minute / time.Duration(cfg.Auth.LoginRateLimit))
	return middleware.NewIPRateLimiter(rl, cfg.Auth.LoginRateBurst)
}

// ============================================================================
// API HANDLER PROVIDERS
// ============================================================================

// --- Auth & User Handlers ---

func provideAuthHandler(authService *core.AuthService, userService *core.UserService, cfg *config.Config) *handler.AuthHandler {
	secureCookies := cfg.Environment == "production"
	return handler.NewAuthHandlerWithConfig(authService, userService, cfg.Auth.TokenDuration, secureCookies)
}

func provideAdminHandler(adminService *core.AdminService, rbacService *core.RBACService) *handler.AdminHandler {
	return handler.NewAdminHandler(adminService, rbacService)
}

func provideSettingsHandler(settingsService *core.SettingsService) *handler.SettingsHandler {
	return handler.NewSettingsHandler(settingsService)
}

// --- Video & Content Handlers ---

func provideVideoHandler(service *core.VideoService, processingService *core.VideoProcessingService, tagService *core.TagService, searchService *core.SearchService) *handler.VideoHandler {
	return handler.NewVideoHandler(service, processingService, tagService, searchService)
}

func provideTagHandler(tagService *core.TagService) *handler.TagHandler {
	return handler.NewTagHandler(tagService)
}

func provideActorHandler(actorService *core.ActorService, cfg *config.Config) *handler.ActorHandler {
	return handler.NewActorHandler(actorService, cfg.Processing.ActorImageDir)
}

func provideInteractionHandler(service *core.InteractionService) *handler.InteractionHandler {
	return handler.NewInteractionHandler(service)
}

func provideActorInteractionHandler(service *core.ActorInteractionService, actorRepo data.ActorRepository) *handler.ActorInteractionHandler {
	return handler.NewActorInteractionHandler(service, actorRepo)
}

func provideSearchHandler(searchService *core.SearchService) *handler.SearchHandler {
	return handler.NewSearchHandler(searchService)
}

func provideWatchHistoryHandler(service *core.WatchHistoryService) *handler.WatchHistoryHandler {
	return handler.NewWatchHistoryHandler(service)
}

// --- Job & Processing Handlers ---

func provideJobHandler(jobHistoryService *core.JobHistoryService, processingService *core.VideoProcessingService) *handler.JobHandler {
	return handler.NewJobHandler(jobHistoryService, processingService)
}

func providePoolConfigHandler(processingService *core.VideoProcessingService, poolConfigRepo data.PoolConfigRepository) *handler.PoolConfigHandler {
	return handler.NewPoolConfigHandler(processingService, poolConfigRepo)
}

func provideProcessingConfigHandler(processingService *core.VideoProcessingService, processingConfigRepo data.ProcessingConfigRepository) *handler.ProcessingConfigHandler {
	return handler.NewProcessingConfigHandler(processingService, processingConfigRepo)
}

func provideTriggerConfigHandler(triggerConfigRepo data.TriggerConfigRepository, processingService *core.VideoProcessingService, triggerScheduler *core.TriggerScheduler) *handler.TriggerConfigHandler {
	return handler.NewTriggerConfigHandler(triggerConfigRepo, processingService, triggerScheduler)
}

func provideDLQHandler(dlqService *core.DLQService) *handler.DLQHandler {
	return handler.NewDLQHandler(dlqService)
}

func provideRetryConfigHandler(retryConfigRepo data.RetryConfigRepository, retryScheduler *core.RetryScheduler) *handler.RetryConfigHandler {
	return handler.NewRetryConfigHandler(retryConfigRepo, retryScheduler)
}

// --- Real-time & Storage Handlers ---

func provideSSEHandler(eventBus *core.EventBus, authService *core.AuthService, logger *logging.Logger) *handler.SSEHandler {
	return handler.NewSSEHandler(eventBus, authService, logger.Logger)
}

func provideStoragePathHandler(service *core.StoragePathService) *handler.StoragePathHandler {
	return handler.NewStoragePathHandler(service)
}

func provideScanHandler(scanService *core.ScanService) *handler.ScanHandler {
	return handler.NewScanHandler(scanService)
}

func provideExplorerHandler(explorerService *core.ExplorerService) *handler.ExplorerHandler {
	return handler.NewExplorerHandler(explorerService)
}

// --- External API Handlers ---

func providePornDBHandler(pornDBService *core.PornDBService) *handler.PornDBHandler {
	return handler.NewPornDBHandler(pornDBService)
}

// ============================================================================
// ROUTER & SERVER PROVIDERS
// ============================================================================

func provideRouter(
	logger *logging.Logger,
	cfg *config.Config,
	videoHandler *handler.VideoHandler,
	authHandler *handler.AuthHandler,
	settingsHandler *handler.SettingsHandler,
	adminHandler *handler.AdminHandler,
	jobHandler *handler.JobHandler,
	poolConfigHandler *handler.PoolConfigHandler,
	processingConfigHandler *handler.ProcessingConfigHandler,
	triggerConfigHandler *handler.TriggerConfigHandler,
	dlqHandler *handler.DLQHandler,
	retryConfigHandler *handler.RetryConfigHandler,
	sseHandler *handler.SSEHandler,
	tagHandler *handler.TagHandler,
	actorHandler *handler.ActorHandler,
	interactionHandler *handler.InteractionHandler,
	actorInteractionHandler *handler.ActorInteractionHandler,
	searchHandler *handler.SearchHandler,
	watchHistoryHandler *handler.WatchHistoryHandler,
	storagePathHandler *handler.StoragePathHandler,
	scanHandler *handler.ScanHandler,
	explorerHandler *handler.ExplorerHandler,
	pornDBHandler *handler.PornDBHandler,
	authService *core.AuthService,
	rbacService *core.RBACService,
	rateLimiter *middleware.IPRateLimiter,
) *gin.Engine {
	return api.NewRouter(
		logger, cfg,
		videoHandler, authHandler, settingsHandler, adminHandler,
		jobHandler, poolConfigHandler, processingConfigHandler, triggerConfigHandler,
		dlqHandler, retryConfigHandler, sseHandler, tagHandler, actorHandler, interactionHandler,
		actorInteractionHandler, searchHandler, watchHistoryHandler, storagePathHandler, scanHandler,
		explorerHandler, pornDBHandler, authService, rbacService, rateLimiter,
	)
}

func provideServer(
	router *gin.Engine,
	logger *logging.Logger,
	cfg *config.Config,
	processingService *core.VideoProcessingService,
	userService *core.UserService,
	jobHistoryService *core.JobHistoryService,
	triggerScheduler *core.TriggerScheduler,
	videoService *core.VideoService,
	tagService *core.TagService,
	searchService *core.SearchService,
	scanService *core.ScanService,
	explorerService *core.ExplorerService,
	retryScheduler *core.RetryScheduler,
	dlqService *core.DLQService,
) *server.Server {
	return server.NewHTTPServer(
		router, logger, cfg,
		processingService, userService, jobHistoryService, triggerScheduler,
		videoService, tagService, searchService, scanService, explorerService, retryScheduler, dlqService,
	)
}
