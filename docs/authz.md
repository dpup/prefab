# Authorization (Authz) Plugin

The Authz plugin provides a declarative, protocol-buffer-based authorization system for Prefab servers. It uses proto annotations to define access control rules and enforces them via a gRPC interceptor.

## Quick Start

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/auth"
    "github.com/dpup/prefab/plugins/authz"
)

// 1. Define roles
const (
    roleUser  = authz.Role("user")
    roleOwner = authz.Role("owner")
    roleAdmin = authz.Role("admin")
)

// 2. Set up authorization plugin
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(authz.Plugin(
        // Define policies (effect, role, action)
        authz.WithPolicy(authz.Allow, roleUser, authz.Action("documents.view")),
        authz.WithPolicy(authz.Allow, roleOwner, authz.Action("documents.edit")),
        authz.WithPolicy(authz.Allow, roleAdmin, authz.Action("*")),

        // Register object fetcher
        authz.WithObjectFetcher("document", authz.AsObjectFetcher(
            authz.Fetcher(db.GetDocumentByID),
        )),

        // Register role describer
        authz.WithRoleDescriber("document", authz.Compose(
            authz.OwnershipRole(roleOwner, func(doc *Document) string {
                return doc.OwnerID
            }),
        )),
    )),
)
```

## Core Concepts

### Roles and Actions

**Roles** are application-defined strings representing user capabilities (e.g., "admin", "owner", "viewer"). Roles are **context-dependent** and determined per-object by Role Describers.

**Actions** are application-defined strings representing operations (e.g., "documents.view", "documents.edit"). Actions are declared in proto annotations.

**Policies** map roles to actions with an effect (Allow or Deny):

```go
authz.WithPolicy(authz.Allow, roleEditor, authz.Action("documents.edit"))
authz.WithPolicy(authz.Deny, roleSuspended, authz.Action("*"))
```

### Authorization Flow

When an RPC is invoked, Prefab enforces authorization through this sequence:

1. **Extract metadata** from proto annotations:
   - Action (e.g., "documents.view")
   - Resource type (e.g., "document")
   - Resource ID from `[(prefab.authz.id) = true]` field
   - Scope from `[(prefab.authz.scope) = true]` field (optional)

2. **Fetch the resource object**:
   - Calls the **Object Fetcher** registered for the resource type
   - Object Fetcher receives the resource ID and returns the actual object
   - Example: `"doc-123"` → `Document{ID: "doc-123", OwnerID: "user-456"}`

3. **Determine user roles**:
   - Calls the **Role Describer** with (user identity, object, scope)
   - Role Describer examines the object and returns applicable roles
   - Example: If user owns the document → `[owner, editor]`

4. **Evaluate policies**:
   - Checks policies for each role using AWS IAM-style precedence
   - Explicit Deny wins > Explicit Allow > Default effect

5. **Grant or deny access** based on the final effect

### Policy Evaluation Precedence

Prefab uses **AWS IAM-style precedence** for clear, predictable authorization:

1. **Explicit Deny wins**: If ANY of the user's roles has a Deny policy for the action, access is denied
2. **Explicit Allow wins**: If no Deny exists and ANY role has an Allow policy, access is granted
3. **Default effect**: If no policies match any role, use the `default_effect` from the RPC method

**Benefits:**
- Create "blocklist" roles that override other permissions (e.g., suspended users)
- Grant permissions safely knowing deny policies provide ultimate control
- Predictable behavior aligned with industry standards (AWS IAM)

**Example:**
```go
// User has roles: [editor, suspended]
authz.WithPolicy(authz.Allow, roleEditor, authz.Action("documents.write"))
authz.WithPolicy(authz.Deny, roleSuspended, authz.Action("*"))
// Result: Denied (explicit deny wins over allow)
```

## Proto Annotations

### Method Options

Annotate your RPC methods with authorization metadata:

```protobuf
import "plugins/authz/authz.proto";

