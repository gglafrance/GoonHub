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
	"goonhub/internal/streaming"

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

		// Scene Repositories
		provideSceneRepository,
		provideTagRepository,
		provideActorRepository,
		provideStudioRepository,
		provideInteractionRepository,
		provideActorInteractionRepository,
		provideStudioInteractionRepository,
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

		// Search Config Repository
		provideSearchConfigRepository,

		// App Settings Repository
		provideAppSettingsRepository,

		// Saved Search Repository
		provideSavedSearchRepository,

		// Marker Repository
		provideMarkerRepository,

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

		// Scene & Content Services
		provideSceneService,
		provideTagService,
		provideActorService,
		provideStudioService,
		provideInteractionService,
		provideActorInteractionService,
		provideStudioInteractionService,
		provideSearchService,
		provideWatchHistoryService,
		provideRelatedScenesService,

		// Processing & Job Services
		provideSceneProcessingService,
		provideJobHistoryService,
		provideJobStatusService,
		provideJobQueueFeeder,
		provideTriggerScheduler,
		provideRetryScheduler,
		provideDLQService,

		// Storage & Scan Services
		provideStoragePathService,
		provideScanService,
		provideExplorerService,

		// External API Services
		providePornDBService,

		// Saved Search Service
		provideSavedSearchService,

		// Homepage Service
		provideHomepageService,

		// Marker Service
		provideMarkerService,

		// Streaming Manager
		provideStreamManager,

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

		// Scene & Content Handlers
		provideSceneHandler,
		provideTagHandler,
		provideActorHandler,
		provideStudioHandler,
		provideInteractionHandler,
		provideActorInteractionHandler,
		provideStudioInteractionHandler,
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

		// Saved Search Handler
		provideSavedSearchHandler,

		// Homepage Handler
		provideHomepageHandler,

		// Marker Handler
		provideMarkerHandler,

		// Import Handler
		provideImportHandler,

		// Stream Stats Handler
		provideStreamStatsHandler,

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

// --- Scene Repositories ---

func provideSceneRepository(db *gorm.DB) data.SceneRepository {
	return data.NewSceneRepository(db)
}

func provideTagRepository(db *gorm.DB) data.TagRepository {
	return data.NewTagRepository(db)
}

func provideActorRepository(db *gorm.DB) data.ActorRepository {
	return data.NewActorRepository(db)
}

func provideStudioRepository(db *gorm.DB) data.StudioRepository {
	return data.NewStudioRepository(db)
}

func provideInteractionRepository(db *gorm.DB) data.InteractionRepository {
	return data.NewInteractionRepository(db)
}

func provideActorInteractionRepository(db *gorm.DB) data.ActorInteractionRepository {
	return data.NewActorInteractionRepository(db)
}

