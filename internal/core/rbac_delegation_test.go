package core

import (
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// delegationFixture is the RBAC fixture with an admin service, every write accepted by the
// repositories, and a non-superuser actor (user 10, role moderator: scenes:view and
// scenes:upload among the fixture permissions, plus users:manage and roles:manage).
func delegationFixture(t *testing.T) (*rbacFixture, *RBACService, *AdminService, *UserAccess) {
	t.Helper()
	f := newRBACFixture(t)
	f.users[10] = &data.User{ID: 10, Username: "mod", Role: "moderator"}
	f.roleRepo.EXPECT().GetByName(gomock.Any()).DoAndReturn(func(name string) (*data.Role, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, r := range f.graph.Roles {
			if r.Name == name {
				cp := r
				return &cp, nil
			}
		}
		return nil, gorm.ErrRecordNotFound
	}).AnyTimes()
	f.roleRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(r *data.Role, _ []uint) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		r.ID = 99
		f.graph.Roles = append(f.graph.Roles, *r)
		return nil
	}).AnyTimes()
	f.roleRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	f.roleRepo.EXPECT().Delete(gomock.Any()).Return(nil).AnyTimes()
	f.roleRepo.EXPECT().CountUsersByRole(gomock.Any()).Return(int64(0), nil).AnyTimes()
	f.roleRepo.EXPECT().ReplaceSettings(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	f.permRepo.EXPECT().ReplaceUserOverrides(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	f.userRepo.EXPECT().Exists(gomock.Any()).Return(false, nil).AnyTimes()
	f.userRepo.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
	f.userRepo.EXPECT().UpdateRole(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	f.userRepo.EXPECT().UpdatePassword(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	f.userRepo.EXPECT().Delete(gomock.Any()).Return(nil).AnyTimes()

	svc := f.service(t)
	admin := NewAdminService(f.userRepo, f.roleRepo, svc, zap.NewNop())
	actor := NewUserAccess(10, "moderator", false, PermScenesView, PermScenesUpload, PermUsersManage, PermRolesManage)
	return f, svc, admin, actor
}

const delegationPassword = "SecurePass123!"

// TestDelegation_Users checks that a non-superuser actor can only hand out what they hold
// when managing accounts, and that a superuser actor passes everything.
func TestDelegation_Users(t *testing.T) {
	allow, deny := data.PermissionEffectAllow, data.PermissionEffectDeny
	tests := []struct {
		name      string
		superuser bool
		run       func(svc *RBACService, admin *AdminService, actor *UserAccess) error
		forbidden bool
	}{
		{"create: superuser role", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.CreateUser(actor, "newbie", delegationPassword, "admin")
		}, true},
		{"create: role granting more than the actor", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.CreateUser(actor, "newbie", delegationPassword, "user") // files:view_size
		}, true},
		{"create: role within the actor", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.CreateUser(actor, "newbie", delegationPassword, "moderator")
		}, false},
		{"create: role granting nothing", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.CreateUser(actor, "newbie", delegationPassword, "nofile")
		}, false},
		{"create: superuser actor gives admin", true, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.CreateUser(actor, "newbie", delegationPassword, "admin")
		}, false},
		{"role: own role", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.UpdateUserRole(actor, 10, "nofile")
		}, true},
		{"role: superuser account", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.UpdateUserRole(actor, 1, "moderator")
		}, true},
		{"role: give the superuser role", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.UpdateUserRole(actor, 2, "admin")
		}, true},
		{"role: give a role beyond the actor", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.UpdateUserRole(actor, 3, "user")
		}, true},
		{"role: give a role within the actor", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.UpdateUserRole(actor, 2, "moderator")
		}, false},
		{"role: superuser actor promotes", true, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.UpdateUserRole(actor, 2, "admin")
		}, false},
		{"password: superuser account", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.ResetUserPassword(actor, 1, delegationPassword)
		}, true},
		{"password: regular account", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.ResetUserPassword(actor, 2, delegationPassword)
		}, false},
		{"delete: superuser account", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.DeleteUser(actor, 1)
		}, true},
		{"delete: regular account", false, func(_ *RBACService, a *AdminService, actor *UserAccess) error {
			return a.DeleteUser(actor, 2)
		}, false},
		{"overrides: own account", false, func(s *RBACService, _ *AdminService, actor *UserAccess) error {
			return s.SetUserOverrides(actor, 10, nil)
		}, true},
		{"overrides: superuser account", false, func(s *RBACService, _ *AdminService, actor *UserAccess) error {
			return s.SetUserOverrides(actor, 1, nil)
		}, true},
		{"overrides: allow what the actor lacks", false, func(s *RBACService, _ *AdminService, actor *UserAccess) error {
			return s.SetUserOverrides(actor, 2, []PermissionSetting{{PermissionID: fxEdit, Effect: allow}})
		}, true},
		{"overrides: allow what the actor holds", false, func(s *RBACService, _ *AdminService, actor *UserAccess) error {
			return s.SetUserOverrides(actor, 2, []PermissionSetting{{PermissionID: fxUpload, Effect: allow}})
		}, false},
		{"overrides: deny anything", false, func(s *RBACService, _ *AdminService, actor *UserAccess) error {
			return s.SetUserOverrides(actor, 2, []PermissionSetting{{PermissionID: fxPaths, Effect: deny}, {PermissionID: fxEdit, Effect: deny}})
		}, false},
		{"overrides: superuser actor allows anything", true, func(s *RBACService, _ *AdminService, actor *UserAccess) error {
			return s.SetUserOverrides(actor, 2, []PermissionSetting{{PermissionID: fxEdit, Effect: allow}})
		}, false},
		{"no actor", false, func(s *RBACService, _ *AdminService, _ *UserAccess) error {
			return s.SetUserOverrides(nil, 2, nil)
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, svc, admin, actor := delegationFixture(t)
			if tc.superuser {
				actor = NewUserAccess(10, "admin", true)
			}
			err := tc.run(svc, admin, actor)
			if tc.forbidden && !apperrors.IsForbidden(err) {
				t.Fatalf("expected 403, got %v", err)
			}
			if !tc.forbidden && err != nil {
				t.Fatalf("expected success, got %v", err)
			}
		})
	}
}

