package core

import (
	"errors"
	"sync"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// rbacFixture backs the RBAC repository mocks with in-memory data that tests can change.
type rbacFixture struct {
	mu        sync.Mutex
	graph     data.RBACGraph
	perms     []data.Permission
	users     map[uint]*data.User
	overrides map[uint][]data.UserPermissionOverride
	userLoads int

	roleRepo *mocks.MockRoleRepository
	permRepo *mocks.MockPermissionRepository
	userRepo *mocks.MockUserRepository
}

// Permission IDs of the fixture.
const (
	fxView uint = iota + 1
	fxUpload
	fxEdit
	fxSize
	fxPaths
	fxUsersCreate
)

// Role IDs of the fixture: admin (superuser), user, moderator, nofile (denies files:*),
// child (parents: user, nofile).
const (
	fxAdmin uint = iota + 1
	fxUser
	fxModerator
	fxNoFile
	fxChild
)

func newRBACFixture(t *testing.T) *rbacFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &rbacFixture{
		graph: data.RBACGraph{
			Roles: []data.Role{
				{ID: fxAdmin, Name: "admin", IsSystem: true, IsSuperuser: true},
				{ID: fxUser, Name: "user", IsSystem: true},
				{ID: fxModerator, Name: "moderator"},
				{ID: fxNoFile, Name: "nofile"},
				{ID: fxChild, Name: "child"},
			},
			Parents: []data.RoleParent{
				{RoleID: fxChild, ParentID: fxUser},
				{RoleID: fxChild, ParentID: fxNoFile},
			},
			Settings: []data.RolePermission{
				{RoleID: fxUser, PermissionID: fxView, Effect: data.PermissionEffectAllow},
				{RoleID: fxUser, PermissionID: fxSize, Effect: data.PermissionEffectAllow},
				{RoleID: fxModerator, PermissionID: fxView, Effect: data.PermissionEffectAllow},
				{RoleID: fxModerator, PermissionID: fxUpload, Effect: data.PermissionEffectAllow},
				{RoleID: fxNoFile, PermissionID: fxSize, Effect: data.PermissionEffectDeny},
				{RoleID: fxNoFile, PermissionID: fxPaths, Effect: data.PermissionEffectDeny},
			},
		},
		perms: []data.Permission{
			{ID: fxView, Name: PermScenesView},
			{ID: fxUpload, Name: PermScenesUpload},
			{ID: fxEdit, Name: PermScenesEdit},
			{ID: fxSize, Name: PermFilesViewSize},
			{ID: fxPaths, Name: PermFilesViewPaths},
			{ID: fxUsersCreate, Name: "users:create"},
		},
		users: map[uint]*data.User{
			1: {ID: 1, Username: "root", Role: "admin"},
			2: {ID: 2, Username: "viewer", Role: "user"},
			3: {ID: 3, Username: "kid", Role: "child"},
		},
		overrides: map[uint][]data.UserPermissionOverride{},
		roleRepo:  mocks.NewMockRoleRepository(ctrl),
		permRepo:  mocks.NewMockPermissionRepository(ctrl),
		userRepo:  mocks.NewMockUserRepository(ctrl),
	}
	f.roleRepo.EXPECT().LoadGraph().DoAndReturn(func() (*data.RBACGraph, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		g := data.RBACGraph{
			Roles:    append([]data.Role{}, f.graph.Roles...),
			Parents:  append([]data.RoleParent{}, f.graph.Parents...),
			Settings: append([]data.RolePermission{}, f.graph.Settings...),
		}
		return &g, nil
	}).AnyTimes()
	f.permRepo.EXPECT().List().DoAndReturn(func() ([]data.Permission, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		return append([]data.Permission{}, f.perms...), nil
	}).AnyTimes()
	f.userRepo.EXPECT().GetByID(gomock.Any()).DoAndReturn(func(id uint) (*data.User, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.userLoads++
		u, ok := f.users[id]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		cp := *u
		return &cp, nil
	}).AnyTimes()
	f.permRepo.EXPECT().ListUserOverrides(gomock.Any()).DoAndReturn(func(id uint) ([]data.UserPermissionOverride, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		return append([]data.UserPermissionOverride{}, f.overrides[id]...), nil
	}).AnyTimes()
	return f
}

