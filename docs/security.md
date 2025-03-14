# Security in Prefab

Prefab includes several security features to help you build secure web applications.

## CSRF Protection

Prefab provides built-in CSRF protection that can be controlled at the route level using proto options:

```proto
rpc CreateResource(Request) returns (Response) {
  option (csrf_mode) = "on";  // CSRF protection required
  option (google.api.http) = {
    post: "/api/resources"
    body: "*"
  };
}
```

CSRF mode values:
- `"on"`: CSRF protection is always required
- `"off"`: No CSRF protection required
- `"auto"` (default): CSRF protection required for non-safe methods (POST, PUT, DELETE, etc.)

### CSRF Implementation

Prefab implements CSRF protection in two ways:

1. **X-CSRF-Protection header**: For XHR/fetch requests, set the header:
   ```javascript
   fetch("/api/resources", {
     method: "POST",
     headers: {
       "Content-Type": "application/json",
       "X-CSRF-Protection": 1,
     },
     credentials: "include",
     body: JSON.stringify(data),
   });
   ```

2. **Double-submit cookie**: For form submissions and full-page navigation, use:
   ```html
   <form action="/api/resources" method="post">
     <input type="hidden" name="csrf-token" value="${token}">
     <!-- form fields -->
     <button type="submit">Submit</button>
   </form>
   ```

   The token can be requested from `/api/meta/config` or found in the `pf-ct` cookie.

## Security Headers

Prefab can be configured to set security headers:

```yaml
server:
  security:
    # Prevent iframe embedding
    xFrameOptions: DENY
    
    # HTTP Strict Transport Security
    hstsExpiration: 31536000s
    hstsIncludeSubdomains: true
    hstsPreload: true
    
    # Cross-Origin Resource Sharing
    corsOrigins:
      - https://app.example.com
    corsAllowedMethods:
      - GET
      - POST
    corsAllowedHeaders:
      - x-csrf-protection
    corsAllowCredentials: true
    corsMaxAge: 72h
```

You can also configure these headers programmatically:

```go
s := prefab.New(
    prefab.WithSecurityHeaders(prefab.SecurityHeaders{
        XFrameOptions:       "DENY",
        HStsExpiration:      31536000 * time.Second,
        HStsIncludeSubdomains: true,
        HStsPreload:         true,
        CORSOrigins:         []string{"https://app.example.com"},
        // other options...
    }),
)
```

## Authentication Security

When using authentication plugins, follow these security practices:

1. **Use strong signing keys**:
   ```yaml
   auth:
     signingKey: your-strong-secret-key
   ```

2. **Set appropriate token expiration**:
   ```yaml
   auth:
     expiration: 1h  # Short-lived tokens for sensitive operations
   ```

3. **Enable token revocation** with a storage plugin:
   ```go
   s := prefab.New(
       prefab.WithPlugin(storage.Plugin(store)),
       prefab.WithPlugin(auth.Plugin()),
   )
   ```

4. **Use HTTPS in production** to protect tokens and cookies.