func TestDelegation_KeepsExistingAllowOverride(t *testing.T) {
	f, svc, _, actor := delegationFixture(t)
	f.setOverrides(2, data.UserPermissionOverride{UserID: 2, PermissionID: fxEdit, Effect: data.PermissionEffectAllow})
	// scenes:edit is already allowed: resending it with a new deny is fine
	err := svc.SetUserOverrides(actor, 2, []PermissionSetting{
		{PermissionID: fxEdit, Effect: data.PermissionEffectAllow},
		{PermissionID: fxSize, Effect: data.PermissionEffectDeny},
	})
	if err != nil {
		t.Fatalf("expected the existing allow to be kept, got %v", err)
	}
}

// TestDelegation_Roles checks the role rules for a non-superuser actor.
func TestDelegation_Roles(t *testing.T) {
	allow, deny := data.PermissionEffectAllow, data.PermissionEffectDeny
	tests := []struct {
		name      string
		actor     *UserAccess
		run       func(s *RBACService, actor *UserAccess) error
		forbidden bool
	}{
		{"settings: own role", nil, func(s *RBACService, a *UserAccess) error {
			return s.SetRoleSettings(a, fxModerator, []PermissionSetting{{PermissionID: fxView, Effect: allow}})
		}, true},
		{"settings: an ancestor of the own role", NewUserAccess(3, "child", false, PermScenesView, PermRolesManage), func(s *RBACService, a *UserAccess) error {
			return s.SetRoleSettings(a, fxUser, nil)
		}, true},
		{"settings: allow what the actor lacks", nil, func(s *RBACService, a *UserAccess) error {
			return s.SetRoleSettings(a, fxNoFile, []PermissionSetting{{PermissionID: fxEdit, Effect: allow}})
		}, true},
		{"settings: allow what the actor holds, deny anything", nil, func(s *RBACService, a *UserAccess) error {
			return s.SetRoleSettings(a, fxNoFile, []PermissionSetting{{PermissionID: fxView, Effect: allow}, {PermissionID: fxEdit, Effect: deny}})
		}, false},
		{"settings: keep an allow the role already has", nil, func(s *RBACService, a *UserAccess) error {
			return s.SetRoleSettings(a, fxUser, []PermissionSetting{{PermissionID: fxView, Effect: allow}, {PermissionID: fxSize, Effect: allow}})
		}, false},
		{"update: own role", nil, func(s *RBACService, a *UserAccess) error {
			_, err := s.UpdateRole(a, fxModerator, RoleInput{Name: "moderator"})
			return err
		}, true},
		{"update: add a parent beyond the actor", nil, func(s *RBACService, a *UserAccess) error {
			_, err := s.UpdateRole(a, fxNoFile, RoleInput{Name: "nofile", ParentIDs: []uint{fxUser}})
			return err
		}, true},
		{"update: add the superuser role as a parent", nil, func(s *RBACService, a *UserAccess) error {
			_, err := s.UpdateRole(a, fxNoFile, RoleInput{Name: "nofile", ParentIDs: []uint{fxAdmin}})
			return err
		}, true},
		{"update: add a parent within the actor", nil, func(s *RBACService, a *UserAccess) error {
			_, err := s.UpdateRole(a, fxNoFile, RoleInput{Name: "nofile", ParentIDs: []uint{fxModerator}})
			return err
		}, false},
		{"update: keep existing parents beyond the actor", nil, func(s *RBACService, a *UserAccess) error {
			_, err := s.UpdateRole(a, fxChild, RoleInput{Name: "child", Description: "renamed", ParentIDs: []uint{fxUser, fxNoFile}})
			return err
		}, false},
		{"create: parent beyond the actor", nil, func(s *RBACService, a *UserAccess) error {
			_, err := s.CreateRole(a, RoleInput{Name: "viewer-plus", ParentIDs: []uint{fxUser}})
			return err
		}, true},
		{"create: parent within the actor", nil, func(s *RBACService, a *UserAccess) error {
			_, err := s.CreateRole(a, RoleInput{Name: "viewer-plus", ParentIDs: []uint{fxModerator}})
			return err
		}, false},
		{"delete: own role", nil, func(s *RBACService, a *UserAccess) error {
			return s.DeleteRole(a, fxModerator)
		}, true},
		{"delete: an ancestor of the own role", NewUserAccess(3, "child", false, PermRolesManage), func(s *RBACService, a *UserAccess) error {
			return s.DeleteRole(a, fxNoFile)
		}, true},
		{"delete: another role", nil, func(s *RBACService, a *UserAccess) error {
			return s.DeleteRole(a, fxNoFile)
		}, false},
		{"superuser actor edits anything", NewUserAccess(1, "admin", true), func(s *RBACService, a *UserAccess) error {
			if _, err := s.UpdateRole(a, fxNoFile, RoleInput{Name: "nofile", ParentIDs: []uint{fxUser}}); err != nil {
				return err
			}
			return s.SetRoleSettings(a, fxNoFile, []PermissionSetting{{PermissionID: fxEdit, Effect: allow}})
		}, false},
		{"no actor", nil, func(s *RBACService, _ *UserAccess) error {
			return s.DeleteRole(nil, fxNoFile)
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, svc, _, actor := delegationFixture(t)
			if tc.actor != nil {
				actor = tc.actor
			}
			err := tc.run(svc, actor)
			if tc.forbidden && !apperrors.IsForbidden(err) {
				t.Fatalf("expected 403, got %v", err)
			}
			if !tc.forbidden && err != nil {
				t.Fatalf("expected success, got %v", err)
			}
		})
	}
}