func (f *rbacFixture) service(t *testing.T) *RBACService {
	t.Helper()
	svc, err := NewRBACService(f.roleRepo, f.permRepo, f.userRepo, zap.NewNop())
	if err != nil {
		t.Fatalf("NewRBACService: %v", err)
	}
	return svc
}

func (f *rbacFixture) setOverrides(userID uint, o ...data.UserPermissionOverride) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.overrides[userID] = o
}

// TestRBACResolution checks the resolution rules of docs/RBAC.md on a role graph.
func TestRBACResolution(t *testing.T) {
	allow, deny := data.PermissionEffectAllow, data.PermissionEffectDeny
	const (
		pA uint = 1
		pB uint = 2
	)
	perms := []data.Permission{{ID: pA, Name: "a:x"}, {ID: pB, Name: "b:x"}}
	type setting struct {
		role   uint
		perm   uint
		effect string
	}
	tests := []struct {
		name      string
		roles     []data.Role
		parents   [][2]uint // role, parent
		settings  []setting
		overrides map[uint]string
		roleID    uint
		perm      uint
		granted   bool
		source    string
	}{
		{
			name:     "own allow",
			roles:    []data.Role{{ID: 1, Name: "r"}},
			settings: []setting{{1, pA, allow}},
			roleID:   1, perm: pA, granted: true, source: PermSourceRole,
		},
		{
			name:   "nothing set",
			roles:  []data.Role{{ID: 1, Name: "r"}},
			roleID: 1, perm: pA, granted: false, source: PermSourceNone,
		},
		{
			name:     "own allow beats a parent deny",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "p"}},
			parents:  [][2]uint{{1, 2}},
			settings: []setting{{1, pA, allow}, {2, pA, deny}},
			roleID:   1, perm: pA, granted: true, source: PermSourceRole,
		},
		{
			name:     "own deny beats a parent allow",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "p"}},
			parents:  [][2]uint{{1, 2}},
			settings: []setting{{1, pA, deny}, {2, pA, allow}},
			roleID:   1, perm: pA, granted: false, source: PermSourceRole,
		},
		{
			name:     "inherited allow",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "p"}},
			parents:  [][2]uint{{1, 2}},
			settings: []setting{{2, pA, allow}},
			roleID:   1, perm: pA, granted: true, source: "parent:p",
		},
		{
			name:     "deny wins among parents",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "viewer"}, {ID: 3, Name: "no-file-info"}},
			parents:  [][2]uint{{1, 2}, {1, 3}},
			settings: []setting{{2, pA, allow}, {3, pA, deny}},
			roleID:   1, perm: pA, granted: false, source: "parent:no-file-info",
		},
		{
			name:     "a parent without setting does not block another one's allow",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "viewer"}, {ID: 3, Name: "no-file-info"}},
			parents:  [][2]uint{{1, 2}, {1, 3}},
			settings: []setting{{2, pA, allow}, {3, pB, deny}},
			roleID:   1, perm: pA, granted: true, source: "parent:viewer",
		},
		{
			name:     "multi-level inheritance reports the deciding ancestor",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "mid"}, {ID: 3, Name: "top"}},
			parents:  [][2]uint{{1, 2}, {2, 3}},
			settings: []setting{{3, pA, allow}},
			roleID:   1, perm: pA, granted: true, source: "parent:top",
		},
		{
			name:     "an intermediate role's own setting overrides higher ancestors",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "mid"}, {ID: 3, Name: "top"}},
			parents:  [][2]uint{{1, 2}, {2, 3}},
			settings: []setting{{3, pA, allow}, {2, pA, deny}},
			roleID:   1, perm: pA, granted: false, source: "parent:mid",
		},
		{
			name:     "deep deny wins over a direct allow among parents",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "a"}, {ID: 3, Name: "b"}, {ID: 4, Name: "deep"}},
			parents:  [][2]uint{{1, 2}, {1, 3}, {3, 4}},
			settings: []setting{{2, pA, allow}, {4, pA, deny}},
			roleID:   1, perm: pA, granted: false, source: "parent:deep",
		},
		{
			name:      "user override allow beats the role deny",
			roles:     []data.Role{{ID: 1, Name: "r"}},
			settings:  []setting{{1, pA, deny}},
			overrides: map[uint]string{pA: allow},
			roleID:    1, perm: pA, granted: true, source: PermSourceOverride,
		},
		{
			name:      "user override deny beats an inherited allow",
			roles:     []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "p"}},
			parents:   [][2]uint{{1, 2}},
			settings:  []setting{{2, pA, allow}},
			overrides: map[uint]string{pA: deny},
			roleID:    1, perm: pA, granted: false, source: PermSourceOverride,
		},
		{
			name:      "superuser role ignores overrides",
			roles:     []data.Role{{ID: 1, Name: "admin", IsSuperuser: true}},
			overrides: map[uint]string{pA: deny},
			roleID:    1, perm: pA, granted: true, source: PermSourceSuperuser,
		},
		{
			name:     "a superuser parent allows, an own deny still wins",
			roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "admin", IsSuperuser: true}},
			parents:  [][2]uint{{1, 2}},
			settings: []setting{{1, pB, deny}},
			roleID:   1, perm: pA, granted: true, source: "parent:admin",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := &data.RBACGraph{Roles: tc.roles}
			for _, p := range tc.parents {
				g.Parents = append(g.Parents, data.RoleParent{RoleID: p[0], ParentID: p[1]})
			}
			for _, s := range tc.settings {
				g.Settings = append(g.Settings, data.RolePermission{RoleID: s.role, PermissionID: s.perm, Effect: s.effect})
			}
			rg := newRBACGraph(g, perms)
			granted, source := rg.userDecision(tc.roleID, tc.overrides, tc.perm)
			if granted != tc.granted || source != tc.source {
				t.Fatalf("got granted=%v source=%q, want granted=%v source=%q", granted, source, tc.granted, tc.source)
			}
		})
	}
}

