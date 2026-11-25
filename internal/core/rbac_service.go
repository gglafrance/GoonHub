package core

import (
	"errors"
	"fmt"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// userAccessTTL bounds how long a user's resolved access, and the role graph, are reused.
// Every change made through the API invalidates them at once; the TTL only covers changes
// made outside of it (SQL on users, roles, role_parents, role_permissions, overrides).
const userAccessTTL = time.Minute

// roleNamePattern is the format of a role name.
var roleNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,49}$`)

// UserAccess is the live access of a user: current role and effective permissions.
type UserAccess struct {
	UserID    uint
	Role      string
	Superuser bool
	granted   map[string]bool
	names     []string
	loadedAt  time.Time
}

// NewUserAccess builds an access from a role and a list of granted permissions (tests and
// callers that do not go through the resolver).
func NewUserAccess(userID uint, role string, superuser bool, permissions ...string) *UserAccess {
	a := &UserAccess{UserID: userID, Role: role, Superuser: superuser, granted: make(map[string]bool, len(permissions))}
	for _, p := range permissions {
		a.granted[p] = true
	}
	a.names = sortedKeys(a.granted)
	return a
}

// Has reports whether the user has the permission. A superuser has every permission.
func (a *UserAccess) Has(permission string) bool {
	if a == nil {
		return false
	}
	return a.Superuser || a.granted[permission]
}

// Permissions returns the names of the granted permissions, sorted.
func (a *UserAccess) Permissions() []string {
	if a == nil {
		return []string{}
	}
	out := make([]string, len(a.names))
	copy(out, a.names)
	return out
}

// PermissionSetting is a role setting or a user override.
type PermissionSetting struct {
	PermissionID uint   `json:"permission_id"`
	Effect       string `json:"effect"`
}

// PermissionView is a catalog permission as listed by the admin API.
type PermissionView struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Group       string `json:"group"`
	Description string `json:"description"`
}

// RoleView is a role as listed by the admin API.
type RoleView struct {
	ID          uint                `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	IsSystem    bool                `json:"is_system"`
	IsSuperuser bool                `json:"is_superuser"`
	ParentIDs   []uint              `json:"parent_ids"`
	Settings    []PermissionSetting `json:"settings"`
	UserCount   int64               `json:"user_count"`
}

// EffectivePermission is the resolved value of a permission with where it comes from.
type EffectivePermission struct {
	Name    string `json:"name"`
	Granted bool   `json:"granted"`
	Source  string `json:"source"`
}

// UserPermissionsView is a user's role, overrides and effective permissions.
type UserPermissionsView struct {
	Role      string                `json:"role"`
	Overrides []PermissionSetting   `json:"overrides"`
	Effective []EffectivePermission `json:"effective"`
}

// RoleInput is the body of a role creation or update.
type RoleInput struct {
	Name        string
	Description string
	ParentIDs   []uint
}

// RoleInUseError is returned when deleting a role still held by users.
type RoleInUseError struct {
	*apperrors.ConflictError
	UserCount int64
}

func (e *RoleInUseError) Unwrap() error { return e.ConflictError }

// RBACService resolves permissions (docs/RBAC.md) and manages roles, their parents and
// settings, and user overrides. Users' access is resolved live from the database and cached
// until a change invalidates it.
type RBACService struct {
	roleRepo data.RoleRepository
	permRepo data.PermissionRepository
	userRepo data.UserRepository
	logger   *zap.Logger

	mu    sync.RWMutex
	graph *rbacGraph
	users map[uint]*UserAccess
	// gen changes on every invalidation: an access resolved meanwhile is not cached
	gen uint64
	// graphLoadedAt is when the role graph was loaded (reloaded after userAccessTTL)
	graphLoadedAt time.Time
	// refreshing guards the reload of a stale graph (one at a time, others use the old one)
	refreshing sync.Mutex
	// writeMu serializes the changes (cycle checks read the graph they modify)
	writeMu sync.Mutex
	now     func() time.Time
}

func NewRBACService(roleRepo data.RoleRepository, permRepo data.PermissionRepository, userRepo data.UserRepository, logger *zap.Logger) (*RBACService, error) {
	s := &RBACService{
		roleRepo: roleRepo,
		permRepo: permRepo,
		userRepo: userRepo,
		logger:   logger,
		users:    make(map[uint]*UserAccess),
		now:      time.Now,
	}
	if err := s.RefreshCache(); err != nil {
		return nil, fmt.Errorf("failed to initialize RBAC cache: %w", err)
	}
	return s, nil
}