rpc GetDocument(GetDocumentRequest) returns (GetDocumentResponse) {
  option (prefab.authz.action) = "documents.view";
  option (prefab.authz.resource) = "document";
  option (prefab.authz.default_effect) = "deny";  // Optional, defaults to "deny"

  option (google.api.http) = {
    get: "/api/workspaces/{workspace_id}/documents/{document_id}"
  };
}
```

**Available options:**
- `(prefab.authz.action)` - The action being performed (e.g., "documents.view")
- `(prefab.authz.resource)` - The resource type (maps to registered Object Fetcher)
- `(prefab.authz.default_effect)` - Default effect if no policy matches: "allow" or "deny"

### Field Options

Mark request fields with authorization metadata:

```protobuf
message GetDocumentRequest {
  string workspace_id = 1 [(prefab.authz.scope) = true];  // Optional scope
  string document_id = 2 [(prefab.authz.id) = true];      // Required resource ID
}
```

**Available options:**
- `[(prefab.authz.id) = true]` - Marks the field containing the resource identifier
- `[(prefab.authz.scope) = true]` - Marks the field containing the scope identifier (optional)

### Complete Proto Example

```protobuf
syntax = "proto3";
package docservice;

import "google/api/annotations.proto";
import "plugins/authz/authz.proto";

service DocumentService {
  rpc ListDocuments(ListDocumentsRequest) returns (ListDocumentsResponse) {
    option (prefab.authz.action) = "documents.list";
    option (prefab.authz.resource) = "workspace";
    option (google.api.http) = {
      get: "/api/workspaces/{workspace_id}/documents"
    };
  }

  rpc GetDocument(GetDocumentRequest) returns (GetDocumentResponse) {
    option (prefab.authz.action) = "documents.view";
    option (prefab.authz.resource) = "document";
    option (prefab.authz.default_effect) = "deny";
    option (google.api.http) = {
      get: "/api/workspaces/{workspace_id}/documents/{document_id}"
    };
  }

  rpc UpdateDocument(UpdateDocumentRequest) returns (UpdateDocumentResponse) {
    option (prefab.authz.action) = "documents.update";
    option (prefab.authz.resource) = "document";
    option (prefab.authz.default_effect) = "deny";
    option (google.api.http) = {
      put: "/api/workspaces/{workspace_id}/documents/{document_id}"
      body: "*"
    };
  }
}

message ListDocumentsRequest {
  string workspace_id = 1 [(prefab.authz.id) = true];
}

message GetDocumentRequest {
  string workspace_id = 1 [(prefab.authz.scope) = true];
  string document_id = 2 [(prefab.authz.id) = true];
}

message UpdateDocumentRequest {
  string workspace_id = 1 [(prefab.authz.scope) = true];
  string document_id = 2 [(prefab.authz.id) = true];
  string title = 3;
  string content = 4;
}
```

## Object Fetchers

Object Fetchers convert resource IDs into actual objects that can be examined by Role Describers.

### Basic Object Fetcher

```go
// Register a fetcher for "document" resources
authz.WithObjectFetcher("document", authz.AsObjectFetcher(
    authz.Fetcher(func(ctx context.Context, id string) (*Document, error) {
        return db.GetDocumentByID(ctx, id)
    }),
))

// Or pass the function directly if signatures match
authz.WithObjectFetcher("document", authz.AsObjectFetcher(
    authz.Fetcher(db.GetDocumentByID),
))
```

The fetcher receives the value from the `[(prefab.authz.id) = true]` field and returns the actual object.

### Object Fetcher Patterns

Prefab provides composable, type-safe patterns for building object fetchers:

#### Fetcher - Type-safe wrapper

The foundational pattern for wrapping any fetch function:

```go
authz.Fetcher(func(ctx context.Context, id string) (*Document, error) {
    return db.GetDocumentByID(ctx, id)
})
```

#### MapFetcher - Static maps

Useful for tests, examples, or small static datasets:

```go
staticDocuments := map[string]*Document{
    "1": {ID: "1", Title: "Doc 1"},
    "2": {ID: "2", Title: "Doc 2"},
}