func TestRBACResolution_OwnDenyWithSuperuserParent(t *testing.T) {
	g := &data.RBACGraph{
		Roles:    []data.Role{{ID: 1, Name: "r"}, {ID: 2, Name: "admin", IsSuperuser: true}},
		Parents:  []data.RoleParent{{RoleID: 1, ParentID: 2}},
		Settings: []data.RolePermission{{RoleID: 1, PermissionID: 7, Effect: data.PermissionEffectDeny}},
	}
	rg := newRBACGraph(g, []data.Permission{{ID: 7, Name: "x:y"}})
	if granted, source := rg.userDecision(1, nil, 7); granted || source != PermSourceRole {
		t.Fatalf("own deny under a superuser parent: got %v %q", granted, source)
	}
}

func TestRBACGraph_CreatesCycle(t *testing.T) {
	// 1 <- 2 <- 3 (3 has parent 2, 2 has parent 1)
	g := newRBACGraph(&data.RBACGraph{
		Roles:   []data.Role{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}, {ID: 3, Name: "c"}, {ID: 4, Name: "d"}},
		Parents: []data.RoleParent{{RoleID: 2, ParentID: 1}, {RoleID: 3, ParentID: 2}},
	}, nil)
	tests := []struct {
		role    uint
		parents []uint
		cycle   bool
	}{
		{1, []uint{3}, true},  // a's parent c, whose ancestors contain a
		{1, []uint{2}, true},  // direct
		{1, []uint{1}, true},  // self
		{3, []uint{1}, false}, // a is already an ancestor: no cycle
		{1, []uint{4}, false},
		{4, []uint{3, 1}, false},
	}
	for _, tc := range tests {
		if got := g.createsCycle(tc.role, tc.parents); got != tc.cycle {
			t.Fatalf("createsCycle(%d, %v) = %v, want %v", tc.role, tc.parents, got, tc.cycle)
		}
	}
}