func provideStudioInteractionRepository(db *gorm.DB) data.StudioInteractionRepository {
	return data.NewStudioInteractionRepository(db)
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

func provideSearchConfigRepository(db *gorm.DB) data.SearchConfigRepository {
	return data.NewSearchConfigRepository(db)
}

func provideAppSettingsRepository(db *gorm.DB) data.AppSettingsRepository {
	return data.NewAppSettingsRepository(db)
}

func provideSavedSearchRepository(db *gorm.DB) data.SavedSearchRepository {
	return data.NewSavedSearchRepository(db)
}

func provideMarkerRepository(db *gorm.DB) data.MarkerRepository {
	return data.NewMarkerRepository(db)
}

// ============================================================================
// EXTERNAL SERVICE PROVIDERS
// ============================================================================

func provideMeilisearchClient(cfg *config.Config, searchConfigRepo data.SearchConfigRepository, logger *logging.Logger) (*meilisearch.Client, error) {
	var maxTotalHits int64 = 100000
	record, err := searchConfigRepo.Get()
	if err != nil {
		logger.Warn(fmt.Sprintf("failed to read search config from DB, using default maxTotalHits: %v", err))
	} else if record != nil {
		maxTotalHits = record.MaxTotalHits
	}

	client, err := meilisearch.NewClient(
		cfg.Meilisearch.Host,
		cfg.Meilisearch.APIKey,
		cfg.Meilisearch.IndexName,
		maxTotalHits,
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

// --- Scene & Content Services ---

func provideSceneService(repo data.SceneRepository, cfg *config.Config, processingService *core.SceneProcessingService, eventBus *core.EventBus, logger *logging.Logger, jobHistoryRepo data.JobHistoryRepository, dlqRepo data.DLQRepository, appSettingsRepo data.AppSettingsRepository) *core.SceneService {
	return core.NewSceneService(repo, cfg.Processing.VideoDir, cfg.Processing.MetadataDir, processingService, eventBus, logger.Logger, jobHistoryRepo, dlqRepo, appSettingsRepo)
}

func provideTagService(tagRepo data.TagRepository, sceneRepo data.SceneRepository, logger *logging.Logger) *core.TagService {
	return core.NewTagService(tagRepo, sceneRepo, logger.Logger)
}

func provideActorService(actorRepo data.ActorRepository, sceneRepo data.SceneRepository, logger *logging.Logger) *core.ActorService {
	return core.NewActorService(actorRepo, sceneRepo, logger.Logger)
}

func provideStudioService(studioRepo data.StudioRepository, sceneRepo data.SceneRepository, logger *logging.Logger) *core.StudioService {
	return core.NewStudioService(studioRepo, sceneRepo, logger.Logger)
}

func provideInteractionService(repo data.InteractionRepository, logger *logging.Logger) *core.InteractionService {
	return core.NewInteractionService(repo, logger.Logger)
}

func provideActorInteractionService(repo data.ActorInteractionRepository, logger *logging.Logger) *core.ActorInteractionService {
	return core.NewActorInteractionService(repo, logger.Logger)
}

func provideStudioInteractionService(repo data.StudioInteractionRepository, logger *logging.Logger) *core.StudioInteractionService {
	return core.NewStudioInteractionService(repo, logger.Logger)
}

func provideSearchService(meiliClient *meilisearch.Client, sceneRepo data.SceneRepository, interactionRepo data.InteractionRepository, tagRepo data.TagRepository, actorRepo data.ActorRepository, markerRepo data.MarkerRepository, logger *logging.Logger) *core.SearchService {
	return core.NewSearchService(meiliClient, sceneRepo, interactionRepo, tagRepo, actorRepo, markerRepo, logger.Logger)
}

func provideWatchHistoryService(repo data.WatchHistoryRepository, sceneRepo data.SceneRepository, searchService *core.SearchService, logger *logging.Logger) *core.WatchHistoryService {
	return core.NewWatchHistoryService(repo, sceneRepo, searchService, logger.Logger)
}

func provideRelatedScenesService(sceneRepo data.SceneRepository, tagRepo data.TagRepository, actorRepo data.ActorRepository, studioRepo data.StudioRepository, logger *logging.Logger) *core.RelatedScenesService {
	return core.NewRelatedScenesService(sceneRepo, tagRepo, actorRepo, studioRepo, logger.Logger)
}

// --- Processing & Job Services ---

func provideSceneProcessingService(repo data.SceneRepository, markerService *core.MarkerService, cfg *config.Config, logger *logging.Logger, eventBus *core.EventBus, jobHistory *core.JobHistoryService, poolConfigRepo data.PoolConfigRepository, processingConfigRepo data.ProcessingConfigRepository, triggerConfigRepo data.TriggerConfigRepository) *core.SceneProcessingService {
	return core.NewSceneProcessingService(repo, markerService, cfg.Processing, logger.Logger, eventBus, jobHistory, poolConfigRepo, processingConfigRepo, triggerConfigRepo)
}

func provideJobHistoryService(repo data.JobHistoryRepository, cfg *config.Config, logger *logging.Logger) *core.JobHistoryService {
	return core.NewJobHistoryService(repo, cfg.Processing, logger.Logger)
}

func provideJobStatusService(jobHistoryService *core.JobHistoryService, processingService *core.SceneProcessingService, logger *logging.Logger) *core.JobStatusService {
	return core.NewJobStatusService(jobHistoryService, processingService, logger.Logger)
}

func provideJobQueueFeeder(jobHistoryRepo data.JobHistoryRepository, sceneRepo data.SceneRepository, markerService *core.MarkerService, processingService *core.SceneProcessingService, logger *logging.Logger) *core.JobQueueFeeder {
	return core.NewJobQueueFeeder(jobHistoryRepo, sceneRepo, markerService, markerService, processingService.GetPoolManager(), logger.Logger)
}

func provideTriggerScheduler(triggerConfigRepo data.TriggerConfigRepository, sceneRepo data.SceneRepository, processingService *core.SceneProcessingService, logger *logging.Logger) *core.TriggerScheduler {
	return core.NewTriggerScheduler(triggerConfigRepo, sceneRepo, processingService, logger.Logger)
}

func provideRetryScheduler(jobHistoryRepo data.JobHistoryRepository, dlqRepo data.DLQRepository, retryConfigRepo data.RetryConfigRepository, sceneRepo data.SceneRepository, eventBus *core.EventBus, logger *logging.Logger) *core.RetryScheduler {
	return core.NewRetryScheduler(jobHistoryRepo, dlqRepo, retryConfigRepo, sceneRepo, eventBus, logger.Logger)
}

func provideDLQService(dlqRepo data.DLQRepository, jobHistoryRepo data.JobHistoryRepository, sceneRepo data.SceneRepository, eventBus *core.EventBus, logger *logging.Logger) *core.DLQService {
	return core.NewDLQService(dlqRepo, jobHistoryRepo, sceneRepo, eventBus, logger.Logger)
}

// --- Storage & Scan Services ---

func provideStoragePathService(repo data.StoragePathRepository, logger *logging.Logger) *core.StoragePathService {
	return core.NewStoragePathService(repo, logger.Logger)
}

func provideScanService(storagePathService *core.StoragePathService, sceneRepo data.SceneRepository, scanHistoryRepo data.ScanHistoryRepository, processingService *core.SceneProcessingService, eventBus *core.EventBus, logger *logging.Logger) *core.ScanService {
	return core.NewScanService(storagePathService, sceneRepo, scanHistoryRepo, processingService, eventBus, logger.Logger)
}

func provideExplorerService(explorerRepo data.ExplorerRepository, storagePathRepo data.StoragePathRepository, sceneRepo data.SceneRepository, tagRepo data.TagRepository, actorRepo data.ActorRepository, jobHistoryRepo data.JobHistoryRepository, eventBus *core.EventBus, logger *logging.Logger, cfg *config.Config) *core.ExplorerService {
	return core.NewExplorerService(explorerRepo, storagePathRepo, sceneRepo, tagRepo, actorRepo, jobHistoryRepo, eventBus, logger.Logger, cfg.Processing.MetadataDir)
}

// --- External API Services ---

func providePornDBService(cfg *config.Config, logger *logging.Logger) *core.PornDBService {
	return core.NewPornDBService(cfg.PornDB.APIKey, logger.Logger)
}

func provideSavedSearchService(repo data.SavedSearchRepository, logger *logging.Logger) *core.SavedSearchService {
	return core.NewSavedSearchService(repo, logger.Logger)
}

func provideHomepageService(
	settingsService *core.SettingsService,
	searchService *core.SearchService,
	savedSearchService *core.SavedSearchService,
	watchHistoryRepo data.WatchHistoryRepository,
	interactionRepo data.InteractionRepository,
	sceneRepo data.SceneRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	studioRepo data.StudioRepository,
	logger *logging.Logger,
) *core.HomepageService {
	return core.NewHomepageService(
		settingsService,
		searchService,
		savedSearchService,
		watchHistoryRepo,
		interactionRepo,
		sceneRepo,
		tagRepo,
		actorRepo,
		studioRepo,
		logger.Logger,
	)
}

func provideMarkerService(markerRepo data.MarkerRepository, sceneRepo data.SceneRepository, tagRepo data.TagRepository, cfg *config.Config, logger *logging.Logger) *core.MarkerService {
	return core.NewMarkerService(markerRepo, sceneRepo, tagRepo, cfg, logger.Logger)
}

// --- Streaming Manager ---

func provideStreamManager(cfg *config.Config, sceneRepo data.SceneRepository, logger *logging.Logger) *streaming.Manager {
	return streaming.NewManager(&cfg.Streaming, sceneRepo, logger.Logger)
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
	if cfg.Server.SecureCookies != nil {
		secureCookies = *cfg.Server.SecureCookies
	}
	return handler.NewAuthHandlerWithConfig(authService, userService, cfg.Auth.TokenDuration, secureCookies)
}

func provideAdminHandler(adminService *core.AdminService, rbacService *core.RBACService, sceneService *core.SceneService) *handler.AdminHandler {
	return handler.NewAdminHandler(adminService, rbacService, sceneService)
}

func provideSettingsHandler(settingsService *core.SettingsService) *handler.SettingsHandler {
	return handler.NewSettingsHandler(settingsService)
}

// --- Scene & Content Handlers ---

func provideSceneHandler(service *core.SceneService, processingService *core.SceneProcessingService, tagService *core.TagService, searchService *core.SearchService, relatedScenesService *core.RelatedScenesService, markerService *core.MarkerService, streamManager *streaming.Manager) *handler.SceneHandler {
	return handler.NewSceneHandler(service, processingService, tagService, searchService, relatedScenesService, markerService, streamManager)
}

func provideTagHandler(tagService *core.TagService) *handler.TagHandler {
	return handler.NewTagHandler(tagService)
}

func provideActorHandler(actorService *core.ActorService, cfg *config.Config) *handler.ActorHandler {
	return handler.NewActorHandler(actorService, cfg.Processing.ActorImageDir)
}

func provideStudioHandler(studioService *core.StudioService, cfg *config.Config) *handler.StudioHandler {
	return handler.NewStudioHandler(studioService, cfg.Processing.StudioLogoDir)
}

func provideInteractionHandler(service *core.InteractionService) *handler.InteractionHandler {
	return handler.NewInteractionHandler(service)
}

func provideActorInteractionHandler(service *core.ActorInteractionService, actorRepo data.ActorRepository) *handler.ActorInteractionHandler {
	return handler.NewActorInteractionHandler(service, actorRepo)
}

func provideStudioInteractionHandler(service *core.StudioInteractionService, studioRepo data.StudioRepository) *handler.StudioInteractionHandler {
	return handler.NewStudioInteractionHandler(service, studioRepo)
}

func provideSearchHandler(searchService *core.SearchService, searchConfigRepo data.SearchConfigRepository) *handler.SearchHandler {
	return handler.NewSearchHandler(searchService, searchConfigRepo)
}

func provideWatchHistoryHandler(service *core.WatchHistoryService) *handler.WatchHistoryHandler {
	return handler.NewWatchHistoryHandler(service)
}

// --- Job & Processing Handlers ---

func provideJobHandler(jobHistoryService *core.JobHistoryService, processingService *core.SceneProcessingService) *handler.JobHandler {
	return handler.NewJobHandler(jobHistoryService, processingService)
}

func providePoolConfigHandler(processingService *core.SceneProcessingService, poolConfigRepo data.PoolConfigRepository) *handler.PoolConfigHandler {
	return handler.NewPoolConfigHandler(processingService, poolConfigRepo)
}

func provideProcessingConfigHandler(processingService *core.SceneProcessingService, processingConfigRepo data.ProcessingConfigRepository) *handler.ProcessingConfigHandler {
	return handler.NewProcessingConfigHandler(processingService, processingConfigRepo)
}

func provideTriggerConfigHandler(triggerConfigRepo data.TriggerConfigRepository, processingService *core.SceneProcessingService, triggerScheduler *core.TriggerScheduler) *handler.TriggerConfigHandler {
	return handler.NewTriggerConfigHandler(triggerConfigRepo, processingService, triggerScheduler)
}

func provideDLQHandler(dlqService *core.DLQService) *handler.DLQHandler {
	return handler.NewDLQHandler(dlqService)
}

func provideRetryConfigHandler(retryConfigRepo data.RetryConfigRepository, retryScheduler *core.RetryScheduler) *handler.RetryConfigHandler {
	return handler.NewRetryConfigHandler(retryConfigRepo, retryScheduler)
}

// --- Real-time & Storage Handlers ---

func provideSSEHandler(eventBus *core.EventBus, authService *core.AuthService, jobStatusService *core.JobStatusService, logger *logging.Logger) *handler.SSEHandler {
	return handler.NewSSEHandler(eventBus, authService, jobStatusService, logger.Logger)
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

func provideSavedSearchHandler(service *core.SavedSearchService) *handler.SavedSearchHandler {
	return handler.NewSavedSearchHandler(service)
}

func provideHomepageHandler(homepageService *core.HomepageService) *handler.HomepageHandler {
	return handler.NewHomepageHandler(homepageService)
}

func provideMarkerHandler(markerService *core.MarkerService) *handler.MarkerHandler {
	return handler.NewMarkerHandler(markerService)
}

func provideImportHandler(sceneRepo data.SceneRepository, markerRepo data.MarkerRepository, logger *logging.Logger) *handler.ImportHandler {
	return handler.NewImportHandler(sceneRepo, markerRepo, logger.Logger)
}

func provideStreamStatsHandler(streamManager *streaming.Manager) *handler.StreamStatsHandler {
	return handler.NewStreamStatsHandler(streamManager)
}

// ============================================================================
// ROUTER & SERVER PROVIDERS
// ============================================================================

func provideRouter(
	logger *logging.Logger,
	cfg *config.Config,
	sceneHandler *handler.SceneHandler,
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
	studioHandler *handler.StudioHandler,
	interactionHandler *handler.InteractionHandler,
	actorInteractionHandler *handler.ActorInteractionHandler,
	studioInteractionHandler *handler.StudioInteractionHandler,
	searchHandler *handler.SearchHandler,
	watchHistoryHandler *handler.WatchHistoryHandler,
	storagePathHandler *handler.StoragePathHandler,
	scanHandler *handler.ScanHandler,
	explorerHandler *handler.ExplorerHandler,
	pornDBHandler *handler.PornDBHandler,
	savedSearchHandler *handler.SavedSearchHandler,
	homepageHandler *handler.HomepageHandler,
	markerHandler *handler.MarkerHandler,
	importHandler *handler.ImportHandler,
	streamStatsHandler *handler.StreamStatsHandler,
	authService *core.AuthService,
	rbacService *core.RBACService,
	rateLimiter *middleware.IPRateLimiter,
) *gin.Engine {
	return api.NewRouter(
		logger, cfg,
		sceneHandler, authHandler, settingsHandler, adminHandler,
		jobHandler, poolConfigHandler, processingConfigHandler, triggerConfigHandler,
		dlqHandler, retryConfigHandler, sseHandler, tagHandler, actorHandler, studioHandler, interactionHandler,
		actorInteractionHandler, studioInteractionHandler, searchHandler, watchHistoryHandler, storagePathHandler, scanHandler,
		explorerHandler, pornDBHandler, savedSearchHandler, homepageHandler, markerHandler, importHandler, streamStatsHandler, authService, rbacService, rateLimiter,
	)
}

func provideServer(
	router *gin.Engine,
	logger *logging.Logger,
	cfg *config.Config,
	processingService *core.SceneProcessingService,
	userService *core.UserService,
	jobHistoryService *core.JobHistoryService,
	jobHistoryRepo data.JobHistoryRepository,
	jobQueueFeeder *core.JobQueueFeeder,
	triggerScheduler *core.TriggerScheduler,
	sceneService *core.SceneService,
	tagService *core.TagService,
	searchService *core.SearchService,
	scanService *core.ScanService,
	explorerService *core.ExplorerService,
	retryScheduler *core.RetryScheduler,
	dlqService *core.DLQService,
	actorService *core.ActorService,
	studioService *core.StudioService,
) *server.Server {
	return server.NewHTTPServer(
		router, logger, cfg,
		processingService, userService, jobHistoryService, jobHistoryRepo, jobQueueFeeder, triggerScheduler,
		sceneService, tagService, searchService, scanService, explorerService, retryScheduler, dlqService,
		actorService, studioService,
	)
}
