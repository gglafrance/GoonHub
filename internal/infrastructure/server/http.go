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
		s.logger.Info("Search indexer wired to services")
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

	s.srv = &http.Server{
		Addr:         ":" + s.cfg.Server.Port,
		Handler:      s.router,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
		IdleTimeout:  s.cfg.Server.IdleTimeout,
	}

	go func() {
		s.logger.Info("Starting server", zap.String("port", s.cfg.Server.Port))
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Fatal("Server start failed", zap.Error(err))
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