func TestResolveUser_LiveRoleAndPermissions(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	access, err := svc.ResolveUser(2)
	if err != nil {
		t.Fatalf("ResolveUser: %v", err)
	}
	if access.Role != "user" || access.Superuser {
		t.Fatalf("unexpected access %+v", access)
	}
	if !access.Has(PermScenesView) || !access.Has(PermFilesViewSize) || access.Has(PermScenesUpload) {
		t.Fatalf("unexpected permissions %v", access.Permissions())
	}

	// the user is moved to moderator: the next resolution after invalidation sees it
	f.mu.Lock()
	f.users[2].Role = "moderator"
	f.mu.Unlock()
	svc.InvalidateUser(2)
	access, err = svc.ResolveUser(2)
	if err != nil {
		t.Fatalf("ResolveUser: %v", err)
	}
	if access.Role != "moderator" || !access.Has(PermScenesUpload) || access.Has(PermFilesViewSize) {
		t.Fatalf("role change not applied: %s %v", access.Role, access.Permissions())
	}
}

func TestResolveUser_DeniedByParentMixin(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	access, err := svc.ResolveUser(3)
	if err != nil {
		t.Fatalf("ResolveUser: %v", err)
	}
	if !access.Has(PermScenesView) {
		t.Fatal("child should inherit scenes:view from user")
	}
	if access.Has(PermFilesViewSize) {
		t.Fatal("nofile denies files:view_size: deny wins among parents")
	}
}

func TestResolveUser_SuperuserHasEverything(t *testing.T) {
	f := newRBACFixture(t)
	f.setOverrides(1, data.UserPermissionOverride{UserID: 1, PermissionID: fxView, Effect: data.PermissionEffectDeny})
	svc := f.service(t)

	access, err := svc.ResolveUser(1)
	if err != nil {
		t.Fatalf("ResolveUser: %v", err)
	}
	if !access.Superuser || !access.Has(PermScenesView) || !access.Has("anything:else") {
		t.Fatalf("superuser should have every permission: %+v", access)
	}
	for _, p := range access.Permissions() {
		if DeprecatedPermissions[p] {
			t.Fatalf("deprecated permission %s listed", p)
		}
	}
}

func TestResolveUser_OverridesAndCache(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	if _, err := svc.ResolveUser(2); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResolveUser(2); err != nil {
		t.Fatal(err)
	}
	if f.userLoads != 1 {
		t.Fatalf("expected the access to be cached, user loaded %d times", f.userLoads)
	}

	f.setOverrides(2,
		data.UserPermissionOverride{UserID: 2, PermissionID: fxSize, Effect: data.PermissionEffectDeny},
		data.UserPermissionOverride{UserID: 2, PermissionID: fxEdit, Effect: data.PermissionEffectAllow},
	)
	f.permRepo.EXPECT().ReplaceUserOverrides(uint(2), gomock.Any()).Return(nil)
	if err := svc.SetUserOverrides(2, []PermissionSetting{
		{PermissionID: fxSize, Effect: data.PermissionEffectDeny},
		{PermissionID: fxEdit, Effect: data.PermissionEffectAllow},
	}); err != nil {
		t.Fatalf("SetUserOverrides: %v", err)
	}
	access, err := svc.ResolveUser(2)
	if err != nil {
		t.Fatal(err)
	}
	if access.Has(PermFilesViewSize) || !access.Has(PermScenesEdit) {
		t.Fatalf("overrides not applied: %v", access.Permissions())
	}

	// the cache expires on its own after the TTL
	loads := f.userLoads
	svc.now = func() time.Time { return time.Now().Add(2 * userAccessTTL) }
	if _, err := svc.ResolveUser(2); err != nil {
		t.Fatal(err)
	}
	if f.userLoads != loads+1 {
		t.Fatal("expected a reload after the TTL")
	}
}

func TestResolveUser_DeletedUserIsNotFound(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	_, err := svc.ResolveUser(99)
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
	if svc.UserHasPermission(99, PermScenesView) {
		t.Fatal("a deleted user has no permission")
	}
}

