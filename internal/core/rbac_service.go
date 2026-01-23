package core

import (
	"fmt"
	"goonhub/internal/data"
	"sync"

	"go.uber.org/zap"
)

type RBACService struct {
	roleRepo data.RoleRepository
	permRepo data.PermissionRepository
	logger   *zap.Logger
	cache    map[string]map[string]bool
	mu       sync.RWMutex
}

func NewRBACService(roleRepo data.RoleRepository, permRepo data.PermissionRepository, logger *zap.Logger) (*RBACService, error) {
	s := &RBACService{
		roleRepo: roleRepo,
		permRepo: permRepo,
		logger:   logger,
		cache:    make(map[string]map[string]bool),
	}
	if err := s.RefreshCache(); err != nil {
		return nil, fmt.Errorf("failed to initialize RBAC cache: %w", err)
	}
	return s, nil
}

func (s *RBACService) HasPermission(role, permission string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	perms, exists := s.cache[role]
	if !exists {
		return false
	}
	return perms[permission]
}

func (s *RBACService) RefreshCache() error {
	rolePerms, err := s.roleRepo.GetAllRolePermissions()
	if err != nil {
		return fmt.Errorf("failed to load role permissions: %w", err)
	}

	newCache := make(map[string]map[string]bool)
	for role, perms := range rolePerms {
		permSet := make(map[string]bool)
		for _, p := range perms {
			permSet[p] = true
		}
		newCache[role] = permSet
	}

	s.mu.Lock()
	s.cache = newCache
	s.mu.Unlock()

	s.logger.Info("RBAC cache refreshed", zap.Int("roles", len(newCache)))
	return nil
}

func (s *RBACService) GetRoles() ([]data.Role, error) {
	return s.roleRepo.List()
}

func (s *RBACService) GetPermissions() ([]data.Permission, error) {
	return s.permRepo.List()
}

func (s *RBACService) SyncRolePermissions(roleID uint, permissionIDs []uint) error {
	if err := s.permRepo.SyncRolePermissions(roleID, permissionIDs); err != nil {
		return fmt.Errorf("failed to sync role permissions: %w", err)
	}
	return s.RefreshCache()
}
