# OAuth2 Plugin

The OAuth plugin turns your Prefab server into an OAuth2 authorization server. It supports standard OAuth2 flows for authorizing third-party applications to access user resources.

## Quick Start

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/auth"
    "github.com/dpup/prefab/plugins/oauth"
)

oauthPlugin := oauth.NewBuilder().
    WithClient(oauth.Client{
        ID:           "my-app",
        Secret:       "secret-key",
        Name:         "My Application",
        RedirectURIs: []string{"https://myapp.com/callback"},
        Scopes:       []string{"read", "write"},
    }).
    Build()

server := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(oauthPlugin),
)
```

The OAuth plugin requires the auth plugin to authenticate users during the authorization flow.

## OAuth Flows

### Authorization Code Flow

Standard OAuth2 flow for web and mobile applications. Users authorize access, receive an authorization code, then exchange it for an access token.

1. Redirect user to `/oauth/authorize`:
   ```
   /oauth/authorize?client_id=my-app&response_type=code&redirect_uri=https://myapp.com/callback&scope=read&state=random
   ```

2. User authenticates and authorizes the application

3. Server redirects to callback with authorization code:
   ```
   https://myapp.com/callback?code=AUTH_CODE&state=random
   ```

4. Exchange code for access token:
   ```bash
   curl -X POST http://localhost:8080/oauth/token \
     -d "grant_type=authorization_code" \
     -d "code=AUTH_CODE" \
     -d "client_id=my-app" \
     -d "client_secret=secret-key" \
     -d "redirect_uri=https://myapp.com/callback"
   ```

Response:
```json
{
  "access_token": "ACCESS_TOKEN",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "REFRESH_TOKEN"
}
```

### PKCE (Proof Key for Code Exchange)

Required for public clients (mobile apps, SPAs) when `oauth.enforcePkce` is enabled. PKCE prevents authorization code interception attacks.

1. Generate code verifier and challenge:
   ```javascript
   const verifier = base64url(randomBytes(32));
   const challenge = base64url(sha256(verifier));
   ```

2. Authorization request includes challenge:
   ```
   /oauth/authorize?client_id=my-app&response_type=code&redirect_uri=...&code_challenge=CHALLENGE&code_challenge_method=S256
   ```

3. Token request includes verifier:
   ```bash
   curl -X POST http://localhost:8080/oauth/token \
     -d "grant_type=authorization_code" \
     -d "code=AUTH_CODE" \
     -d "client_id=my-app" \
     -d "code_verifier=VERIFIER"
   ```

### Client Credentials Flow

For server-to-server authentication without user involvement.

```bash
curl -X POST http://localhost:8080/oauth/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=secret-key" \
  -d "scope=read"
```

### Refresh Tokens

Exchange a refresh token for a new access token:

```bash
curl -X POST http://localhost:8080/oauth/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=REFRESH_TOKEN" \
  -d "client_id=my-app" \
  -d "client_secret=secret-key"
```

## Configuration

### Builder Options

```go
oauth.NewBuilder().
    WithClient(client).                          // Add OAuth client
    WithAccessTokenExpiry(time.Hour).            // Default: 1 hour
    WithRefreshTokenExpiry(7 * 24 * time.Hour).  // Default: 14 days
    WithAuthCodeExpiry(10 * time.Minute).        // Default: 10 minutes
    WithIssuer("https://api.example.com").       // Token issuer URL
    WithEnforcePKCE(true).                       // Require PKCE for public clients
    WithClientStore(customStore).                // Custom client storage
    WithTokenStore(customStore).                 // Custom token storage
    Build()
```

### Config Keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `oauth.enforcePkce` | bool | `false` | Require PKCE for public clients |
| `oauth.issuer` | string | `server.address` | Token issuer URL |

## Client Types

### Confidential Clients

Server-side applications that can securely store a client secret.

```go
oauth.Client{
    ID:           "server-app",
    Secret:       "secret-key",
    RedirectURIs: []string{"https://app.com/callback"},
    Scopes:       []string{"read", "write"},
    Public:       false,
}
```

### Public Clients

Browser-based or mobile applications that cannot securely store secrets. Use PKCE for security.

```go
oauth.Client{
    ID:           "mobile-app",
    Secret:       "",  // No secret for public clients
    RedirectURIs: []string{"myapp://callback"},
    Scopes:       []string{"read"},
    Public:       true,
}
```

Public clients:
- Cannot authenticate with a client secret
- Should use PKCE when `oauth.enforcePkce` is enabled
- Tokens are still secure when PKCE is used correctly

## Scope-Based Authorization

Check OAuth scopes in your handlers:

```go
func protectedHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Verify user is authenticated
    identity, err := auth.IdentityFromContext(ctx)
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Check if request uses OAuth
    if oauth.IsOAuthRequest(ctx) {
        // Require specific scope
        if !oauth.HasScope(ctx, "read") {
            http.Error(w, "Missing 'read' scope", http.StatusForbidden)
            return
        }

        // Get OAuth metadata
        clientID := oauth.OAuthClientIDFromContext(ctx)
        scopes := oauth.OAuthScopesFromContext(ctx)
    }

    // Handle request
}
```

Scope helper functions:

```go
oauth.HasScope(ctx, "read")              // Check single scope
oauth.HasAnyScope(ctx, "read", "write")  // Check any of multiple scopes
oauth.HasAllScopes(ctx, "read", "write") // Check all scopes present
oauth.IsOAuthRequest(ctx)                // Check if OAuth token was used
```

## Token Management

### Token Revocation (RFC 7009)

Revoke an access or refresh token:

```bash
curl -X POST http://localhost:8080/oauth/revoke \
  -u "client_id:client_secret" \
  -d "token=ACCESS_TOKEN" \
  -d "token_type_hint=access_token"
