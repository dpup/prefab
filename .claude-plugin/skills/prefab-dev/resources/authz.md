# Authorization

The authz plugin provides declarative, proto-based access control using AWS IAM-style policy evaluation.

## Proto Annotations

Annotate your proto files with authorization metadata:

```protobuf
import "plugins/authz/authz.proto";

rpc GetDocument(GetDocumentRequest) returns (GetDocumentResponse) {
  option (prefab.authz.action) = "documents.view";
  option (prefab.authz.resource) = "document";
  option (prefab.authz.default_effect) = "deny";
}

message GetDocumentRequest {
  string workspace_id = 1 [(prefab.authz.scope) = true];  // Optional scope
  string document_id = 2 [(prefab.authz.id) = true];      // Required resource ID
}
```

## Server Setup

```go
import "github.com/dpup/prefab/plugins/authz"

const (
    roleUser  = authz.Role("user")
    roleOwner = authz.Role("owner")
    roleAdmin = authz.Role("admin")
)

s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(authz.Plugin(
        // Policies: Allow role X to perform action Y
        authz.WithPolicy(authz.Allow, roleUser, authz.Action("documents.view")),
        authz.WithPolicy(authz.Allow, roleOwner, authz.Action("documents.edit")),
        authz.WithPolicy(authz.Allow, roleAdmin, authz.Action("*")),

        // Object Fetcher: Convert ID to Object
        authz.WithObjectFetcher("document", authz.AsObjectFetcher(
            authz.Fetcher(db.GetDocumentByID),
        )),

        // Role Describer: Determine user roles for object
        authz.WithRoleDescriber("document", authz.Compose(
            authz.OwnershipRole(roleOwner, func(d *Document) string {
                return d.OwnerID
            }),
        )),
    )),
)
```

## Authorization Flow

When an RPC is invoked:
1. Extract action, resource type, ID, and scope from proto annotations
2. Fetch object using registered Object Fetcher
3. Determine user roles using registered Role Describer
4. Evaluate policies using AWS IAM-style precedence (Deny > Allow > Default)
5. Grant or deny access

## Object Fetchers

```go
// Database fetch
authz.Fetcher(db.GetDocByID)

// Static map
authz.MapFetcher(staticDocs)

// Fallback chain
authz.ComposeFetchers(cache, db, api)

// Add validation
authz.ValidatedFetcher(fetcher, validateFunc)
```

## Role Describers

```go
// Grant if user owns resource
authz.OwnershipRole(role, getOwnerID)

// Async identity to owner resolution
authz.IdentityOwnershipRole(role, resolve, getOwnerID)

// Grant roles from scope (validates scope)
authz.MembershipRoles(getScopeID, getRoles)

// Grant based on condition
authz.StaticRole(role, predicate)

// Combine multiple describers
authz.Compose(describer1, describer2, ...)
```

## Policy Precedence

Policies are evaluated in AWS IAM order:
1. **Explicit Deny** - Always takes precedence
2. **Explicit Allow** - Grants access if no deny
3. **Default Effect** - Applied if no matching policy

```go
// Deny always wins
authz.WithPolicy(authz.Deny, roleGuest, authz.Action("admin.*"))

// Allow for specific roles
authz.WithPolicy(authz.Allow, roleAdmin, authz.Action("*"))
```

## Complete Example

```go
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(authz.Plugin(
        // Policies
        authz.WithPolicy(authz.Allow, "member", authz.Action("documents.view")),
        authz.WithPolicy(authz.Allow, "owner", authz.Action("documents.*")),
        authz.WithPolicy(authz.Allow, "admin", authz.Action("*")),

        // Fetcher
        authz.WithObjectFetcher("document", authz.AsObjectFetcher(
            authz.Fetcher(func(ctx context.Context, id string) (*Document, error) {
                return db.GetDocument(ctx, id)
            }),
        )),

        // Role describer
        authz.WithRoleDescriber("document", authz.Compose(
            authz.OwnershipRole("owner", func(d *Document) string {
                return d.OwnerID
            }),
            authz.MembershipRoles(
                func(d *Document) string { return d.WorkspaceID },
                func(ctx context.Context, wsID, userID string) ([]authz.Role, error) {
                    return workspace.GetUserRoles(ctx, wsID, userID)
                },
            ),
        )),
    )),
)
```

For complete documentation, see [/docs/authz.md](/docs/authz.md).
