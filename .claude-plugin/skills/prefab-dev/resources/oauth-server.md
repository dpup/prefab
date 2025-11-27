# OAuth Server

Prefab can act as an OAuth2 authorization server, allowing third-party applications to authenticate users and access protected resources with scoped permissions.

## Basic Setup

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/auth"
    "github.com/dpup/prefab/plugins/oauth"
)

func main() {
    oauthPlugin := oauth.NewBuilder().
        WithClient(oauth.Client{
            ID:           "my-app",
            Secret:       "client-secret",
            Name:         "My Application",
            RedirectURIs: []string{"https://myapp.com/callback"},
            Scopes:       []string{"read", "write"},
        }).
        WithAccessTokenExpiry(time.Hour).
        WithRefreshTokenExpiry(14 * 24 * time.Hour).
        WithIssuer("https://auth.example.com").
        Build()

    server := prefab.New(
        prefab.WithPlugin(auth.Plugin()),
        prefab.WithPlugin(oauthPlugin),
    )

    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## OAuth Endpoints

The plugin automatically registers these endpoints:

| Endpoint | Description |
|----------|-------------|
| `/oauth/authorize` | Authorization endpoint for user consent |
| `/oauth/token` | Token endpoint for exchanging codes/credentials |
| `/.well-known/oauth-authorization-server` | Server metadata (RFC 8414) |

## Client Configuration

### Confidential Clients (Server-side apps)

```go
oauth.Client{
    ID:           "server-app",
    Secret:       "strong-secret-here",
    Name:         "Server Application",
    RedirectURIs: []string{"https://app.example.com/callback"},
    Scopes:       []string{"read", "write", "admin"},
    Public:       false,
}
```

### Public Clients (SPAs, Mobile apps)

```go
oauth.Client{
    ID:           "spa-app",
    Secret:       "",  // No secret for public clients
    Name:         "Single Page App",
    RedirectURIs: []string{
        "http://localhost:3000/callback",
        "https://spa.example.com/callback",
    },
    Scopes:       []string{"read", "write"},
    Public:       true,
}
```

### Scope Validation

Clients can only request scopes listed in their `Scopes` field. If `Scopes` is empty, all scopes are allowed.

## Supported Grant Types

- **Authorization Code**: For user-facing applications
- **Client Credentials**: For server-to-server communication
- **Refresh Token**: For refreshing expired access tokens

## Using OAuth Scopes in Handlers

```go
func (s *Server) GetData(ctx context.Context, req *pb.GetDataRequest) (*pb.Data, error) {
    // Check if this is an OAuth request
    if oauth.IsOAuthRequest(ctx) {
        // Verify required scope
        if !oauth.HasScope(ctx, "read") {
            return nil, errors.NewC("insufficient_scope", codes.PermissionDenied)
        }
    }

    // Get the OAuth client ID if needed
    clientID := oauth.OAuthClientIDFromContext(ctx)

    // Get all scopes
    scopes := oauth.OAuthScopesFromContext(ctx)

    // ... handle request
}
```

### Scope Helper Functions

```go
// Check single scope
oauth.HasScope(ctx, "read")

// Check if any scope matches
oauth.HasAnyScope(ctx, "read", "write")

// Check if all scopes present
oauth.HasAllScopes(ctx, "read", "write")

// Check if request is via OAuth
oauth.IsOAuthRequest(ctx)

// Parse/format scope strings
scopes := oauth.ParseScopes("read write admin")  // []string{"read", "write", "admin"}
str := oauth.FormatScopes(scopes)                 // "read write admin"
```

## Custom Storage

By default, clients and tokens are stored in memory. For production, implement custom storage.

### Custom Client Store

Implement `oauth.ClientStore` to persist clients in your database:

```go
type ClientStore interface {
    GetClient(ctx context.Context, clientID string) (*Client, error)
    CreateClient(ctx context.Context, client *Client) error
    UpdateClient(ctx context.Context, client *Client) error
    DeleteClient(ctx context.Context, clientID string) error
    ListClientsByUser(ctx context.Context, userID string) ([]*Client, error)
}
```

Example implementation:

```go
type PostgresClientStore struct {
    db *sql.DB
}

func (s *PostgresClientStore) GetClient(ctx context.Context, clientID string) (*oauth.Client, error) {
    var client oauth.Client
    var redirectURIs, scopes string

    err := s.db.QueryRowContext(ctx, `
        SELECT id, secret, name, redirect_uris, scopes, public, created_by, created_at
        FROM oauth_clients WHERE id = $1
    `, clientID).Scan(
        &client.ID, &client.Secret, &client.Name,
        &redirectURIs, &scopes,
        &client.Public, &client.CreatedBy, &client.CreatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, oauth.ErrInvalidClient
    }
    if err != nil {
        return nil, err
    }

    client.RedirectURIs = strings.Split(redirectURIs, ",")
    client.Scopes = strings.Split(scopes, " ")
    return &client, nil
}

func (s *PostgresClientStore) CreateClient(ctx context.Context, client *oauth.Client) error {
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO oauth_clients (id, secret, name, redirect_uris, scopes, public, created_by, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `,
        client.ID, client.Secret, client.Name,
        strings.Join(client.RedirectURIs, ","),
        strings.Join(client.Scopes, " "),
        client.Public, client.CreatedBy, client.CreatedAt,
    )
    return err
}