// RefreshCache reloads the roles, parents, settings and permissions and drops every cached
// user access.
func (s *RBACService) RefreshCache() error {
	graph, err := s.roleRepo.LoadGraph()
	if err != nil {
		return fmt.Errorf("failed to load roles: %w", err)
	}
	perms, err := s.permRepo.List()
	if err != nil {
		return fmt.Errorf("failed to load permissions: %w", err)
	}
	g := newRBACGraph(graph, perms)

	s.mu.Lock()
	s.graph = g
	s.users = make(map[uint]*UserAccess)
	s.gen++
	s.graphLoadedAt = s.now()
	s.mu.Unlock()

	s.logger.Info("RBAC cache refreshed", zap.Int("roles", len(g.roles)), zap.Int("permissions", len(perms)))
	return nil
}

// refreshIfStale reloads the role graph when it is older than userAccessTTL. A failed reload
// keeps the current graph (retried on the next call).
func (s *RBACService) refreshIfStale() {
	s.mu.RLock()
	stale := s.now().Sub(s.graphLoadedAt) >= userAccessTTL
	s.mu.RUnlock()
	if !stale || !s.refreshing.TryLock() {
		return
	}
	defer s.refreshing.Unlock()
	s.mu.RLock()
	stale = s.now().Sub(s.graphLoadedAt) >= userAccessTTL
	s.mu.RUnlock()
	if !stale {
		return
	}
	if err := s.RefreshCache(); err != nil {
		s.logger.Warn("Failed to reload the roles and permissions", zap.Error(err))
	}
}

func (s *RBACService) currentGraph() *rbacGraph {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.graph
}

// InvalidateUser drops the cached access of a user (role or overrides changed, user deleted).
func (s *RBACService) InvalidateUser(userID uint) {
	s.mu.Lock()
	delete(s.users, userID)
	s.gen++
	s.mu.Unlock()
}

// ResolveUser returns the live access of a user: current role and effective permissions.
// A user that no longer exists gives a NotFoundError.
func (s *RBACService) ResolveUser(userID uint) (*UserAccess, error) {
	s.refreshIfStale()

	s.mu.RLock()
	cached, ok := s.users[userID]
	g := s.graph
	gen := s.gen
	s.mu.RUnlock()
	if ok && s.now().Sub(cached.loadedAt) < userAccessTTL {
		return cached, nil
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("user", userID)
		}
		return nil, apperrors.NewInternalError("failed to load user", err)
	}
	overrides, err := s.permRepo.ListUserOverrides(userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load user permission overrides", err)
	}
	access := g.userAccess(user, overrides)
	access.loadedAt = s.now()

	s.mu.Lock()
	// Keep it only if nothing was invalidated meanwhile (it may be computed on old data)
	if s.gen == gen {
		s.users[userID] = access
	}
	s.mu.Unlock()
	return access, nil
}

// UserHasPermission reports whether a user currently has a permission (false when the user
// cannot be resolved).
func (s *RBACService) UserHasPermission(userID uint, permission string) bool {
	access, err := s.ResolveUser(userID)
	if err != nil {
		return false
	}
	return access.Has(permission)
}