authz.MapFetcher(staticDocuments)
```

#### ValidatedFetcher - Add validation

Wrap a fetcher with validation logic (e.g., soft-delete checks):

```go
authz.ValidatedFetcher(
    authz.Fetcher(db.GetDocumentByID),
    func(doc *Document) error {
        if doc.Deleted {
            return errors.NewC("document deleted", codes.NotFound)
        }
        if doc.Archived {
            return errors.NewC("document archived", codes.PermissionDenied)
        }
        return nil
    },
)
```

#### ComposeFetchers - Fallback strategies

Try multiple fetchers in order (cache → database → API):

```go
authz.ComposeFetchers(
    authz.MapFetcher(cache),           // Try cache first
    authz.Fetcher(db.GetDocumentByID), // Then database
    authz.Fetcher(api.FetchDocument),  // Finally remote API
)
```

#### TransformKey - Key transformation

Transform the key before fetching:

```go
// Convert string IDs to int IDs
authz.TransformKey(
    func(id string) int { return parseID(id) },
    authz.Fetcher(db.GetDocumentByNumericID),
)
```

#### DefaultFetcher - Return default on error

Return a default value instead of an error:

```go
authz.DefaultFetcher(
    authz.Fetcher(db.GetUserByID),
    &User{ID: "guest", Name: "Guest User"},
)
```

### Real-World Composition Example

```go
authz.WithObjectFetcher("org", authz.AsObjectFetcher(
    authz.ComposeFetchers(
        // Try cache first
        authz.MapFetcher(cache),

        // Then validated database fetch
        authz.ValidatedFetcher(
            authz.Fetcher(db.GetOrgByID),
            func(org *Org) error {
                if org.Deleted {
                    return errors.NewC("org deleted", codes.NotFound)
                }
                return nil
            },
        ),

        // Finally try remote API
        authz.Fetcher(api.FetchOrg),
    ),
))
```

## Role Describers

Role Describers determine what roles a user has for a specific object and scope.

### Basic Role Describer

```go
authz.WithRoleDescriber("document", authz.Compose(
    // Grant owner role if user owns the document
    authz.OwnershipRole(authz.RoleOwner, func(doc *Document) string {
        return doc.OwnerID
    }),

    // Grant editor role based on workspace membership
    authz.MembershipRoles(
        func(doc *Document) string { return doc.WorkspaceID },
        func(ctx context.Context, workspaceID string, identity auth.Identity) ([]authz.Role, error) {
            return getWorkspaceRoles(ctx, workspaceID, identity.Subject)
        },
    ),
))
```

Role describers receive `(identity, object, scope)` and return a list of roles.

### Role Describer Patterns

Prefab provides composable, type-safe patterns for building role describers:

#### Compose - Combine multiple describers

Combines multiple role describers and provides automatic scope validation for `ScopedObject`:

```go
authz.Compose(
    authz.OwnershipRole(...),
    authz.StaticRole(...),
    authz.MembershipRoles(...),
)
```

If the object implements `ScopedObject`, `Compose` automatically validates that `object.ScopeID() == scope` before calling describers. If the scope doesn't match, it returns empty roles.

#### OwnershipRole - Grant role to owner

Grants a role if the user owns the resource:

```go
authz.OwnershipRole(authz.RoleOwner, func(doc *Document) string {
    return doc.OwnerID
})
```

#### ConditionalRole - Async predicate

Grants a role based on an async predicate (i.e. it might error, useful for database queries):

```go
authz.ConditionalRole(authz.RoleEditor, func(ctx context.Context, identity auth.Identity, doc *Document, scope authz.Scope) (bool, error) {
    // Check if user has edit permission in database
    return db.HasEditPermission(ctx, identity.Subject, doc.ID)
})
```

#### StaticRole - Sync predicate

Grants a role based on a sync predicate (i.e. an error isn't possible):

```go
authz.StaticRole(authz.RoleViewer, func(_ context.Context, _ auth.Identity, doc *Document) bool {
    return doc.Published
})
```

#### StaticRoles - Multiple roles from conditions

Returns multiple roles based on conditions:

```go
authz.StaticRoles(func(ctx context.Context, identity auth.Identity, doc *Document) []authz.Role {
    var roles []authz.Role
    if doc.Published {
        roles = append(roles, authz.RoleViewer)
    }
    if doc.Featured {
        roles = append(roles, "featured-viewer")
    }
    return roles
})
```

#### GlobalRole - Context-based role

Grants a role based on context only (no object examination):

```go
authz.GlobalRole(authz.RoleAdmin, func(ctx context.Context, identity auth.Identity, scope authz.Scope) (bool, error) {
    // Check if user is a superuser
    return db.IsSuperuser(ctx, identity.Subject)
})
```

#### MembershipRoles - Parent resource roles

Grants roles based on parent resource membership:

```go
authz.MembershipRoles(
    // Extract parent ID from object
    func(doc *Document) string { return doc.WorkspaceID },

    // Fetch roles from parent
    func(ctx context.Context, workspaceID string, identity auth.Identity) ([]authz.Role, error) {
        workspace, err := fetchWorkspace(ctx, workspaceID)
        if err != nil {
            return nil, err
        }
        return workspace.GetUserRoles(ctx, identity.Subject)
    },
)
```

#### ScopeRoles - Scope-based roles

Grants roles if the object is in a specific scope:

```go
authz.ScopeRoles(
    func(doc *Document) string { return doc.WorkspaceID },
    func(ctx context.Context, workspaceID string, identity auth.Identity) ([]authz.Role, error) {
        workspace, err := fetchWorkspace(ctx, workspaceID)
        if err != nil {
            return nil, err
        }
        return workspace.GetUserRoles(ctx, identity.Subject)
    },
)
```

### Real-World Composition Example

```go
authz.WithRoleDescriber("document", authz.Compose(
    // Grant owner role if user owns the document
    authz.OwnershipRole(authz.RoleOwner, func(doc *Document) string {
        return doc.OwnerID
    }),

    // Grant viewer role if document is published
    authz.StaticRole(authz.RoleViewer, func(_ context.Context, _ auth.Identity, doc *Document) bool {
        return doc.Published
    }),

    // Grant workspace roles based on membership
    authz.MembershipRoles(
        func(doc *Document) string { return doc.WorkspaceID },
        func(ctx context.Context, workspaceID string, identity auth.Identity) ([]authz.Role, error) {
            workspace, err := fetchWorkspace(ctx, workspaceID)
            if err != nil {
                return nil, err
            }
            return workspace.GetUserRoles(ctx, identity.Subject)
        },
    ),

    // Grant admin role to superusers
    authz.GlobalRole(authz.RoleAdmin, func(ctx context.Context, identity auth.Identity, _ authz.Scope) (bool, error) {
        return db.IsSuperuser(ctx, identity.Subject)
    }),
))
```

## Scopes

The `scope` parameter represents the "container" of the object being accessed:
- Document = Object, Workspace = Scope
- Note = Object, Folder = Scope
- File = Object, Organization = Scope

### Scope Validation

When using `Compose` with objects that implement `ScopedObject`, scope validation is automatic:

```go
type Document struct {
    ID          string
    WorkspaceID string
    OwnerID     string
}