// ... implement UpdateClient, DeleteClient, ListClientsByUser
```

### Custom Token Store

Implement `oauth.TokenStore` to persist tokens:

```go
type TokenStore interface {
    Create(ctx context.Context, info TokenInfo) error
    RemoveByCode(ctx context.Context, code string) error
    RemoveByAccess(ctx context.Context, access string) error
    RemoveByRefresh(ctx context.Context, refresh string) error
    GetByCode(ctx context.Context, code string) (TokenInfo, error)
    GetByAccess(ctx context.Context, access string) (TokenInfo, error)
    GetByRefresh(ctx context.Context, refresh string) (TokenInfo, error)
}
```

The `TokenInfo` struct contains all token data:

```go
type TokenInfo struct {
    ClientID            string
    UserID              string
    Scope               string
    Code                string
    CodeCreateAt        time.Time
    CodeExpiresIn       time.Duration
    CodeChallenge       string
    CodeChallengeMethod string
    Access              string
    AccessCreateAt      time.Time
    AccessExpiresIn     time.Duration
    Refresh             string
    RefreshCreateAt     time.Time
    RefreshExpiresIn    time.Duration
    RedirectURI         string
}
```

Example Redis implementation:

```go
type RedisTokenStore struct {
    client *redis.Client
}

func (s *RedisTokenStore) Create(ctx context.Context, info oauth.TokenInfo) error {
    data, _ := json.Marshal(info)

    pipe := s.client.Pipeline()

    if info.Code != "" {
        pipe.Set(ctx, "oauth:code:"+info.Code, data, info.CodeExpiresIn)
    }
    if info.Access != "" {
        pipe.Set(ctx, "oauth:access:"+info.Access, data, info.AccessExpiresIn)
    }
    if info.Refresh != "" {
        pipe.Set(ctx, "oauth:refresh:"+info.Refresh, data, info.RefreshExpiresIn)
    }

    _, err := pipe.Exec(ctx)
    return err
}

func (s *RedisTokenStore) GetByAccess(ctx context.Context, access string) (oauth.TokenInfo, error) {
    data, err := s.client.Get(ctx, "oauth:access:"+access).Bytes()
    if err == redis.Nil {
        return oauth.TokenInfo{}, oauth.ErrInvalidGrant
    }
    if err != nil {
        return oauth.TokenInfo{}, err
    }

    var info oauth.TokenInfo
    if err := json.Unmarshal(data, &info); err != nil {
        return oauth.TokenInfo{}, err
    }
    return info, nil
}

// ... implement remaining methods
```

### Using Custom Stores

```go
oauthPlugin := oauth.NewBuilder().
    WithClientStore(&PostgresClientStore{db: db}).
    WithTokenStore(&RedisTokenStore{client: redisClient}).
    WithClient(oauth.Client{...}).  // Static clients still work
    Build()
```

## Dynamic Client Registration

Add clients at runtime:

```go
// Get the plugin from registry
oauthPlugin := registry.Get("oauth").(*oauth.OAuthPlugin)

// Add a new client
oauthPlugin.AddClient(oauth.Client{
    ID:           "new-client",
    Secret:       generateSecret(),
    Name:         "Dynamically Registered App",
    RedirectURIs: []string{"https://newapp.com/callback"},
    Scopes:       []string{"read"},
    CreatedBy:    userID,
    CreatedAt:    time.Now(),
})

// Or use the store directly for full CRUD
store := oauthPlugin.GetClientStore()
store.CreateClient(ctx, &client)
store.UpdateClient(ctx, &client)
store.DeleteClient(ctx, clientID)
clients, _ := store.ListClientsByUser(ctx, userID)
```

## Token Expiration Configuration

```go
oauth.NewBuilder().
    WithAccessTokenExpiry(time.Hour).           // Access tokens valid for 1 hour
    WithRefreshTokenExpiry(14 * 24 * time.Hour). // Refresh tokens valid for 2 weeks
    WithAuthCodeExpiry(10 * time.Minute).        // Auth codes valid for 10 minutes
    Build()
```

## Testing OAuth Flows

### Client Credentials Flow

```bash
curl -X POST http://localhost:8080/oauth/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=client-secret" \
  -d "scope=read write"
```

### Authorization Code Flow

1. Redirect user to authorize endpoint:
```
http://localhost:8080/oauth/authorize?
  client_id=my-app&
  response_type=code&
  redirect_uri=https://myapp.com/callback&
  scope=read%20write&
  state=random-state
```

2. After user authorizes, exchange code for token:
```bash
curl -X POST http://localhost:8080/oauth/token \
  -d "grant_type=authorization_code" \
  -d "code=AUTH_CODE_HERE" \
  -d "client_id=my-app" \
  -d "client_secret=client-secret" \
  -d "redirect_uri=https://myapp.com/callback"
```

### Using Access Tokens

```bash
curl -H "Authorization: Bearer ACCESS_TOKEN" \
  http://localhost:8080/api/protected
```

## Example: Complete OAuth Server

See `examples/oauthserver/main.go` for a complete working example with:
- Multiple client configurations
- Protected endpoints with scope checks
- Interactive web UI for testing flows
