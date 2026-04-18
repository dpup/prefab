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

The OAuth plugin requires the auth plugin to authenticate users during the authorization flow. Run the full working demo at [examples/oauthserver](../../examples/oauthserver) to see every flow end-to-end, including a consent page with CSRF-protected approval.

The snippet above is the bare minimum for local development. See the [Integration Checklist](#integration-checklist) below for what to configure before taking this to production.

## Integration Checklist

Before going to production, make sure you've done the following:

- [ ] **Set `oauth.issuer`** to your public HTTPS URL (e.g., `https://api.example.com`). Without this, metadata falls back to request-derived URLs, which can be poisoned by a spoofed `Host` header and may advertise `http://` behind a TLS-terminating proxy.
- [ ] **Register every redirect URI exactly** — no wildcards. URIs containing control characters or missing a scheme are rejected at registration (`WithClient` will panic).
- [ ] **Enable `oauth.enforcePkce`** if you have any public clients. This rejects the `plain` PKCE method (which provides no protection) and requires `S256`.
- [ ] **Use a persistent `TokenStore`** (see [Storage](#storage)). The default in-memory store loses all tokens on restart and doesn't scale past a single instance.
- [ ] **Decide how consent works.** The default treats any authenticated user's request as approval. If you register third-party clients, supply a `WithUserAuthorizationHandler` that interposes an explicit consent step (see [Consent](#consent)).
- [ ] **Store client secrets securely.** Confidential clients must have a non-empty secret — `WithClient` panics if `Public: false` and `Secret` is empty.
- [ ] **Require `state` on all authorization requests** from your clients as a CSRF defense (OAuth 2.0 §10.12).

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
   curl -X POST http://localhost:8000/oauth/token \
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

When enforcement is on, **only the `S256` method is accepted**. The `plain` method sets `code_challenge == code_verifier` and provides no protection against an attacker who can observe the authorization request — it's explicitly rejected. Requests without `code_challenge_method` are also rejected (the underlying library would otherwise default them to `plain`).

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
   curl -X POST http://localhost:8000/oauth/token \
     -d "grant_type=authorization_code" \
     -d "code=AUTH_CODE" \
     -d "client_id=my-app" \
     -d "code_verifier=VERIFIER"
   ```

### Client Credentials Flow

For server-to-server authentication without user involvement.

```bash
curl -X POST http://localhost:8000/oauth/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=secret-key" \
  -d "scope=read"
```

### Refresh Tokens

Exchange a refresh token for a new access token. **The client must authenticate** — the refresh token alone is not sufficient credential. Confidential clients send `client_secret`; public clients are authenticated by `client_id` only.

```bash
curl -X POST http://localhost:8000/oauth/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=REFRESH_TOKEN" \
  -d "client_id=my-app" \
  -d "client_secret=secret-key"
```

The refreshed token's scope is capped at the original grant's scope — clients cannot escalate scope via refresh. Omitting `scope` retains the original scope; passing a subset is allowed.

Refresh tokens rotate on use (the old refresh token is invalidated and a new one is issued). A `refresh_token` in the response replaces any previous one; revoking either the access or refresh token invalidates both.

## Configuration

### Builder Options

```go
oauth.NewBuilder().
    WithClient(client).                             // Add OAuth client
    WithAccessTokenExpiry(time.Hour).               // Default: 1 hour
    WithRefreshTokenExpiry(7 * 24 * time.Hour).     // Default: 14 days
    WithAuthCodeExpiry(10 * time.Minute).           // Default: 10 minutes
    WithIssuer("https://api.example.com").          // Token issuer URL
    WithEnforcePKCE(true).                          // Require PKCE for public clients
    WithClientStore(customStore).                   // Custom client storage
    WithTokenStore(customStore).                    // Custom token storage
    WithUserAuthorizationHandler(consentHandler).   // Custom consent/approval logic
    Build()
```

### Config Keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `oauth.enforcePkce` | bool | `false` | Require PKCE for public clients |
| `oauth.issuer` | string | `address` config | Token issuer URL |

## Client Types

`WithClient` validates each registered client and **panics at startup** if the configuration is invalid. This surfaces bootstrap mistakes immediately rather than at the first request. The validation rules are:

- `ID` must be non-empty.
- Confidential clients (`Public: false`) must have a non-empty `Secret`.
- Public clients (`Public: true`) must not have a `Secret`.
- Each `RedirectURIs` entry must be an absolute URL with a scheme and must not contain control characters (`\r`, `\n`, `\t`, `\0`). Newline-containing URIs are rejected specifically to prevent smuggling extra callbacks past the allow-list.

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

## Consent

The `/oauth/authorize` endpoint does not render a consent UI. By default, any authenticated user's request is treated as an approval — safe only when all registered clients are first-party (you trust every client equally, e.g., your own apps and internal services).

For multi-tenant or third-party setups, supply a custom `UserAuthorizationHandler` that enforces an explicit consent step. The handler can redirect the browser to your consent page, verify a signed approval token on return, and then resolve the user's subject:

```go
oauth.NewBuilder().
    WithUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (string, error) {
        identity, err := auth.IdentityFromContext(r.Context())
        if err != nil {
            return "", err
        }

        // Check for a valid consent token (double-submit cookie pattern).
        submitted := r.FormValue("consent")
        cookie, cookieErr := r.Cookie("oauth-consent-csrf")
        if submitted != "" && cookieErr == nil && submitted == cookie.Value {
            if err := prefab.VerifyCSRFToken(submitted, signingKey); err == nil {
                return identity.Subject, nil
            }
        }

        // No valid approval — redirect to the consent page with the
        // original authorize params preserved.
        http.Redirect(w, r, "/consent?"+r.URL.RawQuery, http.StatusFound)
        return "", nil
    }).
    Build()
```

The consent page mints a CSRF token via `prefab.GenerateCSRFToken`, sets it as a cookie, and embeds it as a hidden form field. On approval, the form POSTs back to a handler that replays the authorize request with the consent token attached. See [examples/oauthserver](../../examples/oauthserver) for a full working implementation.

## Authentication and Scope-Based Authorization

### How the server picks an identity

When a request arrives, the auth plugin walks a chain of identity extractors and uses the first one that produces an identity:

1. **`Authorization: Bearer <opaque-token>`** — resolved by the OAuth plugin. If the token is valid, the request is authenticated as the OAuth subject and the scopes are exposed via `oauth.HasScope`, `oauth.OAuthScopesFromContext`, etc. **If the bearer is unknown or expired, the request is rejected with 401 — the server does not fall back to cookie authentication.** This prevents a revoked OAuth token from silently being treated as unauthenticated.
2. **`Authorization: Bearer <jwt>`** — resolved by the auth plugin's JWT header extractor.
3. **`Cookie: pf-id=<jwt>`** — resolved by the auth plugin's cookie extractor.

Net: a request with both a cookie and a Bearer is authenticated by the Bearer. Only requests with no Bearer fall back to the cookie.

### Checking scopes

```go
func protectedHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Verify the request is authenticated (bearer or cookie).
    identity, err := auth.IdentityFromContext(ctx)
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // If the request is using OAuth, enforce the required scope.
    if oauth.IsOAuthRequest(ctx) {
        if !oauth.HasScope(ctx, "read") {
            http.Error(w, "Missing 'read' scope", http.StatusForbidden)
            return
        }

        // OAuth metadata is also available:
        _ = oauth.OAuthClientIDFromContext(ctx)
        _ = oauth.OAuthScopesFromContext(ctx)
    }

    // Handle request using identity.Subject.
}
```

Scope helper functions:

```go
oauth.HasScope(ctx, "read")              // Check single scope
oauth.HasAnyScope(ctx, "read", "write")  // Check any of multiple scopes
oauth.HasAllScopes(ctx, "read", "write") // Check all scopes present
oauth.IsOAuthRequest(ctx)                // Check if OAuth token was used
```

Scopes are space-separated strings, per RFC 6749 §3.3.

## Token Management

### Token Revocation (RFC 7009)

Revoke an access or refresh token:

```bash
curl -X POST http://localhost:8000/oauth/revoke \
  -u "client_id:client_secret" \
  -d "token=ACCESS_TOKEN" \
  -d "token_type_hint=access_token"
```

Clients can only revoke their own tokens. The endpoint returns 200 OK even if the token doesn't exist (per RFC 7009).

### Token Introspection (RFC 7662)

Check token status and metadata:

```bash
curl -X POST http://localhost:8000/oauth/introspect \
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
curl http://localhost:8000/.well-known/oauth-authorization-server
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

## Error Responses

OAuth errors are returned as JSON following RFC 6749 §5.2:

```json
{
  "error": "invalid_client",
  "error_description": "Client authentication failed"
}
```

| Error code | When |
|------------|------|
| `invalid_request` | Malformed request, missing required parameter, unsupported grant type |
| `invalid_client` | Unknown client, wrong secret, public client misconfigured with a secret |
| `invalid_grant` | Bad/expired authorization code, invalid refresh token, PKCE verifier mismatch |
| `invalid_scope` | Requested scope not permitted for the client; refresh tried to escalate scope |
| `access_denied` | User denied consent, or redirect URI not in the client's allow list |
| `unauthorized_client` | Client not allowed to use this grant type |

For the authorization endpoint, errors are delivered as a redirect to the client's `redirect_uri` with `error=` and `state=` query parameters (when the redirect URI is valid; otherwise the response is a plain 400).

## Storage

### In-Memory Storage (Default — dev only)

Clients and tokens are stored in memory. This is the default if you don't supply a `ClientStore` or `TokenStore`. Suitable for development, tests, and single-instance deployments where token persistence isn't required.

```go
oauthPlugin := oauth.NewBuilder().
    WithClient(client).
    Build()
```

Caveats — none of these are appropriate for production:

- All tokens are lost on restart. Any user holding an access token at restart time must reauthorize.
- No horizontal scaling: each server instance has its own independent token store, so clients may authenticate on one instance and get rejected on another.
- Expired entries are swept on each `Create` to bound memory use, but there is no persistent rate limiting, audit trail, or replication.

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
if err := oauthPlugin.AddClient(oauth.Client{
    ID:           "new-client",
    Secret:       "new-secret",
    RedirectURIs: []string{"https://new.com/callback"},
    Scopes:       []string{"read"},
    CreatedBy:    "user123",
}); err != nil {
    return err
}
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

Then visit http://localhost:8000 to test the OAuth flows.

## Security Considerations

- **Client secrets**: Store securely, never commit to version control. Confidential clients must set a non-empty secret; public clients must not.
- **PKCE**: Enable `oauth.enforcePkce` for public clients. Only the S256 method is accepted when enforcement is on.
- **Redirect URIs**: Whitelist exact URIs, never use wildcards. Control characters and relative URLs are rejected at registration.
- **HTTPS**: Use HTTPS in production for all OAuth endpoints. Set `oauth.issuer` explicitly to a stable https URL so metadata doesn't depend on request headers.
- **Scopes**: Grant minimum necessary scopes for each client. Scope allowlists are enforced on all grant types; refresh tokens cannot escalate scope beyond the original grant.
- **Consent**: The plugin does not render a consent UI. When integrating the `/oauth/authorize` endpoint with third-party clients, interpose your own approval step so an authenticated user's session cannot be used to issue tokens to an attacker-registered client without explicit approval.
- **Token expiry**: Use short-lived access tokens and longer refresh tokens. Revoking either side of a grant invalidates both.
