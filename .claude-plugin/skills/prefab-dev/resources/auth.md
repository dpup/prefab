# Authentication

Prefab provides multiple authentication plugins for different use cases.

## Google OAuth

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/auth"
    "github.com/dpup/prefab/plugins/auth/google"
)

s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(google.Plugin()),
)
```

Configure via YAML:
```yaml
auth:
  signingKey: your-secret-key
  expiration: 24h
  google:
    id: your-google-client-id
    secret: your-google-client-secret
```

## Password Authentication

```go
import (
    "github.com/dpup/prefab/plugins/auth/pwdauth"
)

s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(pwdauth.Plugin(
        pwdauth.WithAccountFinder(myAccountStore),
        pwdauth.WithHasher(myPasswordHasher),
    )),
)
```

## Magic Link Authentication

Requires email and templates plugins:

```go
import (
    "github.com/dpup/prefab/plugins/auth/magiclink"
    "github.com/dpup/prefab/plugins/email"
    "github.com/dpup/prefab/plugins/templates"
)

s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(email.Plugin()),
    prefab.WithPlugin(templates.Plugin()),
    prefab.WithPlugin(magiclink.Plugin()),
)
```

## Fake Authentication (Testing)

```go
import (
    "github.com/dpup/prefab/plugins/auth/fakeauth"
)

s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(fakeauth.Plugin(
        fakeauth.WithDefaultIdentity(auth.Identity{
            Subject: "test-user-123",
            Email:   "test@example.com",
            Name:    "Test User",
        }),
        fakeauth.WithIdentityValidator(validateTestIdentity),
    )),
)
```

## API Key Authentication

For programmatic access to your API:

```go
import (
    "github.com/dpup/prefab/plugins/auth/apikey"
)

s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(apikey.Plugin(
        apikey.WithKeyFunc(func(ctx context.Context, key string) (*apikey.KeyOwner, error) {
            // Look up key in your database
            owner, err := db.GetAPIKeyOwner(ctx, key)
            if err != nil {
                return nil, err
            }
            return &apikey.KeyOwner{
                UserID:        owner.UserID,
                Email:         owner.Email,
                EmailVerified: true,
                Name:          owner.Name,
                KeyCreatedAt:  owner.CreatedAt,
            }, nil
        }),
        apikey.WithKeyPrefix("myapp"), // Keys will be "myapp_xxx..."
    )),
)
```

### Generating API Keys

```go
// In your user management service
func (s *Server) CreateAPIKey(ctx context.Context, req *pb.CreateAPIKeyRequest) (*pb.APIKey, error) {
    apiKeyPlugin := s.registry.Get("auth_apikey").(*apikey.APIPlugin)

    key := apiKeyPlugin.NewKey() // Generates "myapp_<random>"

    // Store key hash in database (never store plain key)
    if err := s.db.StoreAPIKey(ctx, req.UserId, hashKey(key)); err != nil {
        return nil, err
    }

    // Return plain key to user (only time it's visible)
    return &pb.APIKey{Key: key}, nil
}
```

### Client Usage

```bash
curl -H "Authorization: myapp_abc123def456..." https://api.example.com/endpoint
```

## Accessing Identity in Handlers

```go
func (s *Server) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.Profile, error) {
    identity, ok := auth.IdentityFromContext(ctx)
    if !ok {
        return nil, errors.NewC("unauthorized", codes.Unauthenticated)
    }

    return &pb.Profile{
        Id:    identity.Subject,
        Email: identity.Email,
        Name:  identity.Name,
    }, nil
}
```

## Authentication Configuration

```yaml
auth:
  signingKey: your-jwt-signing-key  # Required for JWT tokens
  expiration: 24h                    # Token expiration

  google:
    id: google-client-id
    secret: google-client-secret
```

Environment variables:
```bash
export PF__AUTH__SIGNING_KEY=your-secret-key
export PF__AUTH__GOOGLE__ID=your-client-id
export PF__AUTH__GOOGLE__SECRET=your-client-secret
```
