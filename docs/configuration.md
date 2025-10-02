# Configuring Prefab

Prefab provides multiple ways to configure your server:

1. Configuration files (YAML)
2. Environment variables
3. Functional options when creating the server

All configuration is managed through a global `prefab.Config` instance powered by [Koanf](https://github.com/knadh/koanf), which provides a flexible, layered configuration system.

## Configuration Hierarchy

Configuration is **process-global** and loaded eagerly. Sources are applied in this order (later sources override earlier ones):

1. **Prefab's built-in defaults** - Loaded in `init()`, sensible defaults for all prefab settings
2. **Auto-discovered `prefab.yaml`** - Loaded in `init()`, automatically found by searching up the directory tree
3. **Environment variables** - Loaded in `init()`, using the `PF__` prefix
4. **Application defaults** - Loaded via `LoadConfigDefaults()` before creating server
5. **Additional config files** - Loaded via `LoadConfigFile()` before creating server
6. **Functional options** - Options like `WithPort(8080)` applied during server construction

**Important**: Call `LoadConfigDefaults()` and `LoadConfigFile()` **before** creating the server:

```go
// Load config first
prefab.LoadConfigDefaults(map[string]interface{}{
    "myapp.setting": "value",
})
prefab.LoadConfigFile("./app.yaml")

// Then create server
s := prefab.New()

// Config is available
value := prefab.ConfigString("myapp.setting")
```

For testable code, prefer **dependency injection** of config values rather than directly reading from the global config in your business logic.

## Configuration Files

### Auto-Discovery

Prefab automatically searches for a `prefab.yaml` file starting in the current directory and walking up the directory tree. This file is loaded automatically.

```yaml
# prefab.yaml
server:
  host: 0.0.0.0
  port: 8080

auth:
  signingKey: my-secret-key
  expiration: 24h

email:
  from: noreply@example.com
  smtp:
    host: smtp.example.com
    port: 587
    username: user
    password: pass
```

### Additional Config Files

You can load additional configuration files using `LoadConfigFile()` before creating the server:

```go
prefab.LoadConfigFile("./config.yaml")
prefab.LoadConfigFile("./secrets.yaml")  // Can load multiple files

s := prefab.New()
```

This is useful for:
- Separating application config from prefab config
- Loading environment-specific config files
- Keeping secrets in a separate file

## Environment Variables

Prefab automatically reads configuration from environment variables with the `PF__` prefix.

### Naming Convention

Environment variables use the `PF__` prefix and are transformed to dot notation config keys.

**Double underscores (`__`)** separate nested config levels:

| Config Key | Environment Variable |
|------------|---------------------|
| `server.host` | `PF__SERVER__HOST` |
| `server.port` | `PF__SERVER__PORT` |
| `auth.signingKey` | `PF__AUTH__SIGNING_KEY` |
| `auth.google.id` | `PF__AUTH__GOOGLE__ID` |
| `myapp.database.host` | `PF__MYAPP__DATABASE__HOST` |

**Single underscores (`_`)** within a segment are converted to camelCase:

| Environment Variable | Config Key |
|---------------------|------------|
| `PF__SERVER__INCOMING_HEADERS` | `server.incomingHeaders` |
| `PF__MY_APP__CACHE_INTERVAL` | `myApp.cacheInterval` |
| `PF__FOO_BAR__BAZ` | `fooBar.baz` |

**⚠️ Important**: Understand how environment variables map to config keys:

```bash
# If your YAML has camelCase keys:
# myapp:
#   maxRetries: 3

# Then the env var needs underscores before capitals:
export PF__MYAPP__MAX_RETRIES=5  # Correct - maps to myapp.maxRetries

# If your YAML has all lowercase:
# myapp:
#   maxretries: 3

# Then the env var has no underscores:
export PF__MYAPP__MAXRETRIES=5  # Correct - maps to myapp.maxretries
```

**Best practice**: Use snake_case keys in YAML to avoid confusion:

```yaml
# Recommended - snake_case in YAML
myapp:
  max_retries: 3        # Becomes maxRetries internally
                        # Env var: PF__MYAPP__MAX_RETRIES

# This way the YAML structure matches the env var structure
```

**Key insight**: YAML snake_case converts to camelCase internally, which is why using snake_case in YAML is recommended:
- YAML: `max_retries: 3`
- Internal key: `maxRetries`
- Env var: `PF__MYAPP__MAX_RETRIES` (underscores convert to camelCase)
- Code: `ConfigInt("myapp.maxRetries")`

This way the YAML structure matches the environment variable structure.

### Example

```bash
# Prefab configuration
export PF__SERVER__PORT=9000
export PF__AUTH__SIGNING_KEY=secret-key
export PF__AUTH__GOOGLE__ID=google-client-id
export PF__AUTH__GOOGLE__SECRET=google-client-secret

# Your application configuration
export PF__MYAPP__DATABASE__HOST=prod-db.example.com
export PF__MYAPP__CACHE_REFRESH_INTERVAL=10m
```

## Functional Options

The most direct way to configure Prefab is with functional options:

```go
s := prefab.New(
    prefab.WithHost("0.0.0.0"),
    prefab.WithPort(8080),
    prefab.WithHTTPHandler("/custom", myHandler),
    prefab.WithStaticFiles("/static/", "./static/"),
    prefab.WithPlugin(myPlugin),
)
```

Functional options override all other configuration sources and are useful for:
- Programmatic configuration
- Testing scenarios
- Values that should never come from config files (like handlers or plugins)

## Extending Configuration for Your Application

One of Prefab's key features is that applications can easily add their own configuration using the same system. This allows your application to benefit from the same YAML/env/functional option support.

### 1. Define Configuration Defaults

Use `LoadConfigDefaults()` to set default values for your application before creating the server:

```go
prefab.LoadConfigDefaults(map[string]interface{}{
    "myapp.database.host": "localhost",
    "myapp.database.port": 5432,
    "myapp.database.name": "myapp_dev",
    "myapp.cacheRefreshInterval": "5m",
    "myapp.maxRetries": 3,
    "myapp.enableFeatureX": false,
})

s := prefab.New()
```

### 2. Add Configuration to YAML Files

Create a config file with your application settings:

```yaml
# config.yaml or prefab.yaml
server:
  port: 8080

# Your application configuration
myapp:
  database:
    host: localhost
    port: 5432
    name: myapp_production

  cacheRefreshInterval: 10m
  maxRetries: 5
  enableFeatureX: true

  features:
    experimental: false
    betaAccess: true
```

Load it before creating the server:

```go
prefab.LoadConfigFile("./config.yaml")

s := prefab.New()

// Config is available
dbHost := prefab.ConfigString("myapp.database.host")
```

### 3. Override with Environment Variables

Users can override any configuration with environment variables:

```bash
export PF__MYAPP__DATABASE__HOST=prod-db.example.com
export PF__MYAPP__CACHE_REFRESH_INTERVAL=15m
export PF__MYAPP__ENABLE_FEATURE_X=true
```

### 4. Access Configuration in Your Code

Access your configuration anywhere using the global `prefab.Config` instance:

```go
import "github.com/dpup/prefab"

func setupDatabase() {
    host := prefab.ConfigString("myapp.database.host")
    port := prefab.ConfigInt("myapp.database.port")
    dbName := prefab.ConfigString("myapp.database.name")

    // Connect to database...
}

func startCacheRefresher() {
    interval := prefab.ConfigDuration("myapp.cacheRefreshInterval")
    ticker := time.NewTicker(interval)
    // ...
}

func isFeatureEnabled() bool {
    return prefab.ConfigBool("myapp.enableFeatureX")
}
```

### Configuration Access Methods

Prefab provides convenient helper functions for accessing configuration:

```go
// String values
apiKey := prefab.ConfigString("myapp.apiKey")

// Numeric values
port := prefab.ConfigInt("myapp.port")
timeout := prefab.ConfigInt64("myapp.timeout")
ratio := prefab.ConfigFloat64("myapp.ratio")

// Boolean values
enabled := prefab.ConfigBool("myapp.featureEnabled")

// Duration values (parses "5m", "1h", "30s", etc.)
interval := prefab.ConfigDuration("myapp.refreshInterval")
// Or require a duration (panics if missing or invalid)
required := prefab.ConfigMustDuration("myapp.required")

// String slices
hosts := prefab.ConfigStrings("myapp.allowedHosts")

// Byte slices
secret := prefab.ConfigBytes("myapp.secret")

// Maps and nested structures
dbConfig := prefab.ConfigStringMap("myapp.database")

// Check if a key exists
if prefab.ConfigExists("myapp.optionalFeature") {
    // ...
}

// Get all config as a map
allConfig := prefab.ConfigAll()

// Advanced: Access the underlying koanf instance directly
prefab.Config.Unmarshal("myapp.database", &myDatabaseConfig)
```

### Complete Example

Here's a complete example showing recommended patterns:

```go
package main

import (
    "log"
    "time"

    "github.com/dpup/prefab"
)

func main() {
    // 1. Set application defaults
    prefab.LoadConfigDefaults(map[string]interface{}{
        "myapp.database.host": "localhost",
        "myapp.database.port": 5432,
        "myapp.cacheRefreshInterval": "5m",
        "myapp.maxRetries": 3,
    })

    // 2. Load environment-specific config
    prefab.LoadConfigFile("./config.yaml")

    // 3. Create server with plugins
    s := prefab.New(
        prefab.WithPlugin(auth.Plugin()),
    )

    // 4. Validate required configuration on startup
    validateConfig()

    // 5. Use config throughout your application
    startBackgroundJobs()

    // 6. Start the server
    if err := s.Start(); err != nil {
        log.Fatal(err)
    }
}

func validateConfig() {
    required := []string{
        "myapp.database.host",
        "myapp.database.name",
    }

    for _, key := range required {
        if !prefab.ConfigExists(key) {
            log.Fatalf("Required configuration missing: %s", key)
        }
    }
}

func startBackgroundJobs() {
    interval := prefab.ConfigDuration("myapp.cacheRefreshInterval")
    go func() {
        ticker := time.NewTicker(interval)
        for range ticker.C {
            refreshCache()
        }
    }()
}

func refreshCache() {
    // Implementation...
}
```

## Configuration and Testability

Configuration in Prefab is **process-global** by design. For testable code:

**❌ Don't** read config directly in business logic:

```go
func ProcessOrder(order Order) error {
    // Hard to test - coupled to global config
    timeout := prefab.ConfigDuration("myapp.orderTimeout")
    return processWithTimeout(order, timeout)
}
```

**✅ Do** inject config values as dependencies:

```go
type OrderProcessor struct {
    timeout time.Duration
}

func NewOrderProcessor() *OrderProcessor {
    return &OrderProcessor{
        timeout: prefab.ConfigDuration("myapp.orderTimeout"),
    }
}

func (p *OrderProcessor) ProcessOrder(order Order) error {
    // Easy to test - timeout can be injected in tests
    return processWithTimeout(order, p.timeout)
}
```

This makes your code testable without needing to manipulate global state.

## Best Practices

### 1. Use Consistent Namespacing

Always prefix your application configuration with a consistent namespace to avoid conflicts:

```yaml
# Good
myapp:
  database:
    host: localhost

# Bad - conflicts with prefab's server config
server:
  myCustomSetting: value
```

### 2. Provide Sensible Defaults

Use `LoadConfigDefaults()` to ensure your application can run with minimal configuration:

```go
prefab.LoadConfigDefaults(map[string]interface{}{
    "myapp.timeout": "30s",
    "myapp.maxConnections": 100,
})
```

### 3. Use Duration Strings

For time-based configuration, use duration strings rather than raw numbers:

```yaml
# Good - clear and explicit
myapp:
  timeout: 30s
  refreshInterval: 5m

# Bad - what unit is this?
myapp:
  timeout: 30
  refreshInterval: 300
```

### 4. Document Your Configuration

Add comments to your YAML files and maintain a configuration reference:

```yaml
myapp:
  # Database connection settings
  database:
    host: localhost  # Database hostname or IP
    port: 5432       # Database port (default: 5432)

  # How often to refresh the cache (default: 5m)
  cacheRefreshInterval: 5m
```

### 5. Separate Secrets

Keep secrets in a separate file that's not checked into version control:

```go
prefab.LoadConfigFile("./config.yaml")      // Checked into git
prefab.LoadConfigFile("./secrets.yaml")     // Not in git (.gitignore)

s := prefab.New()
```

Or use environment variables for secrets in production:

```bash
export PF__MYAPP__DATABASE__PASSWORD=supersecret
export PF__AUTH__SIGNING_KEY=random-key
```

### 6. Validate Configuration on Startup

Don't wait for runtime errors - validate configuration when the server starts:

```go
if !prefab.ConfigExists("myapp.requiredSetting") {
    log.Fatal("myapp.requiredSetting is required")
}

if prefab.ConfigInt("myapp.maxRetries") < 1 {
    log.Fatal("myapp.maxRetries must be at least 1")
}
```

## Common Configuration Options

### Server Configuration

```yaml
server:
  host: 0.0.0.0  # Server bind address
  port: 8080     # Server port
  
  security:
    xFrameOptions: DENY  # X-Frame-Options header
    
    # CORS settings
    corsOrigins:
      - https://app.example.com
    corsAllowedMethods:
      - GET
      - POST
    corsAllowedHeaders:
      - x-csrf-protection
    corsAllowCredentials: true
    corsMaxAge: 72h
    
    # HSTS settings
    hstsExpiration: 31536000s
    hstsIncludeSubdomains: true
    hstsPreload: true
```

### Authentication Configuration

```yaml
auth:
  signingKey: my-signing-key  # Used for JWT tokens
  expiration: 24h             # Token expiration time
  
  # Google OAuth settings
  google:
    id: your-google-client-id
    secret: your-google-client-secret
    redirectURI: https://your-app.com/auth/google/callback
```

### Email Configuration

```yaml
email:
  from: noreply@example.com
  smtp:
    host: smtp.example.com
    port: 587
    username: user
    password: pass
```