func (d *Document) AuthzType() string { return "document" }
func (d *Document) ScopeID() string { return d.WorkspaceID }
```

If the object's `ScopeID()` doesn't match the request scope, `Compose` returns empty roles.

### Manual Scope Checking

For custom role describers, check scope manually:

```go
func describeRoles(ctx context.Context, identity auth.Identity, object any, scope authz.Scope) ([]authz.Role, error) {
    doc := object.(*Document)

    // Check scope matches
    if string(scope) != doc.WorkspaceID {
        return []authz.Role{}, nil
    }

    // Return roles...
    return roles, nil
}
```

## Custom Roles

Roles are just strings, so you can define your own:

```go
const (
    // Framework-provided roles
    roleAdmin  = authz.RoleAdmin   // "admin"
    roleEditor = authz.RoleEditor  // "editor"
    roleViewer = authz.RoleViewer  // "viewer"
    roleOwner  = authz.RoleOwner   // "owner"

    // Custom roles
    reviewer    = authz.Role("reviewer")
    contributor = authz.Role("contributor")
    moderator   = authz.Role("moderator")
)

authz.WithRoleDescriber("pull_request", authz.Compose(
    // Use framework roles
    authz.OwnershipRole(roleOwner, func(pr *PullRequest) string {
        return pr.AuthorID
    }),

    // Use custom roles
    authz.ConditionalRole(reviewer, func(_ context.Context, identity auth.Identity, pr *PullRequest, _ authz.Scope) (bool, error) {
        for _, r := range pr.Reviewers {
            if r == identity.Subject {
                return true, nil
            }
        }
        return false, nil
    }),
))
```

## Role Hierarchy

Establish a role hierarchy where parent roles inherit child roles:

```go
authz.WithRoleHierarchy(authz.RoleAdmin, authz.RoleEditor, authz.RoleViewer, authz.RoleUser)
```

In this example:
- Admins inherit all editor permissions
- Editors inherit viewer permissions
- Viewers inherit user permissions

**Example:**
```go
authz.Plugin(
    authz.WithPolicy(authz.Allow, authz.RoleViewer, authz.Action("documents.view")),
    authz.WithPolicy(authz.Allow, authz.RoleEditor, authz.Action("documents.edit")),

    authz.WithRoleHierarchy(authz.RoleAdmin, authz.RoleEditor, authz.RoleViewer),
)

// User with "editor" role can:
// - documents.view (inherited from viewer)
// - documents.edit (direct permission)

