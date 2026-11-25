package core

import (
	"fmt"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
)

// Delegation rules (docs/RBAC.md, "Delegation"): an actor who is not a superuser can only
// hand out what they hold. Every check answers a ForbiddenError (403); a superuser actor
// passes them all.

func errNoActor() error {
	return apperrors.NewForbiddenError("the action has no authenticated actor")
}

// exceedingPermission returns the first catalog permission the role grants that the actor
// does not hold ("" when the role's effective permissions are a subset of the actor's).
func (g *rbacGraph) exceedingPermission(roleID uint, actor *UserAccess) string {
	for _, p := range g.visible {
		if g.roleDecision(roleID, p.ID).Effect == data.PermissionEffectAllow && !actor.Has(p.Name) {
			return p.Name
		}
	}
	return ""
}

// actorLineage returns the actor's role and all its ancestors: roles the actor may not edit
// nor delete (it would change the actor's own permissions).
func (g *rbacGraph) actorLineage(actor *UserAccess) map[uint]bool {
	out := map[uint]bool{}
	id, ok := g.roleByName[actor.Role]
	if !ok {
		return out
	}
	stack := []uint{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if out[cur] {
			continue
		}
		out[cur] = true
		stack = append(stack, g.parents[cur]...)
	}
	return out
}

// checkGrantableRole refuses a role (assigned to a user or added as a parent) that is a
// superuser role or grants more than the actor holds.
func (g *rbacGraph) checkGrantableRole(actor *UserAccess, roleID uint, what string) error {
	role, ok := g.roles[roleID]
	if !ok {
		return nil
	}
	if role.IsSuperuser {
		return apperrors.NewForbiddenError(fmt.Sprintf("only a superuser can %s the superuser role %q", what, role.Name))
	}
	if p := g.exceedingPermission(roleID, actor); p != "" {
		return apperrors.NewForbiddenError(fmt.Sprintf("you cannot %s role %q: it grants %s, which you do not have", what, role.Name, p))
	}
	return nil
}

// checkEditableRole refuses changing a superuser role, the actor's own role or one it
// inherits from.
func (g *rbacGraph) checkEditableRole(actor *UserAccess, roleID uint) error {
	role, ok := g.roles[roleID]
	if !ok {
		return nil
	}
	if role.IsSuperuser {
		return apperrors.NewForbiddenError("only a superuser can change a superuser role")
	}
	if g.actorLineage(actor)[roleID] {
		return apperrors.NewForbiddenError("you cannot change your own role nor a role it inherits from")
	}
	return nil
}

// checkAllowedSettings refuses 'allow' settings on permissions the actor does not hold,
// except those already allowed (current: permission ID -> effect).
func (g *rbacGraph) checkAllowedSettings(actor *UserAccess, settings []PermissionSetting, current map[uint]string) error {
	for _, st := range settings {
		if st.Effect != data.PermissionEffectAllow || current[st.PermissionID] == data.PermissionEffectAllow {
			continue
		}
		p := g.permByID[st.PermissionID]
		if !actor.Has(p.Name) {
			return apperrors.NewForbiddenError(fmt.Sprintf("you cannot allow %s: you do not have it", p.Name))
		}
	}
	return nil
}

// CheckAssignableRole refuses assigning a role (user creation, role change) that is a
// superuser role or grants more than the actor holds.
func (s *RBACService) CheckAssignableRole(actor *UserAccess, roleName string) error {
	if actor == nil {
		return errNoActor()
	}
	if actor.Superuser {
		return nil
	}
	g := s.currentGraph()
	id, ok := g.roleByName[roleName]
	if !ok {
		return nil // existence is checked by the caller
	}
	return g.checkGrantableRole(actor, id, "assign")
}

// CheckManageableUser refuses acting on a user holding a superuser role and, unless
// allowSelf, on the actor's own account.
func (s *RBACService) CheckManageableUser(actor *UserAccess, target *data.User, allowSelf bool) error {
	if actor == nil {
		return errNoActor()
	}
	if actor.Superuser {
		return nil
	}
	if !allowSelf && target.ID == actor.UserID {
		return apperrors.NewForbiddenError("you cannot change your own role nor your own permissions")
	}
	if s.IsSuperuserRole(target.Role) {
		return apperrors.NewForbiddenError("only a superuser can manage a superuser account")
	}
	return nil
}
