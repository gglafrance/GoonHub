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
	// Check if admin already exists
	exists, err := s.repo.Exists(username)
	if err != nil {
		return fmt.Errorf("failed to check admin existence: %w", err)
	}

	if exists {
		s.logger.Info("Admin user already exists", zap.String("username", username))
		return nil
	}

	// In production, only create admin on first-time setup (no users exist)
	if environment == "production" {
		userCount, err := s.repo.Count()
		if err != nil {
			return fmt.Errorf("failed to count users: %w", err)
		}
		if userCount > 0 {
			s.logger.Info("Skipping admin creation in production (users already exist)")
			return nil
		}
		s.logger.Info("First-time setup: creating admin user in production")
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
