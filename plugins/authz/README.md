# Prefab Authorization (authz)

The Prefab Authorization plugin provides a flexible role-based access control system for your Go applications. It integrates seamlessly with gRPC services using protocol buffer annotations.

## Quick Start

```go
// Define roles and actions
const (
    RoleAdmin  = authz.Role("admin")
    RoleEditor = authz.Role("editor")
    ActionView = authz.Action("view")
    ActionEdit = authz.Action("edit")
)

// Set up authorization with the builder pattern
authzPlugin := authz.NewBuilder().
    // Define policies (who can do what)
    WithPolicy(authz.Allow, RoleAdmin, ActionView).
    WithPolicy(authz.Allow, RoleAdmin, ActionEdit).
    WithPolicy(authz.Allow, RoleEditor, ActionView).
    
    // Set up role hierarchy (admin includes editor permissions)
    WithRoleHierarchy(RoleAdmin, RoleEditor).
    
    // Register functions to fetch objects by ID
    WithObjectFetcher("document", fetchDocument).
    
    // Register functions to determine user roles
    WithRoleDescriber("document", documentRoleDescriber).
    Build()

// Add to server
server := prefab.New(
    prefab.WithPlugin(authzPlugin),
    // Other plugins and options
)
```

## Using Common Patterns

For even faster setup, use the common patterns:

```go
// Use predefined roles and actions with CRUD permissions
authzPlugin := authz.NewCRUDBuilder().
    WithObjectFetcher("document", fetchDocument).
    WithRoleDescriber("document", documentRoleDescriber).
    Build()
```

## Protocol Buffer Annotations

To protect your endpoints, annotate your .proto files:

```protobuf
rpc GetDocument(GetDocumentRequest) returns (GetDocumentResponse) {
  option (prefab.authz.action) = "documents.view";
  option (prefab.authz.resource) = "document";
}

message GetDocumentRequest {
  string org_id = 1 [(prefab.authz.domain) = true];
  string document_id = 2 [(prefab.authz.id) = true];
}
```

## Role Describers and Object Fetchers

These components connect your business logic to the authorization system:

```go
// Fetch a document by ID
func fetchDocument(ctx context.Context, id any) (any, error) {
    return documentRepository.GetByID(id.(string))
}

// Determine roles for a user relative to a document
func documentRoleDescriber(ctx context.Context, identity auth.Identity, 
                           object any, domain authz.Domain) ([]authz.Role, error) {
    doc := object.(Document)
    roles := []authz.Role{}
    
    // All authenticated users get basic role
    roles = append(roles, authz.RoleUser)
    
    // Document owner gets owner role
    if doc.OwnerID == identity.Subject {
        roles = append(roles, authz.RoleOwner)
    }
    
    // Check for admin role
    if isAdmin(identity) {
        roles = append(roles, authz.RoleAdmin)
    }
    
    return roles, nil
}
```


## Examples

For complete examples, see:
- `examples/authz/custom/authzexample.go` (fully custom configuration)
- `examples/authz/common-builder/authzexample.go` (common builder pattern)