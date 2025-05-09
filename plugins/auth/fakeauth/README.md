# Fake Auth Plugin

The Fake Auth Plugin provides a simple way to create authenticated identities during testing
without the need for real credentials or external dependencies.

## Features

- Create arbitrary identities for testing
- Customize default identity values
- Add validation logic to restrict test identities
- Simple API for test code to obtain authentication tokens
- Works with existing auth plugin infrastructure

## Usage

### Basic Setup

```go
// Create a server with auth and fake auth plugins
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(fake.Plugin()),
)
```

### Customizing the Plugin

```go
// Customize the fake auth plugin
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(fake.Plugin(
        // Set a custom default identity
        fake.WithDefaultIdentity(auth.Identity{
            Provider:      fake.ProviderName,
            Subject:       "test-admin-123",
            Email:         "admin@example.com",
            Name:          "Test Admin",
            EmailVerified: true,
        }),
        // Add validation logic
        fake.WithIdentityValidator(func(ctx context.Context, creds map[string]string) error {
            // Example: only allow certain test emails
            if email, ok := creds["email"]; ok && email != "" {
                if !strings.HasSuffix(email, "@example.com") {
                    return fmt.Errorf("only @example.com emails allowed in tests")
                }
            }
            return nil
        }),
    )),
)
```

### Using in Integration Tests

The plugin provides type-safe helper functions to create authenticated test clients:

```go
func TestUserAPI(t *testing.T) {
    // Set up the test server with fake auth
    s := setupServer()

    // Create a test client for auth service
    ctx := context.Background()
    authClient := auth.NewAuthServiceClient(s.ClientConn())

    emailVerified := true
    token := fake.MustLogin(ctx, authClient, fake.FakeOptions{
        ID:            "test-user-123",
        Email:         "test@example.com",
        Name:          "Test User",
        EmailVerified: &emailVerified,
    })

    // Use the token for authenticated requests
    md := metadata.Pairs("authorization", token)
    authCtx := metadata.NewOutgoingContext(ctx, md)

    // Make authenticated API calls
    userClient := userapi.NewUserServiceClient(s.ClientConn())
    resp, err := userClient.GetUser(authCtx, &userapi.GetUserRequest{...})
    // ...
}
```

### Login Request Credentials

The fake auth plugin supports the following credentials in the login request:

| Key             | Description                           | Default           |
|-----------------|---------------------------------------|-------------------|
| `id`            | Unique user ID                        | "fake-user-123"   |
| `subject`       | Deprecated: Use `id` instead          | -                 |
| `email`         | User email address                    | "fake-user@example.com" |
| `name`          | User display name                     | "Fake User"       |
| `email_verified`| Whether email is verified (true/false)| true              |
| `error_code`    | Simulate error with this code (int)   | -                 |
| `error_message` | Custom error message for simulated errors | "simulated error" |


## Security Considerations

This plugin is intended for **testing only** and should never be used in production
environments. To prevent accidental use in production:

1. Only include this plugin in test builds
2. Use build tags to conditionally include the plugin
3. Add validation that restricts allowed identities in testing

Example of conditional inclusion:

```go
// +build test

package main

import (
    "github.com/dpup/prefab/plugins/auth/fake"
)

func setupTestServer() {
    // Include fake auth plugin for tests
}
```