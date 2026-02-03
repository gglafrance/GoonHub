package server

import (
	"context"
	"errors"
	"fmt"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/logging"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Server struct {
	router            *gin.Engine
	logger            *logging.Logger
	cfg               *config.Config
	processingService *core.SceneProcessingService
	userService       *core.UserService
	jobHistoryService *core.JobHistoryService
	jobHistoryRepo    data.JobHistoryRepository
	jobQueueFeeder    *core.JobQueueFeeder
	triggerScheduler  *core.TriggerScheduler
	sceneService      *core.SceneService
	tagService        *core.TagService
	searchService     *core.SearchService
	scanService       *core.ScanService
	explorerService   *core.ExplorerService
	retryScheduler    *core.RetryScheduler
	dlqService        *core.DLQService
	actorService      *core.ActorService
	studioService     *core.StudioService
	srv               *http.Server
}

func NewHTTPServer(
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
) *Server {
	return &Server{
		router:            router,
		logger:            logger,
		cfg:               cfg,
		processingService: processingService,
		userService:       userService,
		jobHistoryService: jobHistoryService,
		jobHistoryRepo:    jobHistoryRepo,
		jobQueueFeeder:    jobQueueFeeder,
		triggerScheduler:  triggerScheduler,
		sceneService:      sceneService,
		tagService:        tagService,
		searchService:     searchService,
		scanService:       scanService,
		explorerService:   explorerService,
		retryScheduler:    retryScheduler,
		dlqService:        dlqService,
		actorService:      actorService,
		studioService:     studioService,
	}
}