func TestResolveUser_RoleChangeInvalidatesEveryUser(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	if access, _ := svc.ResolveUser(2); access.Has(PermScenesEdit) {
		t.Fatal("user should not edit yet")
	}
	f.mu.Lock()
	f.graph.Settings = append(f.graph.Settings, data.RolePermission{RoleID: fxUser, PermissionID: fxEdit, Effect: data.PermissionEffectAllow})
	f.mu.Unlock()
	f.roleRepo.EXPECT().ReplaceSettings(fxUser, gomock.Any()).Return(nil)
	if err := svc.SetRoleSettings(fxUser, []PermissionSetting{
		{PermissionID: fxView, Effect: data.PermissionEffectAllow},
		{PermissionID: fxSize, Effect: data.PermissionEffectAllow},
		{PermissionID: fxEdit, Effect: data.PermissionEffectAllow},
	}); err != nil {
		t.Fatalf("SetRoleSettings: %v", err)
	}
	if access, _ := svc.ResolveUser(2); !access.Has(PermScenesEdit) {
		t.Fatal("role settings change not applied to the user")
	}
	// child inherits it from user
	if access, _ := svc.ResolveUser(3); !access.Has(PermScenesEdit) {
		t.Fatal("role settings change not inherited by child")
	}
}

func TestResolveUser_Concurrent(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%5 == 0 {
				if err := svc.RefreshCache(); err != nil {
					t.Error(err)
				}
				return
			}
			if i%3 == 0 {
				svc.InvalidateUser(2)
			}
			if _, err := svc.ResolveUser(2); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
}

