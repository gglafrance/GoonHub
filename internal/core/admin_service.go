package core

import (
	"errors"
	"fmt"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"unicode"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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

// CreateUser creates an account. A non-superuser actor can only give a role within their
// own permissions (docs/RBAC.md, "Delegation").
func (s *AdminService) CreateUser(actor *UserAccess, username, password, role string) error {
	// Validate password complexity
	if err := ValidatePassword(password); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	if _, err := s.roleRepo.GetByName(role); err != nil {
		return fmt.Errorf("invalid role: %s", role)
	}
	if err := s.rbac.CheckAssignableRole(actor, role); err != nil {
		return err
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

// UpdateUserRole changes a user's role. A non-superuser actor cannot change their own role
// nor a superuser account's, and can only give a role within their own permissions. The last
// user holding a superuser role keeps it (checked atomically by the repository).
func (s *AdminService) UpdateUserRole(actor *UserAccess, userID uint, newRole string) error {
	if _, err := s.roleRepo.GetByName(newRole); err != nil {
		return fmt.Errorf("invalid role: %s", newRole)
	}
	target, err := s.getUser(userID)
	if err != nil {
		return err
	}
	if err := s.rbac.CheckManageableUser(actor, target, false); err != nil {
		return err
	}
	if err := s.rbac.CheckAssignableRole(actor, newRole); err != nil {
		return err
	}

	if err := s.userRepo.UpdateRole(userID, newRole); err != nil {
		if errors.Is(err, data.ErrLastSuperuser) {
			return apperrors.NewValidationError(err.Error())
		}
		return fmt.Errorf("failed to update user role: %w", err)
	}
	s.rbac.InvalidateUser(userID)

	s.logger.Info("Admin updated user role", zap.Uint("user_id", userID), zap.String("new_role", newRole), zap.Uint("by", actor.UserID))
	return nil
}

func (s *AdminService) getUser(userID uint) (*data.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("user", userID)
		}
		return nil, apperrors.NewInternalError("failed to get user", err)
	}
	return user, nil
}

// SetUserShareMarkers sets whether the user's markers are visible (read-only) to every user.
func (s *AdminService) SetUserShareMarkers(userID uint, share bool) error {
	if _, err := s.userRepo.GetByID(userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.NewNotFoundError("user", userID)
		}
		return apperrors.NewInternalError("failed to get user", err)
	}
	if err := s.userRepo.UpdateShareMarkers(userID, share); err != nil {
		return apperrors.NewInternalError("failed to update marker sharing", err)
	}
	s.logger.Info("Admin updated user marker sharing", zap.Uint("user_id", userID), zap.Bool("share_markers", share))
	return nil
}

// ResetUserPassword sets a user's password. A non-superuser actor cannot reset the password
// of a superuser account.
func (s *AdminService) ResetUserPassword(actor *UserAccess, userID uint, newPassword string) error {
	// Validate password complexity
	if err := ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}
	target, err := s.getUser(userID)
	if err != nil {
		return err
	}
	if err := s.rbac.CheckManageableUser(actor, target, true); err != nil {
		return err
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

// DeleteUser deletes an account (not one's own). A non-superuser actor cannot delete a
// superuser account; the last user holding a superuser role cannot be deleted.
func (s *AdminService) DeleteUser(actor *UserAccess, userID uint) error {
	if actor == nil {
		return errNoActor()
	}
	if userID == actor.UserID {
		return fmt.Errorf("cannot delete your own account")
	}
	target, err := s.getUser(userID)
	if err != nil {
		return err
	}
	if err := s.rbac.CheckManageableUser(actor, target, true); err != nil {
		return err
	}

	if err := s.userRepo.Delete(userID); err != nil {
		if errors.Is(err, data.ErrLastSuperuser) {
			return apperrors.NewValidationError(err.Error())
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}
	s.rbac.InvalidateUser(userID)

	s.logger.Info("Admin deleted user", zap.Uint("user_id", userID), zap.Uint("deleted_by", actor.UserID))
	return nil
}