func (s *Server) Start() error {
	if err := s.userService.EnsureAdminExists(s.cfg.Auth.AdminUsername, s.cfg.Auth.AdminPassword, s.cfg.Environment); err != nil {
		return fmt.Errorf("failed to ensure admin user exists: %w", err)
	}

	// Wire up search indexer to services that need it
	if s.searchService != nil {
		if s.sceneService != nil {
			s.sceneService.SetIndexer(s.searchService)
		}
		if s.tagService != nil {
			s.tagService.SetIndexer(s.searchService)
		}
		if s.processingService != nil {
			s.processingService.SetIndexer(s.searchService)
		}
		if s.scanService != nil {
			s.scanService.SetIndexer(s.searchService)
		}
		if s.explorerService != nil {
			s.explorerService.SetIndexer(s.searchService)
			s.explorerService.SetSearchService(s.searchService)
		}
		if s.actorService != nil {
			s.actorService.SetIndexer(s.searchService)
		}
		if s.studioService != nil {
			s.studioService.SetIndexer(s.searchService)
		}
		s.logger.Info("Search indexer wired to services")
	}

	// Recover any interrupted scans from previous runs
	if s.scanService != nil {
		s.scanService.RecoverInterruptedScans()
	}

	// Wire up scan service to trigger scheduler for scheduled scans
	if s.triggerScheduler != nil && s.scanService != nil {
		s.triggerScheduler.SetScanService(s.scanService)
	}

	// Configure job queue feeder with shutdown config timeouts
	if s.jobQueueFeeder != nil {
		s.jobQueueFeeder.SetOrphanTimeout(s.cfg.Shutdown.OrphanTimeout)
		s.jobQueueFeeder.SetStuckPendingTime(s.cfg.Shutdown.StuckPendingTime)
	}

	if s.processingService != nil {
		s.processingService.Start()
	}

	// Start job queue feeder AFTER processing service starts
	// The feeder moves pending jobs from DB to worker pools
	if s.jobQueueFeeder != nil {
		s.jobQueueFeeder.Start()
	}

	if s.jobHistoryService != nil {
		s.jobHistoryService.StartCleanupTicker()
	}

	if s.triggerScheduler != nil {
		s.triggerScheduler.Start()
	}

	// Wire up retry scheduler and DLQ service to processing service
	if s.retryScheduler != nil {
		s.retryScheduler.SetProcessingService(s.processingService)
		s.retryScheduler.SetJobHistoryService(s.jobHistoryService)
		s.retryScheduler.Start()
	}

	if s.dlqService != nil {
		s.dlqService.SetProcessingService(s.processingService)
	}

	// Wire retry scheduler to job history service for automatic retry scheduling
	if s.jobHistoryService != nil && s.retryScheduler != nil {
		s.jobHistoryService.SetRetryScheduler(s.retryScheduler)
	}

	// Wire processing service to job history service for manual job retries
	if s.jobHistoryService != nil && s.processingService != nil {
		s.jobHistoryService.SetProcessingService(s.processingService)
	}

	s.srv = &http.Server{
		Addr:        ":" + s.cfg.Server.Port,
		Handler:     s.router,
		ReadTimeout: s.cfg.Server.ReadTimeout,
		// WriteTimeout: 0 disables the timeout, required for video streaming.
		// Video streams can be hours long and must not be killed by timeout.
		WriteTimeout: 0,
		IdleTimeout:  s.cfg.Server.IdleTimeout,
	}

	go func() {
		// Check if TLS is configured
		if s.cfg.Server.TLSCertFile != "" && s.cfg.Server.TLSKeyFile != "" {
			s.logger.Info("Starting HTTPS server",
				zap.String("port", s.cfg.Server.Port),
				zap.String("cert", s.cfg.Server.TLSCertFile),
			)
			if err := s.srv.ListenAndServeTLS(s.cfg.Server.TLSCertFile, s.cfg.Server.TLSKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.logger.Fatal("HTTPS server start failed", zap.Error(err))
			}
		} else {
			s.logger.Info("Starting HTTP server", zap.String("port", s.cfg.Server.Port))
			if s.cfg.Environment == "production" {
				s.logger.Warn("Running HTTP without TLS in production - configure tls_cert_file and tls_key_file for HTTPS")
			}
			if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.logger.Fatal("HTTP server start failed", zap.Error(err))
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// ===========================================================================
	// MULTI-PHASE GRACEFUL SHUTDOWN
	// ===========================================================================
	s.logger.Info("Initiating graceful shutdown...",
		zap.Duration("graceful_timeout", s.cfg.Shutdown.GracefulTimeout),
		zap.Duration("job_completion_wait", s.cfg.Shutdown.JobCompletionWait),
	)

	// ---------------------------------------------------------------------------
	// PHASE 1: STOP INTAKE
	// Stop accepting new jobs - feeder, scheduler, retry all stop polling
	// ---------------------------------------------------------------------------
	s.logger.Info("PHASE 1: Stopping job intake...")

	if s.jobQueueFeeder != nil {
		s.jobQueueFeeder.Stop()
		s.logger.Info("Job queue feeder stopped")
	}

	if s.triggerScheduler != nil {
		s.triggerScheduler.Stop()
		s.logger.Info("Trigger scheduler stopped")
	}

	if s.retryScheduler != nil {
		s.retryScheduler.Stop()
		s.logger.Info("Retry scheduler stopped")
	}

	// ---------------------------------------------------------------------------
	// PHASE 2: COMPLETE IN-FLIGHT WORK
	// Wait for currently executing jobs to finish (with timeout)
	// Also drains channel buffers and returns those job IDs
	// ---------------------------------------------------------------------------
	s.logger.Info("PHASE 2: Waiting for in-flight jobs to complete...")

	var bufferedJobs map[string][]string
	if s.processingService != nil {
		bufferedJobs = s.processingService.GracefulStop(s.cfg.Shutdown.JobCompletionWait)
	}

	// ---------------------------------------------------------------------------
	// PHASE 3: RECLAIM BUFFERED JOBS
	// Reset buffered jobs back to pending so they'll be picked up on restart
	// Mark any remaining running jobs as failed (retryable)
	// ---------------------------------------------------------------------------
	s.logger.Info("PHASE 3: Reclaiming buffered jobs...")

	if s.jobHistoryRepo != nil {
		totalReclaimed := int64(0)
		for phase, jobIDs := range bufferedJobs {
			if len(jobIDs) > 0 {
				count, err := s.jobHistoryRepo.ResetJobsToPending(jobIDs)
				if err != nil {
					s.logger.Error("Failed to reset buffered jobs to pending",
						zap.String("phase", phase),
						zap.Error(err),
					)
				} else {
					totalReclaimed += count
					s.logger.Info("Reset buffered jobs to pending",
						zap.String("phase", phase),
						zap.Int64("count", count),
					)
				}
			}
		}

		// Mark any remaining running jobs as interrupted (failed but retryable)
		interruptedCount, err := s.jobHistoryRepo.MarkRunningAsInterrupted()
		if err != nil {
			s.logger.Error("Failed to mark running jobs as interrupted", zap.Error(err))
		} else if interruptedCount > 0 {
			s.logger.Info("Marked running jobs as interrupted",
				zap.Int64("count", interruptedCount),
			)
		}

		s.logger.Info("Phase 3 complete",
			zap.Int64("jobs_reset_to_pending", totalReclaimed),
			zap.Int64("jobs_marked_interrupted", interruptedCount),
		)
	}

	// ---------------------------------------------------------------------------
	// PHASE 4: CLEANUP
	// Stop remaining services and HTTP server
	// ---------------------------------------------------------------------------
	s.logger.Info("PHASE 4: Final cleanup...")

	if s.jobHistoryService != nil {
		s.jobHistoryService.StopCleanupTicker()
	}

	// Shutdown HTTP server with remaining graceful timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Shutdown.GracefulTimeout)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	s.logger.Info("Server shutdown complete")
	return nil
}
