package handler

import "goonhub/internal/core"

// testAccessForRole stands in for the resolved access in handler tests: admin is the
// superuser role, any other role has no permission.
func testAccessForRole(role string) *core.UserAccess {
	return core.NewUserAccess(1, role, role == "admin")
}