// userAccess computes the effective permissions of a user.
func (g *rbacGraph) userAccess(user *data.User, overrides []data.UserPermissionOverride) *UserAccess {
	access := &UserAccess{UserID: user.ID, Role: user.Role, granted: make(map[string]bool)}
	roleID, ok := g.roleByName[user.Role]
	if ok && g.roles[roleID].IsSuperuser {
		access.Superuser = true
		for _, p := range g.visible {
			access.granted[p.Name] = true
		}
		access.names = sortedKeys(access.granted)
		return access
	}
	byPerm := make(map[uint]string, len(overrides))
	for _, o := range overrides {
		byPerm[o.PermissionID] = o.Effect
	}
	for _, p := range g.perms {
		if DeprecatedPermissions[p.Name] {
			continue
		}
		var granted bool
		if ok {
			granted, _ = g.userDecision(roleID, byPerm, p.ID)
		} else if effect, has := byPerm[p.ID]; has {
			granted = effect == data.PermissionEffectAllow
		}
		if granted {
			access.granted[p.Name] = true
		}
	}
	access.names = sortedKeys(access.granted)
	return access
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// SyncCatalog inserts the catalog permissions missing from the table, with their default
// grants, and keeps the group and description of the others up to date. Permissions that
// are not in the catalog are left alone.
func (s *RBACService) SyncCatalog() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	perms, err := s.permRepo.List()
	if err != nil {
		return fmt.Errorf("failed to list permissions: %w", err)
	}
	graph, err := s.roleRepo.LoadGraph()
	if err != nil {
		return fmt.Errorf("failed to load roles: %w", err)
	}
	byName := make(map[string]data.Permission, len(perms))
	for _, p := range perms {
		byName[p.Name] = p
	}
	roleByName := make(map[string]data.Role, len(graph.Roles))
	superusers := make(map[uint]bool)
	for _, r := range graph.Roles {
		roleByName[r.Name] = r
		if r.IsSuperuser {
			superusers[r.ID] = true
		}
	}
	allowedBy := make(map[uint][]uint)
	for _, st := range graph.Settings {
		if st.Effect == data.PermissionEffectAllow {
			allowedBy[st.PermissionID] = append(allowedBy[st.PermissionID], st.RoleID)
		}
	}

	for _, entry := range PermissionCatalog {
		if existing, ok := byName[entry.Name]; ok {
			if existing.GroupName != entry.Group || existing.Description != entry.Description {
				if err := s.permRepo.UpdateMeta(existing.ID, entry.Group, entry.Description); err != nil {
					return fmt.Errorf("failed to update permission %s: %w", entry.Name, err)
				}
			}
			continue
		}
		perm := &data.Permission{Name: entry.Name, GroupName: entry.Group, Description: entry.Description}
		if err := s.permRepo.Create(perm); err != nil {
			return fmt.Errorf("failed to create permission %s: %w", entry.Name, err)
		}
		byName[entry.Name] = *perm

		seen := make(map[uint]bool)
		var roleIDs []uint
		add := func(id uint) {
			if !seen[id] && !superusers[id] {
				seen[id] = true
				roleIDs = append(roleIDs, id)
			}
		}
		for _, name := range entry.GrantRoles {
			if r, ok := roleByName[name]; ok {
				add(r.ID)
			}
		}
		if entry.GrantFrom != "" {
			if from, ok := byName[entry.GrantFrom]; ok {
				for _, id := range allowedBy[from.ID] {
					add(id)
				}
			}
		}
		sort.Slice(roleIDs, func(i, j int) bool { return roleIDs[i] < roleIDs[j] })
		if err := s.permRepo.GrantToRoles(perm.ID, roleIDs); err != nil {
			return fmt.Errorf("failed to grant permission %s: %w", entry.Name, err)
		}
		s.logger.Info("Permission added from the catalog", zap.String("permission", entry.Name), zap.Int("granted_roles", len(roleIDs)))
	}
	return s.RefreshCache()
}

// ListPermissions returns the catalog permissions (deprecated ones excluded), in catalog
// order.
func (s *RBACService) ListPermissions() []PermissionView {
	g := s.currentGraph()
	out := make([]PermissionView, len(g.visible))
	for i, p := range g.visible {
		out[i] = PermissionView{ID: p.ID, Name: p.Name, Group: p.GroupName, Description: p.Description}
	}
	return out
}

// ListRoles returns every role with its parents, own settings and number of users.
func (s *RBACService) ListRoles() ([]RoleView, error) {
	counts, err := s.roleRepo.CountUsersPerRole()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count users per role", err)
	}
	g := s.currentGraph()
	ids := make([]uint, 0, len(g.roles))
	for id := range g.roles {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]RoleView, 0, len(ids))
	for _, id := range ids {
		out = append(out, g.roleView(id, counts[g.roles[id].Name]))
	}
	return out, nil
}

func (g *rbacGraph) roleView(id uint, userCount int64) RoleView {
	r := g.roles[id]
	view := RoleView{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		IsSystem:    r.IsSystem,
		IsSuperuser: r.IsSuperuser,
		ParentIDs:   append([]uint{}, g.parents[id]...),
		Settings:    []PermissionSetting{},
		UserCount:   userCount,
	}
	for _, p := range g.visible {
		if effect, ok := g.settings[id][p.ID]; ok {
			view.Settings = append(view.Settings, PermissionSetting{PermissionID: p.ID, Effect: effect})
		}
	}
	return view
}

