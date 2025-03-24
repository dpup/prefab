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

// NewCRUDBuilder creates a new builder with common configuration
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
func NewCRUDBuilder() *Builder {
	b := NewBuilder().
		WithRoleHierarchy(RoleAdmin, RoleEditor, RoleViewer, RoleUser)
	
	// Admin permissions (already inherits all other roles)
	b.WithPolicy(Allow, RoleAdmin, ActionDelete)
	
	// Editor permissions
	b.WithPolicy(Allow, RoleEditor, ActionCreate)
	b.WithPolicy(Allow, RoleEditor, ActionRead)
	b.WithPolicy(Allow, RoleEditor, ActionUpdate)
	b.WithPolicy(Allow, RoleEditor, ActionList)
	
	// Viewer permissions
	b.WithPolicy(Allow, RoleViewer, ActionRead)
	b.WithPolicy(Allow, RoleViewer, ActionList)
	
	return b
}

