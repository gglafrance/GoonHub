package core

import (
	"fmt"
	"goonhub/internal/data"
	"unicode"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Password validation constants
const (
	MinPasswordLength = 12
	MaxPasswordLength = 128
)

// ValidatePassword checks password meets security requirements.
// Returns nil if valid, error describing the issue otherwise.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		return fmt.Errorf("password must not exceed %d characters", MaxPasswordLength)
	}

	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}

	return nil
}

type AdminService struct {
	userRepo data.UserRepository
	roleRepo data.RoleRepository
	rbac     *RBACService
	logger   *zap.Logger
}

type AdminUserListItem struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	CreatedAt   string `json:"created_at"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}

func NewAdminService(userRepo data.UserRepository, roleRepo data.RoleRepository, rbac *RBACService, logger *zap.Logger) *AdminService {
	return &AdminService{
		userRepo: userRepo,
		roleRepo: roleRepo,
		rbac:     rbac,
		logger:   logger,
	}
}

func (s *AdminService) ListUsers(page, limit int) ([]data.User, int64, error) {
	return s.userRepo.List(page, limit)
}

func (s *AdminService) CreateUser(username, password, role string) error {
	// Validate password complexity
	if err := ValidatePassword(password); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	if _, err := s.roleRepo.GetByName(role); err != nil {
		return fmt.Errorf("invalid role: %s", role)
	}

	exists, err := s.userRepo.Exists(username)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		return fmt.Errorf("username already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := &data.User{
		Username: username,
		Password: string(hashedPassword),
		Role:     role,
	}

	if err := s.userRepo.Create(user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	s.logger.Info("Admin created user", zap.String("username", username), zap.String("role", role))
	return nil
}

func (s *AdminService) UpdateUserRole(userID uint, newRole string) error {
	if _, err := s.roleRepo.GetByName(newRole); err != nil {
		return fmt.Errorf("invalid role: %s", newRole)
	}

	if err := s.userRepo.UpdateRole(userID, newRole); err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	s.logger.Info("Admin updated user role", zap.Uint("user_id", userID), zap.String("new_role", newRole))
	return nil
}

func (s *AdminService) ResetUserPassword(userID uint, newPassword string) error {
	// Validate password complexity
	if err := ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(userID, string(hashedPassword)); err != nil {
		return fmt.Errorf("failed to reset password: %w", err)
	}

	s.logger.Info("Admin reset user password", zap.Uint("user_id", userID))
	return nil
}

func (s *AdminService) DeleteUser(userID, requestingUserID uint) error {
	if userID == requestingUserID {
		return fmt.Errorf("cannot delete your own account")
	}

	if err := s.userRepo.Delete(userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	s.logger.Info("Admin deleted user", zap.Uint("user_id", userID), zap.Uint("deleted_by", requestingUserID))
	return nil
}
