package authz

// Common predefined roles.
const (
	RoleAdmin  = Role("admin")
	RoleEditor = Role("editor")
	RoleViewer = Role("viewer")
	RoleOwner  = Role("owner")
	RoleUser   = Role("user")
)

// Common predefined actions.
const (
	ActionCreate = Action("create")
	ActionRead   = Action("read")
	ActionUpdate = Action("update")
	ActionDelete = Action("delete")
	ActionList   = Action("list")
)

// NewCommonBuilder creates a new builder with common configuration
// including predefined roles and a standard role hierarchy with CRUD permissions.
//
// Role hierarchy: Admin > Editor > Viewer > User
//
// Default permissions:
// - Admin can do everything
// - Editor can create, read, update, list
// - Viewer can read and list
// - User has minimal permissions
//
// The builder can be further customized by adding additional policies,
// object fetchers, and role describers.
func NewCommonBuilder() *Builder {
	return NewBuilder().
		WithRoleHierarchy(RoleAdmin, RoleEditor, RoleViewer, RoleUser).

		// Admin permissions (already inherits all other roles)
		WithPolicy(Allow, RoleAdmin, ActionDelete).

		// Editor permissions
		WithPolicy(Allow, RoleEditor, ActionCreate).
		WithPolicy(Allow, RoleEditor, ActionRead).
		WithPolicy(Allow, RoleEditor, ActionUpdate).
		WithPolicy(Allow, RoleEditor, ActionList).

		// Viewer permissions
		WithPolicy(Allow, RoleViewer, ActionRead).
		WithPolicy(Allow, RoleViewer, ActionList)
}
