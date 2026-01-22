package core

import (
	"fmt"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo   data.UserRepository
	logger *zap.Logger
}

func NewUserService(repo data.UserRepository, logger *zap.Logger) *UserService {
	return &UserService{
		repo:   repo,
		logger: logger,
	}
}

func (s *UserService) EnsureAdminExists(username, password, environment string) error {
	// Skip default admin creation in production
	if environment == "production" {
		s.logger.Info("Skipping default admin creation in production")
		return nil
	}

	exists, err := s.repo.Exists(username)
	if err != nil {
		return fmt.Errorf("failed to check admin existence: %w", err)
	}

	if exists {
		s.logger.Info("Admin user already exists", zap.String("username", username))
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	admin := &data.User{
		Username: username,
		Password: string(hashedPassword),
		Role:     "admin",
	}

	if err := s.repo.Create(admin); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	s.logger.Info("Admin user created", zap.String("username", username), zap.Uint("user_id", admin.ID))
	return nil
}
