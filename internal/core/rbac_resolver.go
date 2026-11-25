package core

import (
	"sort"

	"goonhub/internal/data"
)

// Permission sources reported by the effective views (docs/RBAC.md).
const (
	PermSourceSuperuser = "superuser"
	PermSourceRole      = "role"
	PermSourceOverride  = "override"
	PermSourceNone      = "none"
	permSourceParentPre = "parent:"
)

// roleDecision is the effective value of a role for a permission.
type roleDecision struct {
	// Effect is allow, deny or "" (set nowhere in the role and its ancestors)
	Effect string
	// Origin is the name of the role whose own setting (or superuser flag) decided
	Origin    string
	Superuser bool
}

// source tells, from the point of view of the role named roleName, where the decision comes
// from: superuser, role (own setting), parent:<origin role> or none.
func (d roleDecision) source(roleName string) string {
	switch {
	case d.Effect == "":
		return PermSourceNone
	case d.Origin == roleName && d.Superuser:
		return PermSourceSuperuser
	case d.Origin == roleName:
		return PermSourceRole
	default:
		return permSourceParentPre + d.Origin
	}
}

// rbacGraph is an immutable snapshot of the roles, their parents and settings and the
// permissions, with every role's effective value precomputed.
type rbacGraph struct {
	roles      map[uint]data.Role
	roleByName map[string]uint
	parents    map[uint][]uint
	settings   map[uint]map[uint]string
	perms      []data.Permission // id order
	permByName map[string]data.Permission
	permByID   map[uint]data.Permission
	// visible are the catalog permissions present in the table, in catalog order
	visible   []data.Permission
	effective map[uint]map[uint]roleDecision
}

func newRBACGraph(g *data.RBACGraph, perms []data.Permission) *rbacGraph {
	rg := &rbacGraph{
		roles:      make(map[uint]data.Role, len(g.Roles)),
		roleByName: make(map[string]uint, len(g.Roles)),
		parents:    make(map[uint][]uint),
		settings:   make(map[uint]map[uint]string),
		perms:      perms,
		permByName: make(map[string]data.Permission, len(perms)),
		permByID:   make(map[uint]data.Permission, len(perms)),
		effective:  make(map[uint]map[uint]roleDecision, len(g.Roles)),
	}
	for _, r := range g.Roles {
		rg.roles[r.ID] = r
		rg.roleByName[r.Name] = r.ID
	}
	for _, p := range g.Parents {
		if _, ok := rg.roles[p.ParentID]; !ok {
			continue
		}
		rg.parents[p.RoleID] = append(rg.parents[p.RoleID], p.ParentID)
	}
	for id := range rg.parents {
		sort.Slice(rg.parents[id], func(i, j int) bool { return rg.parents[id][i] < rg.parents[id][j] })
	}
	for _, s := range g.Settings {
		if rg.settings[s.RoleID] == nil {
			rg.settings[s.RoleID] = make(map[uint]string)
		}
		rg.settings[s.RoleID][s.PermissionID] = s.Effect
	}
	for _, p := range perms {
		rg.permByName[p.Name] = p
		rg.permByID[p.ID] = p
	}
	for _, c := range PermissionCatalog {
		if p, ok := rg.permByName[c.Name]; ok {
			rg.visible = append(rg.visible, p)
		}
	}
	for roleID := range rg.roles {
		rg.effective[roleID] = make(map[uint]roleDecision, len(perms))
		for _, p := range perms {
			rg.effective[roleID][p.ID] = rg.decide(roleID, p.ID, map[uint]bool{})
		}
	}
	return rg
}

// decide computes the effective value of a role for a permission: its own setting, else
// its parents' effective values where deny wins, else unset. A superuser role allows
// everything. visiting guards against cycles (refused on write, ignored here).
func (g *rbacGraph) decide(roleID, permID uint, visiting map[uint]bool) roleDecision {
	if byPerm, ok := g.effective[roleID]; ok {
		if d, ok := byPerm[permID]; ok {
			return d
		}
	}
	role, ok := g.roles[roleID]
	if !ok {
		return roleDecision{}
	}
	if role.IsSuperuser {
		return roleDecision{Effect: data.PermissionEffectAllow, Origin: role.Name, Superuser: true}
	}
	if effect, ok := g.settings[roleID][permID]; ok {
		return roleDecision{Effect: effect, Origin: role.Name}
	}
	if visiting[roleID] {
		return roleDecision{}
	}
	visiting[roleID] = true
	defer delete(visiting, roleID)

	var allowed, denied *roleDecision
	for _, parentID := range g.parents[roleID] {
		d := g.decide(parentID, permID, visiting)
		switch d.Effect {
		case data.PermissionEffectDeny:
			if denied == nil {
				denied = &d
			}
		case data.PermissionEffectAllow:
			if allowed == nil {
				allowed = &d
			}
		}
	}
	if denied != nil {
		return *denied
	}
	if allowed != nil {
		return *allowed
	}
	return roleDecision{}
}

// roleDecision returns the precomputed effective value of a role for a permission.
func (g *rbacGraph) roleDecision(roleID, permID uint) roleDecision {
	return g.effective[roleID][permID]
}

// isAncestor reports whether ancestorID is roleID itself or one of its ancestors.
func (g *rbacGraph) isAncestor(ancestorID, roleID uint) bool {
	seen := map[uint]bool{}
	stack := []uint{roleID}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == ancestorID {
			return true
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		stack = append(stack, g.parents[id]...)
	}
	return false
}

// createsCycle reports whether giving roleID the parents parentIDs creates a cycle: a parent
// is the role itself or has the role among its ancestors.
func (g *rbacGraph) createsCycle(roleID uint, parentIDs []uint) bool {
	for _, p := range parentIDs {
		if g.isAncestor(roleID, p) {
			return true
		}
	}
	return false
}

// userDecision resolves a permission for a user holding roleID with the given overrides
// (permission ID -> effect): superuser role, then override, then the role's value.
func (g *rbacGraph) userDecision(roleID uint, overrides map[uint]string, permID uint) (bool, string) {
	role, ok := g.roles[roleID]
	if ok && role.IsSuperuser {
		return true, PermSourceSuperuser
	}
	if effect, ok := overrides[permID]; ok {
		return effect == data.PermissionEffectAllow, PermSourceOverride
	}
	if !ok {
		return false, PermSourceNone
	}
	d := g.roleDecision(roleID, permID)
	return d.Effect == data.PermissionEffectAllow, d.source(role.Name)
}
