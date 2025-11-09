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
    s.RegisterService(
        &yourservice.YourService_ServiceDesc,
        yourservice.RegisterYourServiceHandler,
        &yourServiceImpl{},
    )

    // Start the server - this blocks until shutdown
    if err := s.Start(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Server-Sent Events (SSE)

Bridge gRPC streaming services to web clients using Server-Sent Events. Prefab automatically handles connection management, stream reading, and event formatting.

### Basic Usage

```go
s := prefab.New(
    prefab.WithGRPCService(&CounterService_ServiceDesc, counterService),

    prefab.WithSSEStream(
        "/counter",
        func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (CounterService_StreamClient, error) {
            client := NewCounterServiceClient(cc)
            return client.Stream(ctx, &CounterRequest{})
        },
    ),
)
```

Prefab automatically handles stream reading, protobuf-to-JSON conversion, SSE formatting, and cleanup when clients disconnect.

### Path Parameters

```go
prefab.WithSSEStream(
    "/notes/{id}/updates",
    func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (NotesStreamService_StreamUpdatesClient, error) {
        client := NewNotesStreamServiceClient(cc)
        return client.StreamUpdates(ctx, &StreamRequest{NoteId: params["id"]})
    },
)
```

### Query Parameters

Access query parameters as `params["query.paramName"]`:

```go
req := &StreamRequest{NoteId: params["id"]}
if since := params["query.since"]; since != "" {
    req.Since = parseTimestamp(since)
}
```

### Client Usage

**JavaScript:**
```javascript
const eventSource = new EventSource('http://localhost:8080/counter');
eventSource.onmessage = (event) => {
    console.log('Received:', JSON.parse(event.data));
};
```

**curl:**
```bash
curl -N http://localhost:8080/counter
```

See `examples/ssestream/` for a complete working example.

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

The authz plugin provides declarative, proto-based access control. See [authz.md](authz.md) for complete documentation.

#### Proto Annotations

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

#### Server Setup

```go
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

        // Object Fetcher: Convert ID → Object
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

#### Authorization Flow

When an RPC is invoked:
1. Extract action, resource type, ID, and scope from proto annotations
2. Fetch object using registered Object Fetcher
3. Determine user roles using registered Role Describer
4. Evaluate policies using AWS IAM-style precedence (Deny > Allow > Default)
5. Grant or deny access

#### Common Patterns

**Object Fetchers:**
```go
authz.Fetcher(db.GetDocByID)                     // Database fetch
authz.MapFetcher(staticDocs)                     // Static map
authz.ComposeFetchers(cache, db, api)            // Fallback chain
authz.ValidatedFetcher(fetcher, validateFunc)    // Add validation
```

**Role Describers:**
```go
authz.OwnershipRole(role, getOwnerID)              // Grant if user owns resource
authz.IdentityOwnershipRole(role, resolve, getOwnerID) // Async identity→owner resolution
authz.MembershipRoles(getScopeID, getRoles)        // Grant roles from scope (validates scope)
authz.StaticRole(role, predicate)                  // Grant based on condition
authz.Compose(describer1, describer2, ...)         // Combine multiple describers
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

Prefab provides flexible configuration through YAML files, environment variables, and functional options. All configuration is managed through a global `prefab.Config` instance powered by Koanf.

### Via YAML

```yaml
# config.yaml or prefab.yaml
server:
  host: 0.0.0.0
  port: 8080

auth:
  signingKey: your-secret-key
  expiration: 24h

  google:
    id: your-google-client-id
    secret: your-google-client-secret

# Add your own application-specific configuration
myapp:
  cacheRefreshInterval: 5m
  maxRetries: 3
  enableFeatureX: true
```

```go
prefab.LoadConfigFile("./config.yaml")

s := prefab.New()

// Access your custom config anywhere in your code
interval := prefab.ConfigDuration("myapp.cacheRefreshInterval")
retries := prefab.ConfigInt("myapp.maxRetries")
enabled := prefab.ConfigBool("myapp.enableFeatureX")
```

### Via Environment Variables

Environment variables use the `PF__` prefix with double underscores for nesting:

```bash
# Prefab configuration
export PF__SERVER__PORT=9000
export PF__AUTH__SIGNING_KEY=your-secret-key
export PF__AUTH__GOOGLE__ID=your-google-client-id
export PF__AUTH__GOOGLE__SECRET=your-google-client-secret

# Your application configuration
export PF__MYAPP__CACHE_REFRESH_INTERVAL=10m
export PF__MYAPP__MAX_RETRIES=5
export PF__MYAPP__ENABLE_FEATURE_X=true
```

Environment variable naming convention:
- Double underscores (`__`) separate config levels: `PF__SERVER__PORT` → `server.port`
- Single underscores (`_`) within a segment become camelCase:
  - `PF__SERVER__INCOMING_HEADERS` → `server.incomingHeaders`
  - `PF__FOO_BAR__BAZ` → `fooBar.baz`

**⚠️ Warning**: Environment variable transformation works like this:
- Env var `PF__MYAPP__MAX_RETRIES` → config key `myapp.maxRetries` (camelCase)
- Env var `PF__MYAPP__MAXRETRIES` → config key `myapp.maxretries` (lowercase)

If your YAML uses snake_case, it converts to camelCase internally, so:
- YAML `max_retries` → internal key `maxRetries` → env var `PF__MYAPP__MAX_RETRIES`

**Best practice**: Use snake_case in YAML so the structure matches environment variables.

### Via Functional Options

```go
s := prefab.New(
    // Prefab options
    prefab.WithPort(8080),
    prefab.WithSecurityHeaders(prefab.SecurityHeaders{
        XFrameOptions: "DENY",
        HStsExpiration: 31536000 * time.Second,
    }),

)

// Load application config before creating server
prefab.LoadConfigDefaults(map[string]interface{}{
    "myapp.cacheRefreshInterval": "5m",
    "myapp.maxRetries": 3,
    "myapp.enableFeatureX": true,
})
prefab.LoadConfigFile("./app.yaml")
```

### Configuration Hierarchy

Configuration is **process-global** and loaded eagerly. Sources are applied in this order (later sources override earlier):

1. Prefab's built-in defaults (loaded in `init()`)
2. Auto-discovered `prefab.yaml` (loaded in `init()`)
3. Environment variables with PF__ prefix (loaded in `init()`)
4. Application defaults (loaded immediately via `WithConfigDefaults()`)
5. Additional config files (loaded immediately via `WithConfigFile()`)
6. Functional options (applied during server construction)

**Important**: `WithConfigDefaults()` and `WithConfigFile()` load config **immediately** when called, making values available right away before `s.Start()`.

### Extending Configuration

Applications can easily add their own configuration using the same system:

```go
func main() {
    // Set application defaults
    prefab.LoadConfigDefaults(map[string]interface{}{
        "myapp.database.host": "localhost",
        "myapp.database.port": 5432,
        "myapp.cacheRefreshInterval": "5m",
    })

    // Load config file (can override defaults)
    prefab.LoadConfigFile("./config.yaml")

    // Create server
    s := prefab.New()

    // Access config anywhere
    dbHost := prefab.ConfigString("myapp.database.host")
    dbPort := prefab.ConfigInt("myapp.database.port")
    cacheInterval := prefab.ConfigDuration("myapp.cacheRefreshInterval")

    // ... register services ...

    if err := s.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Configuration Validation

Prefab automatically validates critical configuration at startup. You can also add validation for your own config:

```go
func main() {
    // Load configuration
    prefab.LoadConfigDefaults(map[string]interface{}{
        "myapp.apiKey": "",  // Will be required
        "myapp.timeout": 30,
    })

    // Validate required configuration
    apiKey := prefab.ConfigMustString("myapp.apiKey",
        "Set PF__MYAPP__API_KEY environment variable")

    // Validate with range checking
    timeout := prefab.ConfigMustInt("myapp.timeout", 1, 300)

    // Custom validation
    port := prefab.ConfigInt("myapp.database.port")
    if err := prefab.ValidatePort(port); err != nil {
        panic(fmt.Sprintf("myapp.database.port: %v", err))
    }

    // Create server (also performs automatic validation)
    s := prefab.New()

    // ... rest of setup ...
}
```

**Best practices:**
- Use a consistent namespace prefix (e.g., `myapp.`) for your configuration
- Provide sensible defaults via `LoadConfigDefaults()`
- Use YAML for environment-specific config
- Use environment variables for secrets and deployment overrides
- **Validate required config on startup** using `ConfigMust*` functions
- Use `ValidatePort()`, `ValidateURL()` etc. for common validations
- For testable code, inject config values as dependencies rather than reading from the global config in business logic

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
    // Option 1: Get by name with type assertion
    p.dependency = r.Get("anotherplugin").(anotherPlugin)

    // Option 2: Get by type
    dep, ok := prefab.GetPlugin[anotherPlugin](r)
    if !ok {
        return fmt.Errorf("failed to get anotherplugin")
    }
    p.dependency = dep

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

## Error Handling

Prefab provides an enhanced error package that integrates with the logging system, automatically tracking errors with stack traces, gRPC codes, HTTP status codes, and custom log fields.

### Basic Error Creation

```go
import (
    "github.com/dpup/prefab/errors"
    "google.golang.org/grpc/codes"
)

// Simple error with stack trace
err := errors.New("something went wrong")

// Error with gRPC code
err := errors.NewC("invalid input", codes.InvalidArgument)

// Wrap existing error
err := errors.Wrap(existingErr, 0)

// Add gRPC code to existing error
err = errors.WithCode(err, codes.NotFound)

// Set user-presentable message (different from internal error)
err = errors.WithUserPresentableMessage(err, "Resource not found")

// Override HTTP status code
err = errors.WithHTTPStatusCode(err, 404)
```

### Adding Log Fields to Errors

You can attach structured log fields to errors, which the logging middleware will automatically unpack when the error is logged:

```go
// Add a single field
err := errors.New("database connection failed").
    WithLogField("database", "users_db")

// Add multiple fields
err := errors.New("payment processing failed").WithLogFields(map[string]interface{}{
    "user_id":    req.UserId,
    "payment_id": payment.ID,
    "amount":     payment.Amount,
    "currency":   payment.Currency,
})

// Chain with other error methods
err := errors.NewC("validation failed", codes.InvalidArgument).
    WithLogField("field", "email").
    WithLogField("value", req.Email).
    WithUserPresentableMessage("Invalid email address")

// Add fields to existing errors
if err := processPayment(payment); err != nil {
    return errors.WithLogField(err, "payment_id", payment.ID)
}
```

### How Log Fields Work

When an error with log fields is returned from an RPC handler:

1. The error interceptor automatically calls `trackError()`
2. Standard error fields are tracked: `error.type`, `error.http_status`, `error.message`, `error.stack_trace`
3. **Custom log fields are automatically unpacked** and added to the logging context
4. When the middleware logs the error, all fields appear in the structured log output

**Example flow:**

```go
func (s *Server) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.Order, error) {
    order, err := s.db.CreateOrder(req)
    if err != nil {
        // Add context to the error
        return nil, errors.Wrap(err, 0).
            WithCode(codes.Internal).
            WithLogField("user_id", req.UserId).
            WithLogField("order_type", req.Type).
            WithLogField("retry_count", s.retryCount).
            WithUserPresentableMessage("Failed to create order")
    }
    return order, nil
}
```

**Resulting log output** (when error occurs):

```json
{
  "level": "error",
  "msg": "finished call with code Internal",
  "error.type": "*errors.Error",
  "error.http_status": 500,
  "error.message": "database connection timeout",
  "error.stack_trace": ["server.go:123", "handler.go:45", ...],
  "user_id": "usr_123",
  "order_type": "subscription",
  "retry_count": 3
}
```

### Common Patterns

**Progressive Error Enrichment:**
```go
func processOrder(ctx context.Context, orderID string) error {
    err := fetchOrder(orderID)
    if err != nil {
        err = errors.WithLogField(err, "order_id", orderID)
    }

    err = validateOrder(order)
    if err != nil {
        err = errors.WithLogField(err, "validation_step", "inventory_check")
        return err
    }

    return nil
}
```

**Database Errors:**
```go
func (s *Server) GetDocument(ctx context.Context, req *pb.GetDocRequest) (*pb.Document, error) {
    doc, err := s.db.GetDocument(req.DocumentId)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.NewC("document not found", codes.NotFound).
                WithLogField("document_id", req.DocumentId).
                WithUserPresentableMessage("Document not found")
        }
        return nil, errors.Wrap(err, 0).
            WithCode(codes.Internal).
            WithLogField("document_id", req.DocumentId).
            WithLogField("database", "documents").
            WithUserPresentableMessage("Failed to retrieve document")
    }
    return doc, nil
}
```

**External API Errors:**
```go
func (s *Server) ProcessPayment(ctx context.Context, req *pb.PaymentRequest) (*pb.Payment, error) {
    resp, err := s.paymentClient.Charge(ctx, &payment.ChargeRequest{
        Amount:   req.Amount,
        Currency: req.Currency,
    })
    if err != nil {
        return nil, errors.Wrap(err, 0).
            WithCode(codes.Internal).
            WithLogFields(map[string]interface{}{
                "provider":      "stripe",
                "amount":        req.Amount,
                "currency":      req.Currency,
                "customer_id":   req.CustomerId,
                "attempt_count": s.attemptCount,
            }).
            WithUserPresentableMessage("Payment processing failed")
    }
    return resp, nil
}
```

**Validation Errors:**
```go
func validateUser(user *User) error {
    if user.Email == "" {
        return errors.NewC("email is required", codes.InvalidArgument).
            WithLogField("user_id", user.ID).
            WithLogField("validation_field", "email").
            WithUserPresentableMessage("Email address is required")
    }

    if !emailRegex.MatchString(user.Email) {
        return errors.NewC("invalid email format", codes.InvalidArgument).
            WithLogField("user_id", user.ID).
            WithLogField("validation_field", "email").
            WithLogField("email_value", user.Email).
            WithUserPresentableMessage("Invalid email address format")
    }

    return nil
}
```

### Best Practices

1. **Add contextual fields** that help with debugging and observability
2. **Don't log sensitive data** (passwords, tokens, etc.) in error fields
3. **Use consistent field names** across your application (e.g., `user_id`, not `userId` in one place and `user_id` in another)
4. **Add fields at the point of error creation** rather than wrapping multiple times
5. **Use meaningful field names** that will be searchable in logs
6. **Separate user-facing messages** from internal error details using `WithUserPresentableMessage()`

## For More Information

See the full documentation in the `/docs` directory:

- [Quickstart Guide](quickstart.md)
- [Getting Started](getting-started.md)
- [Plugins](plugins.md)
- [Configuration](configuration.md)
- [Security](security.md)