// User with "admin" role can:
// - documents.view (inherited from editor → viewer)
// - documents.edit (inherited from editor)
```

## Wildcard Actions and Resources

Use wildcards in policies for broad permissions:

```go
// Admin can perform any action
authz.WithPolicy(authz.Allow, authz.RoleAdmin, authz.Action("*"))

// Suspended users are denied everything
authz.WithPolicy(authz.Deny, roleSuspended, authz.Action("*"))
```

Use wildcards in object fetchers and role describers for default handling:

```go
// Default role describer for all resource types
authz.WithRoleDescriber("*", func(ctx context.Context, identity auth.Identity, object any, scope authz.Scope) ([]authz.Role, error) {
    // Grant basic user role to all authenticated users
    return []authz.Role{authz.RoleUser}, nil
})
```

## Builder Pattern

For complex setups, use the builder pattern:

```go
builder := authz.NewBuilder().
    WithPolicy(authz.Allow, roleUser, authz.Action("documents.view")).
    WithPolicy(authz.Allow, roleOwner, authz.Action("documents.edit")).
    WithPolicy(authz.Allow, roleAdmin, authz.Action("*")).
    WithRoleHierarchy(roleAdmin, roleEditor, roleViewer, roleUser).
    WithObjectFetcher("document", authz.AsObjectFetcher(
        authz.Fetcher(db.GetDocumentByID),
    )).
    WithRoleDescriber("document", authz.Compose(
        authz.OwnershipRole(roleOwner, func(doc *Document) string {
            return doc.OwnerID
        }),
    ))

s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(builder.Build()),
)
```

### Common CRUD Setup

For standard CRUD operations, define policies for common actions:

```go
builder := authz.NewBuilder().
    // CRUD policies
    WithPolicy(authz.Allow, authz.RoleViewer, authz.ActionRead).
    WithPolicy(authz.Allow, authz.RoleEditor, authz.ActionCreate).
    WithPolicy(authz.Allow, authz.RoleEditor, authz.ActionRead).
    WithPolicy(authz.Allow, authz.RoleEditor, authz.ActionUpdate).
    WithPolicy(authz.Allow, authz.RoleAdmin, authz.ActionDelete).
    WithPolicy(authz.Allow, authz.RoleAdmin, authz.Action("*")).
    // Object fetcher and role describer
    WithObjectFetcher("document", authz.AsObjectFetcher(
        authz.Fetcher(db.GetDocumentByID),
    )).
    WithRoleDescriber("document", authz.Compose(
        authz.OwnershipRole(authz.RoleOwner, func(doc *Document) string {
            return doc.OwnerID
        }),
    ))

s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(builder.Build()),
)
```

Common CRUD actions are predefined:
- `authz.ActionCreate` - Create operations
- `authz.ActionRead` - Read operations
- `authz.ActionUpdate` - Update operations
- `authz.ActionDelete` - Delete operations

## Debugging

### Debug Endpoint

The authz plugin provides a debug endpoint at `/debug/authz` that shows:
- Registered policies
- Role hierarchy
- Registered object fetchers and role describers

### Structured Logging

Authorization decisions are logged with structured fields for debugging:
- `authz.action` - The action being evaluated
- `authz.resource` - The resource type
- `authz.objectID` - The ID of the object being accessed
- `authz.scope` - The scope (if specified)
- `authz.roles` - The roles assigned to the user
- `authz.evaluated_policies` - List of policies that were evaluated (role + effect)
- `authz.effect` - The final effect (ALLOW/DENY)
- `authz.reason` - Why access was granted or denied

**Example log output:**
```json
{
  "authz.action": "documents.write",
  "authz.resource": "document",
  "authz.objectID": "doc-123",
  "authz.roles": ["editor", "suspended"],
  "authz.evaluated_policies": [
    {"role": "editor", "effect": "ALLOW"},
    {"role": "suspended", "effect": "DENY"}
  ],
  "authz.effect": "DENY",
  "authz.reason": "denied by policy"
}
```

### Enhanced Error Messages

When access is denied, users receive clear, actionable error messages:

**Before:**
```
Error: you are not authorized to perform this action
```

**After:**
```
Error: Access denied: explicitly denied by role 'suspended'
```

The error message explains why access was denied based on the evaluated policies:
- "no roles assigned" - User has no roles for this resource
- "no policies match action 'X' for your roles" - No policies cover this action
- "explicitly denied by role 'X'" - A deny policy blocked access
- "action 'X' not explicitly allowed (default: deny)" - No allow policy matched

### Audit Logging

Configure an audit logger to receive all authorization decisions for compliance and security monitoring:

```go
authz.WithAuditLogger(func(ctx context.Context, decision authz.AuthzDecision) {
    log.Printf("authz: user=%s action=%s resource=%s:%s effect=%s",
        decision.Identity.Subject,
        decision.Action,
        decision.Resource,
        decision.ObjectID,
        decision.Effect)

    // Send to audit system
    auditSystem.LogAuthzDecision(ctx, decision)
})
```

The `AuthzDecision` contains:
- `Action` - The action that was attempted
- `Resource` - The resource type
- `ObjectID` - The resource identifier
- `Scope` - The scope (if specified)
- `Identity` - The authenticated user's identity
- `Roles` - The roles assigned to the user
- `Effect` - The final decision (Allow or Deny)
- `DefaultEffect` - The default effect from the RPC
- `Reason` - Human-readable reason for the decision
- `EvaluatedPolicies` - List of policies that were checked

The audit logger is called for **both allowed and denied requests**, providing complete visibility.

## Complete Example

```go
package main