```

Clients can only revoke their own tokens. The endpoint returns 200 OK even if the token doesn't exist (per RFC 7009).

### Token Introspection (RFC 7662)

Check token status and metadata:

```bash
curl -X POST http://localhost:8080/oauth/introspect \
  -u "client_id:client_secret" \
  -d "token=ACCESS_TOKEN"
```

Response for active token:
```json
{
  "active": true,
  "client_id": "my-app",
  "scope": "read write",
  "sub": "user123",
  "exp": 1234567890,
  "iat": 1234564290,
  "token_type": "Bearer",
  "iss": "https://api.example.com"
}
```

Response for inactive token:
```json
{
  "active": false
}
```

Clients can only introspect their own tokens.

## OAuth Server Metadata

The plugin exposes OAuth server metadata at `/.well-known/oauth-authorization-server` per RFC 8414:

```bash
curl http://localhost:8080/.well-known/oauth-authorization-server
```

Response includes:
- Endpoint URLs (authorization, token, revocation, introspection)
- Supported grant types and response types
- Supported authentication methods
- Supported PKCE methods

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/oauth/authorize` | GET | Authorization endpoint (user approval) |
| `/oauth/token` | POST | Token endpoint (exchange codes, refresh tokens) |
| `/oauth/revoke` | POST | Revoke access or refresh tokens |
| `/oauth/introspect` | POST | Check token status and metadata |
| `/.well-known/oauth-authorization-server` | GET | OAuth server metadata |

## Storage

### In-Memory Storage (Default)

Clients and tokens are stored in memory. Suitable for development and single-instance deployments where token persistence isn't required.

```go
oauthPlugin := oauth.NewBuilder().
    WithClient(client).
    Build()
```

Tokens are lost on server restart.

### Persistent Storage

Implement `ClientStore` and `TokenStore` interfaces to persist clients and tokens:

```go
type ClientStore interface {
    GetClient(ctx context.Context, clientID string) (*Client, error)
    CreateClient(ctx context.Context, client *Client) error
    UpdateClient(ctx context.Context, client *Client) error
    DeleteClient(ctx context.Context, clientID string) error
    ListClientsByUser(ctx context.Context, userID string) ([]*Client, error)
}

type TokenStore interface {
    Create(ctx context.Context, info TokenInfo) error
    GetByCode(ctx context.Context, code string) (TokenInfo, error)
    GetByAccess(ctx context.Context, access string) (TokenInfo, error)
    GetByRefresh(ctx context.Context, refresh string) (TokenInfo, error)
    RemoveByCode(ctx context.Context, code string) error
    RemoveByAccess(ctx context.Context, access string) error
    RemoveByRefresh(ctx context.Context, refresh string) error
}
```

Configure with custom stores:

```go
oauthPlugin := oauth.NewBuilder().
    WithClientStore(myClientStore).
    WithTokenStore(myTokenStore).
    Build()
```

## Dynamic Client Management

Add clients at runtime:

```go
// Get OAuth plugin from registry
oauthPlugin := registry.Get(oauth.PluginName).(*oauth.OAuthPlugin)

// Add client dynamically
oauthPlugin.AddClient(oauth.Client{
    ID:           "new-client",
    Secret:       "new-secret",
    RedirectURIs: []string{"https://new.com/callback"},
    Scopes:       []string{"read"},
    CreatedBy:    "user123",
})
```

Or use the client store directly:

```go
store := oauthPlugin.GetClientStore()
store.CreateClient(ctx, &oauth.Client{...})
```

## Example

See [examples/oauthserver](../../examples/oauthserver) for a complete working example with:
- Authorization code flow
- Client credentials flow
- Scope-based endpoint protection
- Interactive web interface for testing

Run the example:
```bash
go run ./examples/oauthserver
```

Then visit http://localhost:8080 to test the OAuth flows.

## Security Considerations

- **Client secrets**: Store securely, never commit to version control
- **PKCE**: Enable for public clients with `oauth.enforcePkce` config
- **Redirect URIs**: Whitelist exact URIs, never use wildcards
- **HTTPS**: Use HTTPS in production for all OAuth endpoints
- **Scopes**: Grant minimum necessary scopes for each client
- **Token expiry**: Use short-lived access tokens, longer refresh tokens
