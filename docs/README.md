# Prefab Documentation

Welcome to the Prefab documentation! Prefab is a Go library that simplifies creating gRPC servers with a JSON/REST
Gateway. This documentation will help AI coding assistants effectively use Prefab in your projects.

## Claude Code Plugin

Prefab includes a Claude Code plugin at `.claude/plugins/prefab/` with comprehensive skills and resources
for building Prefab servers. The plugin provides topic-specific documentation that is dynamically loaded
based on your task, covering server setup, authentication, authorization, SSE streaming, configuration,
storage, and more.

## Contents

- [Quickstart Guide](quickstart.md) - Get a basic server running in minutes
- [Getting Started](getting-started.md) - Core concepts and basic usage
- [Configuration](configuration.md) - How to configure Prefab servers
- [Plugins](plugins.md) - Using and creating plugins
- [Security](security.md) - CSRF protection, security headers, and best practices
- [Authorization](authz.md) - Declarative access control
- [Logging](logging.md) - Structured context-aware logging

## Key Features

- **Simplified Server Setup**: Create production-ready gRPC servers with minimal boilerplate
- **Multiplex gRPC and HTTP**: Serve both gRPC and HTTP/REST APIs on the same port
- **Pluggable Architecture**: Extend functionality with modular plugins
- **Authentication**: Google OAuth, Magic Links, Password-based, and API Key authentication
- **Authorization**: Role-based access control for API endpoints
- **Storage**: Simple CRUD operations with different storage backends
- **Configuration**: Configure via YAML, environment variables, or code
- **Security**: Built-in CSRF protection and security headers

## Common Usage Pattern

```go
package main

import (
    "fmt"
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/auth"
    "yourpackage/yourservice"
)

func main() {
    // Create server with desired plugins and options
    s := prefab.New(
        prefab.WithPlugin(auth.Plugin()),
        prefab.WithStaticFiles("/", "./static/"),
    )

    // Register your service
    s.RegisterService(
        &yourservice.YourService_ServiceDesc,
        yourservice.RegisterYourServiceHandler,
        &serviceImpl{},
    )

    // Start the server
    if err := s.Start(); err != nil {
        fmt.Println(err)
    }
}
```

## What You'll Need

1. Go 1.16 or later
2. Protocol Buffers compiler (protoc)
3. gRPC Gateway protoc plugins

## Additional Resources

- See the [examples](../examples) directory for working code samples
- Read the [README.md](../README.md) for an overview of Prefab