func TestRoleEffective_Sources(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	perms, err := svc.RoleEffective(fxChild)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]EffectivePermission{}
	for _, p := range perms {
		got[p.Name] = p
	}
	if p := got[PermScenesView]; !p.Granted || p.Source != "parent:user" {
		t.Fatalf("scenes:view = %+v", p)
	}
	if p := got[PermFilesViewSize]; p.Granted || p.Source != "parent:nofile" {
		t.Fatalf("files:view_size = %+v", p)
	}
	if p := got[PermScenesEdit]; p.Granted || p.Source != PermSourceNone {
		t.Fatalf("scenes:edit = %+v", p)
	}
	if _, ok := got["users:create"]; ok {
		t.Fatal("deprecated permission listed")
	}
	admin, _ := svc.RoleEffective(fxAdmin)
	for _, p := range admin {
		if !p.Granted || p.Source != PermSourceSuperuser {
			t.Fatalf("admin %s = %+v", p.Name, p)
		}
	}
	if _, err := svc.RoleEffective(404); !apperrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestUserPermissions_View(t *testing.T) {
	f := newRBACFixture(t)
	f.setOverrides(2, data.UserPermissionOverride{UserID: 2, PermissionID: fxUpload, Effect: data.PermissionEffectAllow})
	svc := f.service(t)

	view, err := svc.UserPermissions(2)
	if err != nil {
		t.Fatal(err)
	}
	if view.Role != "user" || len(view.Overrides) != 1 || view.Overrides[0].PermissionID != fxUpload {
		t.Fatalf("unexpected view %+v", view)
	}
	for _, p := range view.Effective {
		switch p.Name {
		case PermScenesUpload:
			if !p.Granted || p.Source != PermSourceOverride {
				t.Fatalf("upload = %+v", p)
			}
		case PermScenesView:
			if !p.Granted || p.Source != PermSourceRole {
				t.Fatalf("view = %+v", p)
			}
		}
	}
	if _, err := svc.UserPermissions(99); !apperrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestSetUserOverrides_Validation(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)
	tests := []struct {
		name string
		in   []PermissionSetting
	}{
		{"unknown permission", []PermissionSetting{{PermissionID: 404, Effect: "allow"}}},
		{"deprecated permission", []PermissionSetting{{PermissionID: fxUsersCreate, Effect: "allow"}}},
		{"bad effect", []PermissionSetting{{PermissionID: fxView, Effect: "maybe"}}},
		{"duplicate", []PermissionSetting{{PermissionID: fxView, Effect: "allow"}, {PermissionID: fxView, Effect: "deny"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.SetUserOverrides(2, tc.in); !apperrors.IsValidation(err) {
				t.Fatalf("expected a validation error, got %v", err)
			}
		})
	}
	if err := svc.SetUserOverrides(99, nil); !apperrors.IsNotFound(err) {
		t.Fatalf("expected NotFound for an unknown user, got %v", err)
	}
}

func TestCreateRole(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	tests := []struct {
		name  string
		in    RoleInput
		check func(error) bool
	}{
		{"invalid name", RoleInput{Name: "Bad Name"}, apperrors.IsValidation},
		{"too short", RoleInput{Name: "a"}, apperrors.IsValidation},
		{"duplicate name", RoleInput{Name: "moderator"}, apperrors.IsConflict},
		{"unknown parent", RoleInput{Name: "viewer-plus", ParentIDs: []uint{404}}, apperrors.IsValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.CreateRole(tc.in); !tc.check(err) {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}

	f.roleRepo.EXPECT().Create(gomock.Any(), []uint{fxUser, fxNoFile}).DoAndReturn(func(r *data.Role, parents []uint) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		r.ID = 10
		f.graph.Roles = append(f.graph.Roles, *r)
		for _, p := range parents {
			f.graph.Parents = append(f.graph.Parents, data.RoleParent{RoleID: 10, ParentID: p})
		}
		return nil
	})
	f.roleRepo.EXPECT().CountUsersByRole("restricted").Return(int64(0), nil)
	view, err := svc.CreateRole(RoleInput{Name: "restricted", Description: " Viewer without file info ", ParentIDs: []uint{fxNoFile, fxUser, fxUser}})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if view.ID != 10 || view.Description != "Viewer without file info" || len(view.ParentIDs) != 2 || len(view.Settings) != 0 {
		t.Fatalf("unexpected role %+v", view)
	}
}

func TestUpdateRole(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	// cycle: user gets child (whose parent is user) as a parent
	if _, err := svc.UpdateRole(fxUser, RoleInput{Name: "user", ParentIDs: []uint{fxChild}}); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error for a cycle, got %v", err)
	}
	if _, err := svc.UpdateRole(fxModerator, RoleInput{Name: "moderator", ParentIDs: []uint{fxModerator}}); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error for a self parent, got %v", err)
	}
	if _, err := svc.UpdateRole(fxAdmin, RoleInput{Name: "root"}); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error for renaming the superuser role, got %v", err)
	}
	if _, err := svc.UpdateRole(fxUser, RoleInput{Name: "viewer"}); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error for renaming a system role, got %v", err)
	}
	if _, err := svc.UpdateRole(fxModerator, RoleInput{Name: "user"}); !apperrors.IsConflict(err) {
		t.Fatalf("expected a conflict for a taken name, got %v", err)
	}
	if _, err := svc.UpdateRole(404, RoleInput{Name: "ghost"}); !apperrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}

	f.roleRepo.EXPECT().Update(gomock.Any(), []uint{fxNoFile}).DoAndReturn(func(r *data.Role, parents []uint) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		for i := range f.graph.Roles {
			if f.graph.Roles[i].ID == r.ID {
				f.graph.Roles[i].Name = r.Name
			}
		}
		f.graph.Parents = append(f.graph.Parents, data.RoleParent{RoleID: r.ID, ParentID: fxNoFile})
		return nil
	})
	f.roleRepo.EXPECT().CountUsersByRole("mod").Return(int64(3), nil)
	view, err := svc.UpdateRole(fxModerator, RoleInput{Name: "mod", ParentIDs: []uint{fxNoFile}})
	if err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if view.Name != "mod" || view.UserCount != 3 || len(view.ParentIDs) != 1 {
		t.Fatalf("unexpected role %+v", view)
	}
}

