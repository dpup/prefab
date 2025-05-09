# Prefab Library Reference

This reference provides essential patterns and examples for working with the Prefab library. Use this as a quick reference when implementing a Prefab-based server.

## Server Creation and Initialization

```go
import (
    "github.com/dpup/prefab"
    // Import plugins as needed
)

func main() {
    // Create server with options
    s := prefab.New(
        prefab.WithPort(8080),
        prefab.WithHTTPHandler("/custom", myHandler),
        prefab.WithStaticFiles("/static/", "./static/"),
        // Add plugins as needed
    )

    // Register service and gateway
    yourservice.RegisterYourServiceHandlerFromEndpoint(s.GatewayArgs())
    yourservice.RegisterYourServiceServer(s.ServiceRegistrar(), &yourServiceImpl{})

    // Start the server - this blocks until shutdown
    if err := s.Start(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Plugin Integration

### Authentication

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/auth"
    "github.com/dpup/prefab/plugins/auth/google"    // OAuth
    "github.com/dpup/prefab/plugins/auth/magiclink" // Email magic links
    "github.com/dpup/prefab/plugins/auth/pwdauth"   // Password auth
    "github.com/dpup/prefab/plugins/auth/fakeauth"  // Fake auth for testing
)

// Google OAuth authentication
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(google.Plugin()),
)

// Password authentication with custom account store
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(pwdauth.Plugin(
        pwdauth.WithAccountFinder(myAccountStore),
        pwdauth.WithHasher(myPasswordHasher),
    )),
)

// Magic link authentication (requires email plugin)
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(email.Plugin()),
    prefab.WithPlugin(templates.Plugin()),
    prefab.WithPlugin(magiclink.Plugin()),
)

// Fake authentication for testing
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(fake.Plugin(
        // Optionally customize default identity
        fake.WithDefaultIdentity(auth.Identity{
            Subject: "test-user-123",
            Email:   "test@example.com",
            Name:    "Test User",
        }),
        // Optionally add validation
        fake.WithIdentityValidator(validateTestIdentity),
    )),
)
```

### Authorization

#### Proto File Setup

```protobuf
syntax = "proto3";
package resourceservice;
option go_package = "./resourceservice";

import "google/api/annotations.proto";
import "plugins/authz/authz.proto";  // Import Prefab's authz proto

service ResourceService {
  // List all resources - requires basic user access
  rpc ListResources(ListResourcesRequest) returns (ListResourcesResponse) {
    option (prefab.authz.action) = "resources.list";
    option (prefab.authz.resource) = "workspace";

    option (google.api.http) = {
      get: "/api/{workspace_id}/resources"
    };
  }

  // Get a specific resource - checks ownership or admin access
  rpc GetResource(GetResourceRequest) returns (GetResourceResponse) {
    option (prefab.authz.action) = "resources.view";
    option (prefab.authz.resource) = "resource";
    option (prefab.authz.default_effect) = "deny";

    option (google.api.http) = {
      get: "/api/{workspace_id}/resources/{resource_id}"
    };
  }

  // Update a resource - requires owner permission
  rpc UpdateResource(UpdateResourceRequest) returns (UpdateResourceResponse) {
    option (prefab.authz.action) = "resources.update";
    option (prefab.authz.resource) = "resource";
    option (prefab.authz.default_effect) = "deny";

    option (google.api.http) = {
      put: "/api/{workspace_id}/resources/{resource_id}"
      body: "*"
    };
  }
}

message ListResourcesRequest {
  string workspace_id = 1 [(prefab.authz.id) = true];
}

message ListResourcesResponse {
  repeated Resource resources = 1;
}

message GetResourceRequest {
  string workspace_id = 1 [(prefab.authz.domain) = true];
  string resource_id = 2 [(prefab.authz.id) = true];
}

message GetResourceResponse {
  Resource resource = 1;
}

message UpdateResourceRequest {
  string workspace_id = 1 [(prefab.authz.domain) = true];
  string resource_id = 2 [(prefab.authz.id) = true];
  string title = 3;
  string content = 4;
}

message UpdateResourceResponse {
  Resource resource = 1;
}

message Resource {
  string id = 1;
  string title = 2;
  string content = 3;
  string owner_id = 4;
}
```

#### Understanding Prefab's Authorization Proto Options

Prefab's authorization system uses proto options to define access control rules directly in your service definitions:

1. **Method Options**:

   - `(prefab.authz.action)`: Specifies the action being performed (e.g., "resources.view", "resources.update"). Actions are used in policy rules to control who can perform them.
   - `(prefab.authz.resource)`: Defines the type of resource being accessed. This maps to the object fetcher registered with `WithObjectFetcher()`.
   - `(prefab.authz.default_effect)`: Sets the default effect if no policy matches:
     - `"allow"`: Allow access by default unless explicitly denied
     - `"deny"`: Deny access by default unless explicitly allowed

2. **Field Options**:

   - `[(prefab.authz.id) = true]`: Marks a field as containing the resource identifier. The value will be passed to the appropriate object fetcher.
   - `[(prefab.authz.domain) = true]`: Marks a field as containing the domain identifier. This allows for domain-scoped permissions. A domain might be a workspace, organization, or folder.

3. **Authorization Flow**:
   1. When an RPC is called, Prefab extracts the action from the method options
   2. It gets the resource ID from fields marked with `[(prefab.authz.id) = true]`
   3. It gets the domain from fields marked with `[(prefab.authz.domain) = true]`
   4. It fetches the resource object using the registered object fetcher
   5. It calls the role describer to determine the user's roles for that object and domain
   6. It checks if any policy allows or denies the action for the user's roles
   7. It applies the default effect if no policy matches

