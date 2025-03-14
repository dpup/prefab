# Configuring Prefab

Prefab provides multiple ways to configure your server:

1. Functional options when creating the server
2. Configuration files 
3. Environment variables

## Functional Options

The most direct way to configure Prefab is with functional options when creating the server:

```go
s := prefab.New(
    prefab.WithHost("0.0.0.0"),
    prefab.WithPort(8080),
    prefab.WithHTTPHandler("/custom", myHandler),
    prefab.WithStaticFiles("/static/", "./static/"),
    prefab.WithPlugin(myPlugin),
    // many other options available
)
```

## Configuration Files

Prefab supports configuration via YAML files:

```go
s := prefab.New(
    prefab.WithConfigFile("./config.yaml"),
)
```

Example `config.yaml`:

```yaml
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

## Environment Variables

Prefab automatically reads configuration from environment variables. The naming convention maps configuration keys to environment variables:

```
server.host → SERVER_HOST
auth.google.clientId → AUTH_GOOGLE_ID
```

Example:

```bash
export SERVER_PORT=9000
export AUTH_SIGNING_KEY=secret-key
export AUTH_GOOGLE_ID=google-client-id
export AUTH_GOOGLE_SECRET=google-client-secret
```

## Configuration Order

When multiple configuration sources are used, they are applied in this order (later sources override earlier ones):

1. Default values
2. Configuration files
3. Environment variables
4. Functional options

This allows you to have a base configuration file and override specific values with environment variables or functional options.

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