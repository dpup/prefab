# Security Best Practices

## CSRF Protection

Use proto options to control CSRF protection:

```protobuf
rpc CreateResource(Request) returns (Response) {
  option (csrf_mode) = "on";
  option (google.api.http) = {
    post: "/api/resources"
    body: "*"
  };
}
```

## Authentication Security

- Use strong signing keys (minimum 256 bits)
- Set appropriate token expiration
- Enable token revocation with storage plugin
- Use HTTPS in production
- Rotate signing keys periodically

```yaml
auth:
  signingKey: use-a-strong-random-key-here
  expiration: 24h
```

## Security Headers

Configure security headers in YAML:

```yaml
server:
  security:
    xFrameOptions: DENY
    hstsExpiration: 31536000s
    hstsIncludeSubdomains: true
    corsOrigins:
      - https://app.example.com
```

Or via functional options:

```go
s := prefab.New(
    prefab.WithSecurityHeaders(prefab.SecurityHeaders{
        XFrameOptions:        "DENY",
        HStsExpiration:       31536000 * time.Second,
        HStsIncludeSubdomains: true,
    }),
)
```

## CORS Configuration

```yaml
server:
  security:
    corsOrigins:
      - https://app.example.com
      - https://admin.example.com
    corsMethods:
      - GET
      - POST
      - PUT
      - DELETE
    corsHeaders:
      - Authorization
      - Content-Type
```

## Input Validation

Always validate input at system boundaries:

```go
func (s *Server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    // Validate required fields
    if req.Email == "" {
        return nil, errors.NewC("email is required", codes.InvalidArgument)
    }

    // Validate format
    if !isValidEmail(req.Email) {
        return nil, errors.NewC("invalid email format", codes.InvalidArgument)
    }

    // Validate length limits
    if len(req.Name) > 255 {
        return nil, errors.NewC("name too long", codes.InvalidArgument)
    }

    // Process request...
}
```

## Secrets Management

- Never commit secrets to version control
- Use environment variables for secrets:

```bash
export PF__AUTH__SIGNING_KEY=your-secret-key
export PF__AUTH__GOOGLE__SECRET=your-client-secret
```

- Use secret management services in production (Vault, AWS Secrets Manager)

## Rate Limiting

Implement rate limiting for sensitive endpoints:

```go
// Use a rate limiting middleware or interceptor
s := prefab.New(
    prefab.WithGRPCInterceptor(rateLimitInterceptor),
)
```

## Logging Security

- Don't log sensitive data (passwords, tokens, PII)
- Use structured logging for audit trails
- Redact sensitive fields in error logs

```go
// Good
err := errors.New("authentication failed").
    WithLogField("user_id", userID)

// Bad - don't log passwords
err := errors.New("authentication failed").
    WithLogField("password", password)  // NEVER DO THIS
```

## HTTPS in Production

Always use HTTPS in production. Prefab can be deployed behind a reverse proxy (nginx, Caddy, cloud load balancer) that handles TLS termination.

For complete documentation, see [/docs/security.md](/docs/security.md).
