package server

import (
	"context"
	"errors"
	"fmt"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Server struct {
	router            *gin.Engine
	logger            *logging.Logger
	cfg               *config.Config
	processingService *core.VideoProcessingService
	userService       *core.UserService
	jobHistoryService *core.JobHistoryService
	triggerScheduler  *core.TriggerScheduler
	videoService      *core.VideoService
	tagService        *core.TagService
	searchService     *core.SearchService
	scanService       *core.ScanService
	retryScheduler    *core.RetryScheduler
	dlqService        *core.DLQService
	srv               *http.Server
}

func NewHTTPServer(
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
	retryScheduler *core.RetryScheduler,
	dlqService *core.DLQService,
) *Server {
	return &Server{
		router:            router,
		logger:            logger,
		cfg:               cfg,
		processingService: processingService,
		userService:       userService,
		jobHistoryService: jobHistoryService,
		triggerScheduler:  triggerScheduler,
		videoService:      videoService,
		tagService:        tagService,
		searchService:     searchService,
		scanService:       scanService,
		retryScheduler:    retryScheduler,
		dlqService:        dlqService,
	}
}

func (s *Server) Start() error {
	if err := s.userService.EnsureAdminExists(s.cfg.Auth.AdminUsername, s.cfg.Auth.AdminPassword, s.cfg.Environment); err != nil {
		return fmt.Errorf("failed to ensure admin user exists: %w", err)
	}

	// Wire up search indexer to services that need it
	if s.searchService != nil {
		if s.videoService != nil {
			s.videoService.SetIndexer(s.searchService)
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

	if s.processingService != nil {
		s.processingService.Start()
		defer s.processingService.Stop()
	}

	if s.jobHistoryService != nil {
		s.jobHistoryService.StartCleanupTicker()
		defer s.jobHistoryService.StopCleanupTicker()
	}

	if s.triggerScheduler != nil {
		s.triggerScheduler.Start()
		defer s.triggerScheduler.Stop()
	}

	// Wire up retry scheduler and DLQ service to processing service
	if s.retryScheduler != nil {
		s.retryScheduler.SetProcessingService(s.processingService)
		s.retryScheduler.SetJobHistoryService(s.jobHistoryService)
		s.retryScheduler.Start()
		defer s.retryScheduler.Stop()
	}

	if s.dlqService != nil {
		s.dlqService.SetProcessingService(s.processingService)
	}

	// Wire retry scheduler to job history service for automatic retry scheduling
	if s.jobHistoryService != nil && s.retryScheduler != nil {
		s.jobHistoryService.SetRetryScheduler(s.retryScheduler)
	}

	s.srv = &http.Server{
		Addr:         ":" + s.cfg.Server.Port,
		Handler:      s.router,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
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

	s.logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	s.logger.Info("Server exiting")
	return nil
}