func TestDeleteRole(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	if err := svc.DeleteRole(fxUser); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error for a system role, got %v", err)
	}
	if err := svc.DeleteRole(fxAdmin); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error for the superuser role, got %v", err)
	}
	if err := svc.DeleteRole(404); !apperrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}

	f.roleRepo.EXPECT().CountUsersByRole("moderator").Return(int64(2), nil)
	err := svc.DeleteRole(fxModerator)
	var inUse *RoleInUseError
	if !errors.As(err, &inUse) || inUse.UserCount != 2 || !apperrors.IsConflict(err) {
		t.Fatalf("expected RoleInUseError with 2 users, got %v", err)
	}

	f.roleRepo.EXPECT().CountUsersByRole("nofile").Return(int64(0), nil)
	f.roleRepo.EXPECT().Delete(fxNoFile).DoAndReturn(func(id uint) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		var roles []data.Role
		for _, r := range f.graph.Roles {
			if r.ID != id {
				roles = append(roles, r)
			}
		}
		f.graph.Roles = roles
		var parents []data.RoleParent
		for _, p := range f.graph.Parents {
			if p.ParentID != id && p.RoleID != id {
				parents = append(parents, p)
			}
		}
		f.graph.Parents = parents
		return nil
	})
	if err := svc.DeleteRole(fxNoFile); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	// child lost nofile as a parent: it gets files:view_size from user again
	if access, _ := svc.ResolveUser(3); !access.Has(PermFilesViewSize) {
		t.Fatal("child should no longer be denied files:view_size")
	}
}

func TestSetRoleSettings_Validation(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	if err := svc.SetRoleSettings(fxAdmin, nil); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error on the superuser role, got %v", err)
	}
	if err := svc.SetRoleSettings(404, nil); !apperrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
	if err := svc.SetRoleSettings(fxUser, []PermissionSetting{{PermissionID: fxView, Effect: "grant"}}); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error for a bad effect, got %v", err)
	}
}

func TestListRolesAndPermissions(t *testing.T) {
	f := newRBACFixture(t)
	svc := f.service(t)

	perms := svc.ListPermissions()
	// catalog order: scenes:view, scenes:upload, scenes:edit, files:view_size, files:view_paths
	want := []string{PermScenesView, PermScenesUpload, PermScenesEdit, PermFilesViewSize, PermFilesViewPaths}
	if len(perms) != len(want) {
		t.Fatalf("got %d permissions, want %d: %+v", len(perms), len(want), perms)
	}
	for i, p := range perms {
		if p.Name != want[i] {
			t.Fatalf("permission %d = %s, want %s", i, p.Name, want[i])
		}
	}

	f.roleRepo.EXPECT().CountUsersPerRole().Return(map[string]int64{"admin": 1, "user": 4}, nil)
	roles, err := svc.ListRoles()
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 5 || roles[0].Name != "admin" || !roles[0].IsSuperuser || roles[1].UserCount != 4 {
		t.Fatalf("unexpected roles %+v", roles)
	}
	child := roles[4]
	if len(child.ParentIDs) != 2 || child.ParentIDs[0] != fxUser || child.ParentIDs[1] != fxNoFile {
		t.Fatalf("unexpected child parents %+v", child.ParentIDs)
	}
	if len(roles[3].Settings) != 2 || roles[3].Settings[0].Effect != data.PermissionEffectDeny {
		t.Fatalf("unexpected nofile settings %+v", roles[3].Settings)
	}
}

