package data

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Integration tests of the RBAC repositories against PostgreSQL (migration 094: role parents,
// allow/deny settings, user overrides, users.role foreign key). Needs
// GOONHUB_TEST_IMPORT_POSTGRES_DSN (disposable database).

func rbacTestRole(t *testing.T, db *gorm.DB, repo *RoleRepositoryImpl, name string, parents ...uint) *Role {
	t.Helper()
	role := &Role{Name: name, Description: "rbac integration test"}
	if err := repo.Create(role, parents); err != nil {
		t.Fatalf("create role %s: %v", name, err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM users WHERE role = ?", role.Name)
		db.Exec("DELETE FROM roles WHERE id = ?", role.ID)
	})
	return role
}

func rbacSuffix() string { return uuid.NewString()[:8] }

func TestRBACRepository_RolesParentsAndSettings(t *testing.T) {
	db := importTestDB(t)
	roles := NewRoleRepository(db)
	perms := NewPermissionRepository(db)
	sfx := rbacSuffix()

	base := rbacTestRole(t, db, roles, "base-"+sfx)
	mixin := rbacTestRole(t, db, roles, "mixin-"+sfx)
	child := rbacTestRole(t, db, roles, "child-"+sfx, base.ID, mixin.ID)

	perm := &Permission{Name: "test:" + sfx, GroupName: "Test", Description: "rbac integration test"}
	if err := perms.Create(perm); err != nil {
		t.Fatalf("create permission: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM permissions WHERE id = ?", perm.ID) })

	if err := roles.ReplaceSettings(base.ID, []RolePermission{{PermissionID: perm.ID, Effect: PermissionEffectAllow}}); err != nil {
		t.Fatalf("replace settings: %v", err)
	}
	if err := roles.ReplaceSettings(mixin.ID, []RolePermission{{PermissionID: perm.ID, Effect: PermissionEffectDeny}}); err != nil {
		t.Fatalf("replace settings: %v", err)
	}

	g, err := roles.LoadGraph()
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	var childParents []uint
	for _, p := range g.Parents {
		if p.RoleID == child.ID {
			childParents = append(childParents, p.ParentID)
		}
	}
	if len(childParents) != 2 || childParents[0] != base.ID || childParents[1] != mixin.ID {
		t.Fatalf("child parents = %v", childParents)
	}
	effects := map[uint]string{}
	for _, s := range g.Settings {
		if s.PermissionID == perm.ID {
			effects[s.RoleID] = s.Effect
		}
	}
	if effects[base.ID] != PermissionEffectAllow || effects[mixin.ID] != PermissionEffectDeny {
		t.Fatalf("settings = %v", effects)
	}

	// an invalid effect is refused by the CHECK constraint
	if err := roles.ReplaceSettings(base.ID, []RolePermission{{PermissionID: perm.ID, Effect: "maybe"}}); err == nil {
		t.Fatal("expected the effect check to refuse 'maybe'")
	}

	// GrantToRoles keeps an existing deny and adds the missing allow
	if err := perms.GrantToRoles(perm.ID, []uint{mixin.ID, child.ID}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	var rows []RolePermission
	db.Where("permission_id = ?", perm.ID).Order("role_id").Find(&rows)
	got := map[uint]string{}
	for _, r := range rows {
		got[r.RoleID] = r.Effect
	}
	if got[mixin.ID] != PermissionEffectDeny || got[child.ID] != PermissionEffectAllow {
		t.Fatalf("after grant = %v", got)
	}

	// Update: rename and replace the parents
	child.Name = "renamed-" + sfx
	if err := roles.Update(child, []uint{mixin.ID}); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, err := roles.GetByID(child.ID)
	if err != nil || reloaded.Name != child.Name {
		t.Fatalf("reloaded %+v, %v", reloaded, err)
	}
	g, _ = roles.LoadGraph()
	childParents = nil
	for _, p := range g.Parents {
		if p.RoleID == child.ID {
			childParents = append(childParents, p.ParentID)
		}
	}
	if len(childParents) != 1 || childParents[0] != mixin.ID {
		t.Fatalf("parents after update = %v", childParents)
	}

	// a role cannot be its own parent (CHECK)
	if err := roles.Update(child, []uint{child.ID}); err == nil {
		t.Fatal("expected the self-parent check to fail")
	}

	// deleting a parent removes the link
	if err := roles.Delete(mixin.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var links int64
	db.Model(&RoleParent{}).Where("parent_id = ?", mixin.ID).Count(&links)
	if links != 0 {
		t.Fatalf("%d links left to a deleted role", links)
	}
}

func TestRBACRepository_UsersRoleForeignKey(t *testing.T) {
	db := importTestDB(t)
	roles := NewRoleRepository(db)
	sfx := rbacSuffix()
	role := rbacTestRole(t, db, roles, "fk-"+sfx)

	user := &User{Username: "rbac-fk-" + sfx, Password: "x", Role: role.Name}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM users WHERE id = ?", user.ID) })

	if err := db.Create(&User{Username: "rbac-ghost-" + sfx, Password: "x", Role: "ghost-" + sfx}).Error; err == nil {
		t.Fatal("expected the foreign key to refuse an unknown role")
	}

	counts, err := roles.CountUsersPerRole()
	if err != nil || counts[role.Name] != 1 {
		t.Fatalf("counts = %v, %v", counts, err)
	}

	// renaming the role follows in users.role (ON UPDATE CASCADE)
	role.Name = "fk2-" + sfx
	if err := roles.Update(role, nil); err != nil {
		t.Fatalf("rename: %v", err)
	}
	var reloaded User
	db.First(&reloaded, user.ID)
	if reloaded.Role != role.Name {
		t.Fatalf("users.role = %q, want %q", reloaded.Role, role.Name)
	}

	// a role held by a user cannot be deleted (ON DELETE RESTRICT)
	if err := roles.Delete(role.ID); err == nil {
		t.Fatal("expected the foreign key to refuse deleting a held role")
	}
}

func TestRBACRepository_UserOverrides(t *testing.T) {
	db := importTestDB(t)
	perms := NewPermissionRepository(db)
	sfx := rbacSuffix()

	user := &User{Username: "rbac-ov-" + sfx, Password: "x", Role: "user"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM users WHERE id = ?", user.ID) })
	p1 := &Permission{Name: "ov1:" + sfx}
	p2 := &Permission{Name: "ov2:" + sfx}
	for _, p := range []*Permission{p1, p2} {
		if err := perms.Create(p); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { db.Exec("DELETE FROM permissions WHERE id IN (?, ?)", p1.ID, p2.ID) })

	if err := perms.UpdateMeta(p1.ID, "Group", "Desc"); err != nil {
		t.Fatalf("update meta: %v", err)
	}
	var reloaded Permission
	db.First(&reloaded, p1.ID)
	if reloaded.GroupName != "Group" || reloaded.Description != "Desc" {
		t.Fatalf("meta = %+v", reloaded)
	}

	if err := perms.ReplaceUserOverrides(user.ID, []UserPermissionOverride{
		{PermissionID: p1.ID, Effect: PermissionEffectAllow},
		{PermissionID: p2.ID, Effect: PermissionEffectDeny},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := perms.ListUserOverrides(user.ID)
	if err != nil || len(got) != 2 || got[0].Effect != PermissionEffectAllow || got[1].Effect != PermissionEffectDeny {
		t.Fatalf("overrides = %+v, %v", got, err)
	}
	if err := perms.ReplaceUserOverrides(user.ID, []UserPermissionOverride{{PermissionID: p2.ID, Effect: PermissionEffectAllow}}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _ = perms.ListUserOverrides(user.ID)
	if len(got) != 1 || got[0].PermissionID != p2.ID || got[0].Effect != PermissionEffectAllow {
		t.Fatalf("overrides after replace = %+v", got)
	}

	// deleting the user removes the overrides
	if err := db.Delete(&User{}, user.ID).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}
	got, _ = perms.ListUserOverrides(user.ID)
	if len(got) != 0 {
		t.Fatalf("overrides left after the user deletion: %+v", got)
	}
}
