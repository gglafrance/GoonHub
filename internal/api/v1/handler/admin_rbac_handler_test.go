package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

// fakeRBAC is an RBACManager returning canned results and recording what it got.
type fakeRBAC struct {
	err          error
	gotInput     core.RoleInput
	gotSettings  []core.PermissionSetting
	gotOverrides []core.PermissionSetting
}

var _ RBACManager = (*core.RBACService)(nil)

func (f *fakeRBAC) ListPermissions() []core.PermissionView {
	return []core.PermissionView{{ID: 1, Name: "scenes:view", Group: "Scenes", Description: "Browse"}}
}
func (f *fakeRBAC) ListRoles() ([]core.RoleView, error) {
	return []core.RoleView{{ID: 1, Name: "admin", IsSystem: true, IsSuperuser: true, ParentIDs: []uint{}, Settings: []core.PermissionSetting{}, UserCount: 1}}, f.err
}
func (f *fakeRBAC) GetRole(id uint) (*core.RoleView, error) {
	return &core.RoleView{ID: id, Name: "r", ParentIDs: []uint{}, Settings: f.gotSettings}, nil
}
func (f *fakeRBAC) CreateRole(_ *core.UserAccess, in core.RoleInput) (*core.RoleView, error) {
	f.gotInput = in
	if f.err != nil {
		return nil, f.err
	}
	return &core.RoleView{ID: 9, Name: in.Name, ParentIDs: in.ParentIDs, Settings: []core.PermissionSetting{}}, nil
}
func (f *fakeRBAC) UpdateRole(_ *core.UserAccess, id uint, in core.RoleInput) (*core.RoleView, error) {
	f.gotInput = in
	if f.err != nil {
		return nil, f.err
	}
	return &core.RoleView{ID: id, Name: in.Name, ParentIDs: in.ParentIDs, Settings: []core.PermissionSetting{}}, nil
}
func (f *fakeRBAC) DeleteRole(*core.UserAccess, uint) error { return f.err }
func (f *fakeRBAC) SetRoleSettings(_ *core.UserAccess, _ uint, s []core.PermissionSetting) error {
	f.gotSettings = s
	return f.err
}
func (f *fakeRBAC) RoleEffective(uint) ([]core.EffectivePermission, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []core.EffectivePermission{{Name: "scenes:view", Granted: true, Source: "parent:user"}}, nil
}
func (f *fakeRBAC) UserPermissions(uint) (*core.UserPermissionsView, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &core.UserPermissionsView{Role: "user", Overrides: f.gotOverrides, Effective: []core.EffectivePermission{{Name: "scenes:view", Granted: true, Source: "role"}}}, nil
}
func (f *fakeRBAC) SetUserOverrides(_ *core.UserAccess, _ uint, o []core.PermissionSetting) error {
	f.gotOverrides = o
	return f.err
}

// newRBACAdminRouter mounts the RBAC admin routes the way routes.go does, for a user
// holding the given permissions.
func newRBACAdminRouter(fake *fakeRBAC, perms ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &AdminHandler{RBACService: fake}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "x"})
		middleware.SetAccess(c, core.NewUserAccess(1, "x", false, perms...))
		c.Next()
	})
	perm, anyPerm := middleware.RequirePermission, middleware.RequireAnyPermission
	admin := r.Group("/admin")
	admin.GET("/permissions", anyPerm(core.PermRolesManage, core.PermUsersManage), h.ListPermissions)
	admin.GET("/roles", anyPerm(core.PermRolesManage, core.PermUsersManage), h.ListRoles)
	admin.POST("/roles", perm(core.PermRolesManage), h.CreateRole)
	admin.PUT("/roles/:id", perm(core.PermRolesManage), h.UpdateRole)
	admin.DELETE("/roles/:id", perm(core.PermRolesManage), h.DeleteRole)
	admin.PUT("/roles/:id/permissions", perm(core.PermRolesManage), h.SetRolePermissions)
	admin.GET("/roles/:id/effective", perm(core.PermRolesManage), h.GetRoleEffective)
	admin.GET("/users/:id/permissions", perm(core.PermUsersManage), h.GetUserPermissions)
	admin.PUT("/users/:id/permissions", perm(core.PermUsersManage), h.SetUserPermissions)
	return r
}

func doRBACJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRBACAdmin_PermissionGates(t *testing.T) {
	tests := []struct {
		name   string
		perms  []string
		method string
		path   string
		body   string
		want   int
	}{
		{"roles list with users:manage", []string{core.PermUsersManage}, http.MethodGet, "/admin/roles", "", 200},
		{"permissions list with roles:manage", []string{core.PermRolesManage}, http.MethodGet, "/admin/permissions", "", 200},
		{"roles list without permission", nil, http.MethodGet, "/admin/roles", "", 403},
		{"create role needs roles:manage", []string{core.PermUsersManage}, http.MethodPost, "/admin/roles", `{"name":"guest"}`, 403},
		{"user permissions need users:manage", []string{core.PermRolesManage}, http.MethodGet, "/admin/users/2/permissions", "", 403},
		{"user permissions with users:manage", []string{core.PermUsersManage}, http.MethodGet, "/admin/users/2/permissions", "", 200},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doRBACJSON(newRBACAdminRouter(&fakeRBAC{}, tc.perms...), tc.method, tc.path, tc.body)
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestRBACAdmin_StatusCodes(t *testing.T) {
	inUse := &core.RoleInUseError{ConflictError: apperrors.NewConflictError("role", "role is held by 3 user(s)"), UserCount: 3}
	tests := []struct {
		name   string
		err    error
		method string
		path   string
		body   string
		want   int
	}{
		{"create", nil, http.MethodPost, "/admin/roles", `{"name":"guest","description":"d","parent_ids":[2]}`, 201},
		{"create without name", nil, http.MethodPost, "/admin/roles", `{"description":"d"}`, 400},
		{"create invalid name", apperrors.NewValidationError("bad name"), http.MethodPost, "/admin/roles", `{"name":"Bad"}`, 400},
		{"create duplicate", apperrors.NewConflictError("role", "exists"), http.MethodPost, "/admin/roles", `{"name":"user"}`, 409},
		{"update", nil, http.MethodPut, "/admin/roles/3", `{"name":"mod","parent_ids":[]}`, 200},
		{"update cycle", apperrors.NewValidationError("cycle"), http.MethodPut, "/admin/roles/3", `{"name":"mod","parent_ids":[4]}`, 400},
		{"update unknown", apperrors.NewNotFoundError("role", 404), http.MethodPut, "/admin/roles/404", `{"name":"x1"}`, 404},
		{"update bad id", nil, http.MethodPut, "/admin/roles/abc", `{"name":"x1"}`, 400},
		{"delete", nil, http.MethodDelete, "/admin/roles/3", "", 200},
		{"delete system", apperrors.NewValidationError("system"), http.MethodDelete, "/admin/roles/2", "", 400},
		{"delete in use", inUse, http.MethodDelete, "/admin/roles/3", "", 409},
		{"settings", nil, http.MethodPut, "/admin/roles/3/permissions", `{"settings":[{"permission_id":1,"effect":"deny"}]}`, 200},
		{"settings bad effect", nil, http.MethodPut, "/admin/roles/3/permissions", `{"settings":[{"permission_id":1,"effect":"grant"}]}`, 400},
		{"settings missing", nil, http.MethodPut, "/admin/roles/3/permissions", `{}`, 400},
		{"settings on superuser", apperrors.NewValidationError("superuser"), http.MethodPut, "/admin/roles/1/permissions", `{"settings":[]}`, 400},
		{"settings beyond the actor", apperrors.NewForbiddenError("you do not have it"), http.MethodPut, "/admin/roles/3/permissions", `{"settings":[]}`, 403},
		{"update own role", apperrors.NewForbiddenError("own role"), http.MethodPut, "/admin/roles/3", `{"name":"mod"}`, 403},
		{"overrides on a superuser", apperrors.NewForbiddenError("superuser account"), http.MethodPut, "/admin/users/1/permissions", `{"overrides":[]}`, 403},
		{"effective", nil, http.MethodGet, "/admin/roles/3/effective", "", 200},
		{"effective unknown", apperrors.NewNotFoundError("role", 9), http.MethodGet, "/admin/roles/9/effective", "", 404},
		{"user permissions unknown", apperrors.NewNotFoundError("user", 9), http.MethodGet, "/admin/users/9/permissions", "", 404},
		{"set overrides", nil, http.MethodPut, "/admin/users/2/permissions", `{"overrides":[{"permission_id":4,"effect":"allow"}]}`, 200},
		{"set overrides missing", nil, http.MethodPut, "/admin/users/2/permissions", `{}`, 400},
		{"set overrides unknown permission", apperrors.NewValidationError("unknown permission"), http.MethodPut, "/admin/users/2/permissions", `{"overrides":[{"permission_id":99,"effect":"allow"}]}`, 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRBACAdminRouter(&fakeRBAC{err: tc.err}, core.PermRolesManage, core.PermUsersManage)
			w := doRBACJSON(r, tc.method, tc.path, tc.body)
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestRBACAdmin_DeleteInUseReturnsCount(t *testing.T) {
	inUse := &core.RoleInUseError{ConflictError: apperrors.NewConflictError("role", "held"), UserCount: 3}
	w := doRBACJSON(newRBACAdminRouter(&fakeRBAC{err: inUse}, core.PermRolesManage), http.MethodDelete, "/admin/roles/3", "")
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["user_count"] != float64(3) || body["code"] != "ROLE_IN_USE" {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestRBACAdmin_BodiesReachTheService(t *testing.T) {
	fake := &fakeRBAC{}
	r := newRBACAdminRouter(fake, core.PermRolesManage, core.PermUsersManage)

	w := doRBACJSON(r, http.MethodPost, "/admin/roles", `{"name":"guest","description":"Guests","parent_ids":[2,4]}`)
	if w.Code != 201 || fake.gotInput.Name != "guest" || fake.gotInput.Description != "Guests" || fmt.Sprint(fake.gotInput.ParentIDs) != "[2 4]" {
		t.Fatalf("create: %d %+v", w.Code, fake.gotInput)
	}

	w = doRBACJSON(r, http.MethodPut, "/admin/roles/3/permissions", `{"settings":[{"permission_id":1,"effect":"allow"},{"permission_id":5,"effect":"deny"}]}`)
	if w.Code != 200 || len(fake.gotSettings) != 2 || fake.gotSettings[1] != (core.PermissionSetting{PermissionID: 5, Effect: "deny"}) {
		t.Fatalf("settings: %d %+v", w.Code, fake.gotSettings)
	}
	var role core.RoleView
	if err := json.Unmarshal(w.Body.Bytes(), &role); err != nil || len(role.Settings) != 2 {
		t.Fatalf("settings response: %s", w.Body.String())
	}

	w = doRBACJSON(r, http.MethodPut, "/admin/users/2/permissions", `{"overrides":[]}`)
	if w.Code != 200 || fake.gotOverrides == nil || len(fake.gotOverrides) != 0 {
		t.Fatalf("clear overrides: %d %+v", w.Code, fake.gotOverrides)
	}
	var view map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"role", "overrides", "effective"} {
		if _, ok := view[k]; !ok {
			t.Fatalf("missing %q in %s", k, w.Body.String())
		}
	}
}

func TestRBACAdmin_ListShapes(t *testing.T) {
	r := newRBACAdminRouter(&fakeRBAC{}, core.PermRolesManage)
	w := doRBACJSON(r, http.MethodGet, "/admin/permissions", "")
	if !strings.Contains(w.Body.String(), `"permissions":[{"id":1,"name":"scenes:view","group":"Scenes","description":"Browse"}]`) {
		t.Fatalf("permissions: %s", w.Body.String())
	}
	w = doRBACJSON(r, http.MethodGet, "/admin/roles", "")
	for _, k := range []string{`"is_system":true`, `"is_superuser":true`, `"parent_ids":[]`, `"settings":[]`, `"user_count":1`} {
		if !strings.Contains(w.Body.String(), k) {
			t.Fatalf("roles: missing %s in %s", k, w.Body.String())
		}
	}
	w = doRBACJSON(r, http.MethodGet, "/admin/roles/3/effective", "")
	if !strings.Contains(w.Body.String(), `{"permissions":[{"name":"scenes:view","granted":true,"source":"parent:user"}]}`) {
		t.Fatalf("effective: %s", w.Body.String())
	}
}

func TestRefuseHiddenSizeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		vis      response.FileVisibility
		sort     string
		filtered bool
		refused  bool
	}{
		{response.FileVisibility{}, "size_desc", false, true},
		{response.FileVisibility{}, "size_asc", false, true},
		{response.FileVisibility{}, "", true, true},
		{response.FileVisibility{}, "created_at_desc", false, false},
		{response.FileVisibility{Size: true}, "size_desc", true, false},
	}
	for _, tc := range tests {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		if got := refuseHiddenSizeQuery(c, tc.vis, tc.sort, tc.filtered); got != tc.refused {
			t.Fatalf("%+v: refused=%v", tc, got)
		}
		if tc.refused && w.Code != http.StatusForbidden {
			t.Fatalf("%+v: status %d", tc, w.Code)
		}
	}
}
