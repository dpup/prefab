# Configuration

Prefab provides flexible configuration through YAML files, environment variables, and functional options.

## Via YAML

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

// Access your custom config anywhere
interval := prefab.ConfigDuration("myapp.cacheRefreshInterval")
retries := prefab.ConfigInt("myapp.maxRetries")
enabled := prefab.ConfigBool("myapp.enableFeatureX")
```

## Via Environment Variables

Environment variables use the `PF__` prefix with double underscores for nesting:

```bash
# Prefab configuration
export PF__SERVER__PORT=9000
export PF__AUTH__SIGNING_KEY=your-secret-key
export PF__AUTH__GOOGLE__ID=your-google-client-id

# Your application configuration
export PF__MYAPP__CACHE_REFRESH_INTERVAL=10m
export PF__MYAPP__MAX_RETRIES=5
```

### Naming Convention

- Double underscores (`__`) separate config levels: `PF__SERVER__PORT` → `server.port`
- Single underscores (`_`) within a segment become camelCase:
  - `PF__SERVER__INCOMING_HEADERS` → `server.incomingHeaders`
  - `PF__FOO_BAR__BAZ` → `fooBar.baz`

**Best practice**: Use snake_case in YAML so the structure matches environment variables.

## Via Functional Options

```go
s := prefab.New(
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
})
prefab.LoadConfigFile("./app.yaml")
```

## Configuration Hierarchy

Sources are applied in this order (later overrides earlier):

1. Prefab's built-in defaults
2. Auto-discovered `prefab.yaml`
3. Environment variables with `PF__` prefix
4. Application defaults via `LoadConfigDefaults()`
5. Additional config files via `LoadConfigFile()`
6. Functional options

## Configuration Validation

```go
func main() {
    prefab.LoadConfigDefaults(map[string]interface{}{
        "myapp.apiKey": "",
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

    s := prefab.New()
    // ...
}
```

## Best Practices

- Use a consistent namespace prefix (e.g., `myapp.`) for your configuration
- Provide sensible defaults via `LoadConfigDefaults()`
- Use YAML for environment-specific config
- Use environment variables for secrets and deployment overrides
- Validate required config on startup using `ConfigMust*` functions
- For testable code, inject config values as dependencies

For complete documentation, see [/docs/configuration.md](/docs/configuration.md).