This declarative approach allows authorization rules to be clearly defined at the API design level while implementation details are handled by the server code.

#### Server Code Setup

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/auth"
    "github.com/dpup/prefab/plugins/authz"
)

// Define roles
const (
    roleUser  = authz.Role("user")
    roleAdmin = authz.Role("admin")
    roleOwner = authz.Role("owner")
)

// Set up authorization policies
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(pwdauth.Plugin(...)),
    prefab.WithPlugin(authz.Plugin(
        // Allow users to read resources
        authz.WithPolicy(authz.Allow, roleUser, authz.Action("resources.list")),
        authz.WithPolicy(authz.Allow, roleUser, authz.Action("resources.view")),

        // Allow owners to modify resources
        authz.WithPolicy(authz.Allow, roleOwner, authz.Action("resources.update")),

        // Allow admins to do everything
        authz.WithPolicy(authz.Allow, roleAdmin, authz.Action("*")),

        // Define object fetchers for authorization
        authz.WithObjectFetcher("resource", fetchResource),

        // Define role describers to determine user roles
        authz.WithRoleDescriber("*", roleDescriber),
    )),
)

// Resource fetcher implementation
func fetchResource(ctx context.Context, key any) (any, error) {
    resourceID := key.(string)
    // Fetch the resource from your database
    resource, err := db.GetResourceByID(resourceID)
    if err != nil {
        return nil, err
    }
    return resource, nil
}

// Role describer implementation - determines roles for a user on an object
func roleDescriber(ctx context.Context, identity auth.Identity, object any, domain authz.Domain) ([]authz.Role, error) {
    // All authenticated users have the "user" role
    roles := []authz.Role{roleUser}

    // Check if user is an admin
    isAdmin, err := isUserAdmin(identity.Subject)
    if err != nil {
        return nil, err
    }
    if isAdmin {
        roles = append(roles, roleAdmin)
    }

    // Check if user is the owner of this resource
    if resource, ok := object.(Resource); ok {
        if resource.OwnerID == identity.Subject {
            roles = append(roles, roleOwner)
        }
    }

    return roles, nil
}
```

### Storage

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/storage"
    "github.com/dpup/prefab/plugins/storage/memstore"
    "github.com/dpup/prefab/plugins/storage/sqlite"
)

// In-memory storage (for testing)
s := prefab.New(
    prefab.WithPlugin(storage.Plugin(memstore.New())),
)

// SQLite storage (for development or lightweight production)
s := prefab.New(
    prefab.WithPlugin(storage.Plugin(sqlite.New("database.db"))),
)
```

## Configuration

### Via YAML

```yaml
# config.yaml
server:
  host: 0.0.0.0
  port: 8080

auth:
  signingKey: your-secret-key
  expiration: 24h

  google:
    id: your-google-client-id
    secret: your-google-client-secret
```

```go
s := prefab.New(
    prefab.WithConfigFile("./config.yaml"),
)
```

### Via Environment Variables

```bash
export SERVER_PORT=9000
export AUTH_SIGNING_KEY=your-secret-key
export AUTH_GOOGLE_ID=your-google-client-id
export AUTH_GOOGLE_SECRET=your-google-client-secret
```

### Via Functional Options

```go
s := prefab.New(
    prefab.WithPort(8080),
    prefab.WithSecurityHeaders(prefab.SecurityHeaders{
        XFrameOptions: "DENY",
        HStsExpiration: 31536000 * time.Second,
    }),
)
```

## Custom Plugins

```go
type myPlugin struct {
    // Plugin state
    dependency anotherPlugin
}

// Plugin name for dependency resolution
func (p *myPlugin) Name() string {
    return "myplugin"
}

// Specify required dependencies
func (p *myPlugin) Deps() []string {
    return []string{"anotherplugin"}
}

// Add server options
func (p *myPlugin) ServerOptions() []prefab.ServerOption {
    return []prefab.ServerOption{
        prefab.WithGRPCInterceptor(p.interceptor),
    }
}

// Initialize plugin
func (p *myPlugin) Init(ctx context.Context, r *prefab.Registry) error {
    p.dependency = r.Get("anotherplugin").(anotherPlugin)
    return nil
}

// Register plugin
s := prefab.New(
    prefab.WithPlugin(&myPlugin{}),
)
```

## Security Best Practices

1. **CSRF Protection**: Use proto options to control CSRF protection:

   ```proto
   rpc CreateResource(Request) returns (Response) {
     option (csrf_mode) = "on";
     option (google.api.http) = {
       post: "/api/resources"
       body: "*"
     };
   }
   ```

2. **Authentication Security**:

   - Use strong signing keys
   - Set appropriate token expiration
   - Enable token revocation with storage plugin
   - Use HTTPS in production

3. **Security Headers**:
   ```yaml
   server:
     security:
       xFrameOptions: DENY
       hstsExpiration: 31536000s
       hstsIncludeSubdomains: true
       corsOrigins:
         - https://app.example.com
   ```

## For More Information

See the full documentation in the `/docs` directory:

- [Quickstart Guide](quickstart.md)
- [Getting Started](getting-started.md)
- [Plugins](plugins.md)
- [Configuration](configuration.md)
- [Security](security.md)