// TestSyncCatalog checks that missing catalog permissions are inserted with their default
// grants (named roles, roles having the "from" permission allowed, never the superuser) and
// that existing ones only get their group and description updated.
func TestSyncCatalog(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)

	existing := []data.Permission{}
	nextID := uint(1)
	for _, c := range PermissionCatalog {
		switch c.Name {
		case PermScenesView, PermScenesUpload:
			existing = append(existing, data.Permission{ID: nextID, Name: c.Name, GroupName: c.Group, Description: c.Description})
			nextID++
		case PermUsersManage:
			// outdated group and description
			existing = append(existing, data.Permission{ID: nextID, Name: c.Name, Description: "Manage users"})
			nextID++
		}
	}
	existing = append(existing, data.Permission{ID: nextID, Name: "users:create", Description: "Create new users"})
	viewID := existing[0].ID
	usersManageID := existing[2].ID
	graph := &data.RBACGraph{
		Roles: []data.Role{
			{ID: 1, Name: "admin", IsSuperuser: true, IsSystem: true},
			{ID: 2, Name: "moderator"},
			{ID: 3, Name: "user", IsSystem: true},
			{ID: 4, Name: "guest"},
			{ID: 5, Name: "custom"},
		},
		Settings: []data.RolePermission{
			{RoleID: 1, PermissionID: viewID, Effect: data.PermissionEffectAllow},
			{RoleID: 2, PermissionID: viewID, Effect: data.PermissionEffectAllow},
			{RoleID: 3, PermissionID: viewID, Effect: data.PermissionEffectAllow},
			{RoleID: 4, PermissionID: viewID, Effect: data.PermissionEffectDeny},
			{RoleID: 5, PermissionID: viewID, Effect: data.PermissionEffectAllow},
		},
	}

	// initial load (constructor), then the sync, then the refresh at the end of the sync
	roleRepo.EXPECT().LoadGraph().Return(graph, nil).Times(3)
	permRepo.EXPECT().List().Return(existing, nil).Times(3)
	permRepo.EXPECT().UpdateMeta(usersManageID, PermGroupAdmin, gomock.Any()).Return(nil)

	created := map[string]uint{}
	permRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(p *data.Permission) error {
		nextID++
		p.ID = nextID
		created[p.Name] = p.ID
		if p.GroupName == "" {
			t.Errorf("permission %s created without a group", p.Name)
		}
		return nil
	}).Times(len(PermissionCatalog) - 3)

	grants := map[string][]uint{}
	byID := func(id uint) string {
		for name, cid := range created {
			if cid == id {
				return name
			}
		}
		return ""
	}
	permRepo.EXPECT().GrantToRoles(gomock.Any(), gomock.Any()).DoAndReturn(func(permID uint, roleIDs []uint) error {
		grants[byID(permID)] = roleIDs
		return nil
	}).Times(len(PermissionCatalog) - 3)

	svc, err := NewRBACService(roleRepo, permRepo, userRepo, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SyncCatalog(); err != nil {
		t.Fatalf("SyncCatalog: %v", err)
	}

	equal := func(a, b []uint) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	wantGrants := map[string][]uint{
		PermScenesEdit:     {2},
		PermTagsManage:     {2},
		PermFilesViewSize:  {2},
		PermFilesViewPaths: {2},
		PermFilesViewTech:  {2},
		PermExplorerAccess: {2},
		PermJobsViewStatus: {2},
		PermScenesShare:    {2, 3, 5}, // from scenes:view: not admin (superuser), not guest (deny)
		PermMarkersCreate:  {2, 3, 5},
		PermJobsManage:     nil,
		PermLibraryHealth:  nil,
		PermActorsManage:   nil,
	}
	for name, want := range wantGrants {
		if got := grants[name]; !equal(got, want) {
			t.Fatalf("grants of %s = %v, want %v", name, got, want)
		}
	}
	if _, ok := created["users:create"]; ok {
		t.Fatal("deprecated permission must not be recreated")
	}
}

func TestSyncCatalog_NothingMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)

	perms := make([]data.Permission, len(PermissionCatalog))
	for i, c := range PermissionCatalog {
		perms[i] = data.Permission{ID: uint(i + 1), Name: c.Name, GroupName: c.Group, Description: c.Description}
	}
	roleRepo.EXPECT().LoadGraph().Return(&data.RBACGraph{}, nil).AnyTimes()
	permRepo.EXPECT().List().Return(perms, nil).AnyTimes()
	// no Create, UpdateMeta nor GrantToRoles expected

	svc, err := NewRBACService(roleRepo, permRepo, mocks.NewMockUserRepository(ctrl), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SyncCatalog(); err != nil {
		t.Fatalf("SyncCatalog: %v", err)
	}
}

func TestRefreshCache_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	roleRepo.EXPECT().LoadGraph().Return(nil, errors.New("db down"))
	if _, err := NewRBACService(roleRepo, mocks.NewMockPermissionRepository(ctrl), mocks.NewMockUserRepository(ctrl), zap.NewNop()); err == nil {
		t.Fatal("expected an error")
	}
}