import (
    "context"

    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/auth"
    "github.com/dpup/prefab/plugins/authz"
)

// Define roles
const (
    roleUser  = authz.Role("user")
    roleOwner = authz.Role("owner")
    roleAdmin = authz.Role("admin")
)

// Define your domain object
type Document struct {
    ID          string
    WorkspaceID string
    OwnerID     string
    Published   bool
}

func (d *Document) AuthzType() string { return "document" }
func (d *Document) ScopeID() string { return d.WorkspaceID }

func main() {
    s := prefab.New(
        prefab.WithPlugin(auth.Plugin()),
        prefab.WithPlugin(authz.Plugin(
            // Define policies
            authz.WithPolicy(authz.Allow, roleUser, authz.Action("documents.view")),
            authz.WithPolicy(authz.Allow, roleOwner, authz.Action("documents.edit")),
            authz.WithPolicy(authz.Allow, roleAdmin, authz.Action("*")),

            // Role hierarchy
            authz.WithRoleHierarchy(roleAdmin, roleOwner, roleUser),

            // Object fetcher
            authz.WithObjectFetcher("document", authz.AsObjectFetcher(
                authz.Fetcher(getDocument),
            )),

            // Role describer
            authz.WithRoleDescriber("document", authz.Compose(
                authz.OwnershipRole(roleOwner, func(doc *Document) string {
                    return doc.OwnerID
                }),
                authz.StaticRole(roleUser, func(_ context.Context, _ auth.Identity, doc *Document) bool {
                    return doc.Published
                }),
            )),
        )),
    )

    // Register your service
    s.RegisterService(...)

    s.Start()
}

func getDocument(ctx context.Context, id string) (*Document, error) {
    // Fetch from database
    return db.GetDocumentByID(ctx, id)
}
```

## Best Practices

1. **Use proto annotations** - Define authorization rules in proto files for clear documentation
2. **Use composable patterns** - Leverage `Compose`, `OwnershipRole`, `MembershipRoles` to eliminate boilerplate
3. **Implement ScopedObject** - Get automatic scope validation with `Compose`
4. **Use role hierarchy** - Define clear role inheritance to reduce policy duplication
5. **Use explicit deny for blocklists** - Create suspended/banned roles that override other permissions
6. **Test authorization** - Write tests for role describers and policy evaluation
7. **Log authorization decisions** - Enable structured logging to debug access issues
8. **Use the debug endpoint** - Verify policies and registrations are correct

## Migration from Manual Type Assertions

**Old pattern:**

```go
builder.WithRoleDescriberFn("document", func(ctx context.Context, identity auth.Identity, object any, scope authz.Scope) ([]authz.Role, error) {
    // Manual type assertion
    doc, ok := object.(*Document)
    if !ok {
        return nil, errors.NewC("expected Document", codes.Internal)
    }

    // Manual scope validation
    if string(scope) != doc.WorkspaceID {
        return []authz.Role{}, nil
    }

    var roles []authz.Role
    if doc.OwnerID == identity.Subject {
        roles = append(roles, authz.RoleOwner)
    }
    return roles, nil
})
```

**New pattern:**

```go
builder.WithRoleDescriber("document", authz.Compose(
    authz.OwnershipRole(authz.RoleOwner, func(doc *Document) string {
        return doc.OwnerID
    }),
))
```

The new patterns eliminate:
- Manual type assertions
- Manual scope validation
- Boilerplate error handling
- Repetitive role-checking logic