// GetRole returns a role view.
func (s *RBACService) GetRole(id uint) (*RoleView, error) {
	g := s.currentGraph()
	r, ok := g.roles[id]
	if !ok {
		return nil, apperrors.NewNotFoundError("role", id)
	}
	count, err := s.roleRepo.CountUsersByRole(r.Name)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count role users", err)
	}
	view := g.roleView(id, count)
	return &view, nil
}

// validateRoleInput checks the name, description and parents of a role (roleID 0: new role).
func (g *rbacGraph) validateRoleInput(roleID uint, in *RoleInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if !roleNamePattern.MatchString(in.Name) {
		return apperrors.NewValidationErrorWithField("name", "role name must be 2 to 50 characters: lowercase letters, digits, '-' and '_', starting with a letter or a digit")
	}
	if len(in.Description) > 255 {
		return apperrors.NewValidationErrorWithField("description", "description must not exceed 255 characters")
	}
	if existing, ok := g.roleByName[in.Name]; ok && existing != roleID {
		return apperrors.NewConflictError("role", fmt.Sprintf("a role named %q already exists", in.Name))
	}
	seen := make(map[uint]bool, len(in.ParentIDs))
	parents := make([]uint, 0, len(in.ParentIDs))
	for _, pid := range in.ParentIDs {
		if seen[pid] {
			continue
		}
		seen[pid] = true
		if _, ok := g.roles[pid]; !ok {
			return apperrors.NewValidationErrorWithField("parent_ids", fmt.Sprintf("parent role %d does not exist", pid))
		}
		if roleID != 0 && pid == roleID {
			return apperrors.NewValidationErrorWithField("parent_ids", "a role cannot be its own parent")
		}
		parents = append(parents, pid)
	}
	if roleID != 0 && g.createsCycle(roleID, parents) {
		return apperrors.NewValidationErrorWithField("parent_ids", "these parents would create a cycle")
	}
	sort.Slice(parents, func(i, j int) bool { return parents[i] < parents[j] })
	in.ParentIDs = parents
	return nil
}

