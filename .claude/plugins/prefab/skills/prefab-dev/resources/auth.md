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
