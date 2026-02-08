package server

import (
	"context"
	"errors"
	"goonhub/internal/config"
	"goonhub/internal/infrastructure/logging"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ShareServer is a lightweight HTTP server that serves only share-related routes.
// It is designed to be exposed on a separate public domain while the main app stays behind a VPN.
// If Port is empty, the share server is disabled and all methods are no-ops.
type ShareServer struct {
	router *gin.Engine
	port   string
	cfg    *config.Config
	logger *logging.Logger
	srv    *http.Server
}

// NewShareServer creates a new ShareServer. Returns nil if port is empty (feature disabled).
func NewShareServer(router *gin.Engine, port string, cfg *config.Config, logger *logging.Logger) *ShareServer {
	if port == "" {
		return nil
	}
	return &ShareServer{
		router: router,
		port:   port,
		cfg:    cfg,
		logger: logger,
	}
}

// Start begins listening on the configured port. No-op on nil receiver.
func (s *ShareServer) Start() {
	if s == nil {
		return
	}

	s.srv = &http.Server{
		Addr:              ":" + s.port,
		Handler:           s.router,
		ReadHeaderTimeout: s.cfg.Server.ReadTimeout,
		ReadTimeout:       s.cfg.Server.ReadTimeout,
		WriteTimeout:      0, // Disabled for video streaming
		IdleTimeout:       s.cfg.Server.IdleTimeout,
	}

	go func() {
		if s.cfg.Server.TLSCertFile != "" && s.cfg.Server.TLSKeyFile != "" {
			s.logger.Info("Starting share HTTPS server",
				zap.String("port", s.port),
				zap.String("cert", s.cfg.Server.TLSCertFile),
			)
			if err := s.srv.ListenAndServeTLS(s.cfg.Server.TLSCertFile, s.cfg.Server.TLSKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.logger.Fatal("Share HTTPS server start failed", zap.Error(err))
			}
		} else {
			s.logger.Info("Starting share HTTP server", zap.String("port", s.port))
			if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.logger.Fatal("Share HTTP server start failed", zap.Error(err))
			}
		}
	}()
}

// Shutdown gracefully shuts down the share server. No-op on nil receiver.
func (s *ShareServer) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	s.logger.Info("Shutting down share server...")
	return s.srv.Shutdown(ctx)
}