// CreateRole creates a role with its parents (no setting: it inherits everything). A
// non-superuser actor can only give it parents within their own permissions.
func (s *RBACService) CreateRole(actor *UserAccess, in RoleInput) (*RoleView, error) {
	if actor == nil {
		return nil, errNoActor()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	g := s.currentGraph()
	if err := g.validateRoleInput(0, &in); err != nil {
		return nil, err
	}
	if !actor.Superuser {
		for _, pid := range in.ParentIDs {
			if err := g.checkGrantableRole(actor, pid, "inherit from"); err != nil {
				return nil, err
			}
		}
	}
	role := &data.Role{Name: in.Name, Description: in.Description}
	if err := s.roleRepo.Create(role, in.ParentIDs); err != nil {
		return nil, apperrors.NewInternalError("failed to create role", err)
	}
	if err := s.RefreshCache(); err != nil {
		return nil, apperrors.NewInternalError("failed to refresh permissions", err)
	}
	s.logger.Info("Role created", zap.String("role", role.Name), zap.Uints("parent_ids", in.ParentIDs))
	return s.GetRole(role.ID)
}

// UpdateRole renames a role, changes its description and replaces its parents. System roles
// keep their name; cycles are refused. A non-superuser actor cannot change their own role
// (nor its ancestors) and can only add parents within their own permissions.
func (s *RBACService) UpdateRole(actor *UserAccess, id uint, in RoleInput) (*RoleView, error) {
	if actor == nil {
		return nil, errNoActor()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	g := s.currentGraph()
	current, ok := g.roles[id]
	if !ok {
		return nil, apperrors.NewNotFoundError("role", id)
	}
	if !actor.Superuser {
		if err := g.checkEditableRole(actor, id); err != nil {
			return nil, err
		}
	}
	if err := g.validateRoleInput(id, &in); err != nil {
		return nil, err
	}
	if !actor.Superuser {
		existing := map[uint]bool{}
		for _, pid := range g.parents[id] {
			existing[pid] = true
		}
		for _, pid := range in.ParentIDs {
			if existing[pid] {
				continue
			}
			if err := g.checkGrantableRole(actor, pid, "inherit from"); err != nil {
				return nil, err
			}
		}
	}
	if in.Name != current.Name && (current.IsSuperuser || current.IsSystem) {
		return nil, apperrors.NewValidationErrorWithField("name", "a system role cannot be renamed")
	}
	role := &data.Role{ID: id, Name: in.Name, Description: in.Description}
	if err := s.roleRepo.Update(role, in.ParentIDs); err != nil {
		return nil, apperrors.NewInternalError("failed to update role", err)
	}
	if err := s.RefreshCache(); err != nil {
		return nil, apperrors.NewInternalError("failed to refresh permissions", err)
	}
	s.logger.Info("Role updated", zap.Uint("role_id", id), zap.String("role", in.Name), zap.Uints("parent_ids", in.ParentIDs))
	return s.GetRole(id)
}

// DeleteRole deletes a role that is not a system role and that no user holds. Its child
// roles lose it as a parent.
func (s *RBACService) DeleteRole(actor *UserAccess, id uint) error {
	if actor == nil {
		return errNoActor()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	g := s.currentGraph()
	role, ok := g.roles[id]
	if !ok {
		return apperrors.NewNotFoundError("role", id)
	}
	if role.IsSystem || role.IsSuperuser {
		return apperrors.NewValidationError("a system role cannot be deleted")
	}
	if !actor.Superuser && g.actorLineage(actor)[id] {
		return apperrors.NewForbiddenError("you cannot delete your own role nor a role it inherits from")
	}
	count, err := s.roleRepo.CountUsersByRole(role.Name)
	if err != nil {
		return apperrors.NewInternalError("failed to count role users", err)
	}
	if count > 0 {
		return &RoleInUseError{
			ConflictError: apperrors.NewConflictError("role", fmt.Sprintf("role %q is held by %d user(s)", role.Name, count)),
			UserCount:     count,
		}
	}
	if err := s.roleRepo.Delete(id); err != nil {
		return apperrors.NewInternalError("failed to delete role", err)
	}
	if err := s.RefreshCache(); err != nil {
		return apperrors.NewInternalError("failed to refresh permissions", err)
	}
	s.logger.Info("Role deleted", zap.Uint("role_id", id), zap.String("role", role.Name))
	return nil
}

// validateSettings checks permission settings: known visible permissions, allow or deny,
// each permission once.
func (g *rbacGraph) validateSettings(settings []PermissionSetting, field string) error {
	seen := make(map[uint]bool, len(settings))
	for _, st := range settings {
		p, ok := g.permByID[st.PermissionID]
		if !ok || DeprecatedPermissions[p.Name] {
			return apperrors.NewValidationErrorWithField(field, fmt.Sprintf("unknown permission %d", st.PermissionID))
		}
		if st.Effect != data.PermissionEffectAllow && st.Effect != data.PermissionEffectDeny {
			return apperrors.NewValidationErrorWithField(field, fmt.Sprintf("effect must be allow or deny (permission %s)", p.Name))
		}
		if seen[st.PermissionID] {
			return apperrors.NewValidationErrorWithField(field, fmt.Sprintf("permission %s is listed twice", p.Name))
		}
		seen[st.PermissionID] = true
	}
	return nil
}

// SetRoleSettings replaces a role's own settings (absent permission = inherit). Refused on a
// superuser role. A non-superuser actor cannot change their own role (nor its ancestors) and
// can only allow permissions they hold (or that the role already allows).
func (s *RBACService) SetRoleSettings(actor *UserAccess, id uint, settings []PermissionSetting) error {
	if actor == nil {
		return errNoActor()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	g := s.currentGraph()
	role, ok := g.roles[id]
	if !ok {
		return apperrors.NewNotFoundError("role", id)
	}
	if role.IsSuperuser {
		return apperrors.NewValidationError("a superuser role has every permission: its settings cannot be changed")
	}
	if err := g.validateSettings(settings, "settings"); err != nil {
		return err
	}
	if !actor.Superuser {
		if err := g.checkEditableRole(actor, id); err != nil {
			return err
		}
		if err := g.checkAllowedSettings(actor, settings, g.settings[id]); err != nil {
			return err
		}
	}
	rows := make([]data.RolePermission, len(settings))
	for i, st := range settings {
		rows[i] = data.RolePermission{RoleID: id, PermissionID: st.PermissionID, Effect: st.Effect}
	}
	if err := s.roleRepo.ReplaceSettings(id, rows); err != nil {
		return apperrors.NewInternalError("failed to save role permissions", err)
	}
	if err := s.RefreshCache(); err != nil {
		return apperrors.NewInternalError("failed to refresh permissions", err)
	}
	s.logger.Info("Role permissions updated", zap.Uint("role_id", id), zap.String("role", role.Name), zap.Int("settings", len(settings)))
	return nil
}

// RoleEffective returns the effective value of every catalog permission for a role, with
// its source.
func (s *RBACService) RoleEffective(id uint) ([]EffectivePermission, error) {
	g := s.currentGraph()
	role, ok := g.roles[id]
	if !ok {
		return nil, apperrors.NewNotFoundError("role", id)
	}
	out := make([]EffectivePermission, len(g.visible))
	for i, p := range g.visible {
		d := g.roleDecision(id, p.ID)
		out[i] = EffectivePermission{Name: p.Name, Granted: d.Effect == data.PermissionEffectAllow, Source: d.source(role.Name)}
	}
	return out, nil
}

func (s *RBACService) getUser(userID uint) (*data.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("user", userID)
		}
		return nil, apperrors.NewInternalError("failed to load user", err)
	}
	return user, nil
}

// UserPermissions returns a user's role, overrides and effective permissions with their
// source.
func (s *RBACService) UserPermissions(userID uint) (*UserPermissionsView, error) {
	user, err := s.getUser(userID)
	if err != nil {
		return nil, err
	}
	overrides, err := s.permRepo.ListUserOverrides(userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load user permission overrides", err)
	}
	g := s.currentGraph()
	view := &UserPermissionsView{Role: user.Role, Overrides: []PermissionSetting{}, Effective: make([]EffectivePermission, len(g.visible))}
	byPerm := make(map[uint]string, len(overrides))
	for _, o := range overrides {
		byPerm[o.PermissionID] = o.Effect
	}
	for _, p := range g.visible {
		if effect, ok := byPerm[p.ID]; ok {
			view.Overrides = append(view.Overrides, PermissionSetting{PermissionID: p.ID, Effect: effect})
		}
	}
	roleID, hasRole := g.roleByName[user.Role]
	for i, p := range g.visible {
		var granted bool
		source := PermSourceNone
		if hasRole {
			granted, source = g.userDecision(roleID, byPerm, p.ID)
		} else if effect, ok := byPerm[p.ID]; ok {
			granted, source = effect == data.PermissionEffectAllow, PermSourceOverride
		}
		view.Effective[i] = EffectivePermission{Name: p.Name, Granted: granted, Source: source}
	}
	return view, nil
}

// SetUserOverrides replaces a user's permission overrides. A non-superuser actor cannot
// change their own overrides nor those of a superuser account, and can only allow
// permissions they hold (or that the user is already allowed by an override).
func (s *RBACService) SetUserOverrides(actor *UserAccess, userID uint, overrides []PermissionSetting) error {
	if actor == nil {
		return errNoActor()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	user, err := s.getUser(userID)
	if err != nil {
		return err
	}
	if err := s.CheckManageableUser(actor, user, false); err != nil {
		return err
	}
	g := s.currentGraph()
	if err := g.validateSettings(overrides, "overrides"); err != nil {
		return err
	}
	if !actor.Superuser {
		existing, err := s.permRepo.ListUserOverrides(userID)
		if err != nil {
			return apperrors.NewInternalError("failed to load user permission overrides", err)
		}
		current := make(map[uint]string, len(existing))
		for _, o := range existing {
			current[o.PermissionID] = o.Effect
		}
		if err := g.checkAllowedSettings(actor, overrides, current); err != nil {
			return err
		}
	}
	rows := make([]data.UserPermissionOverride, len(overrides))
	for i, o := range overrides {
		rows[i] = data.UserPermissionOverride{UserID: userID, PermissionID: o.PermissionID, Effect: o.Effect}
	}
	if err := s.permRepo.ReplaceUserOverrides(userID, rows); err != nil {
		return apperrors.NewInternalError("failed to save user permission overrides", err)
	}
	s.InvalidateUser(userID)
	s.logger.Info("User permission overrides updated", zap.Uint("user_id", userID), zap.Int("overrides", len(overrides)))
	return nil
}

// IsSuperuserRole reports whether the named role is a superuser role.
func (s *RBACService) IsSuperuserRole(name string) bool {
	g := s.currentGraph()
	id, ok := g.roleByName[name]
	return ok && g.roles[id].IsSuperuser
}

// SuperuserRoleNames returns the names of the superuser roles.
func (s *RBACService) SuperuserRoleNames() []string {
	g := s.currentGraph()
	var out []string
	for _, r := range g.roles {
		if r.IsSuperuser {
			out = append(out, r.Name)
		}
	}
	sort.Strings(out)
	return out
}
