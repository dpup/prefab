# Server Setup and Initialization

## Basic Server Creation

```go
import (
    "github.com/dpup/prefab"
    // Import plugins as needed
)

func main() {
    // Create server with options
    s := prefab.New(
        prefab.WithPort(8080),
        prefab.WithHTTPHandler("/custom", myHandler),
        prefab.WithStaticFiles("/static/", "./static/"),
        // Add plugins as needed
    )

    // Register service and gateway
    s.RegisterService(
        &yourservice.YourService_ServiceDesc,
        yourservice.RegisterYourServiceHandler,
        &yourServiceImpl{},
    )

    // Start the server - this blocks until shutdown
    if err := s.Start(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Server Options

Common server options:

- `prefab.WithPort(port)` - Set the server port
- `prefab.WithHTTPHandler(path, handler)` - Add custom HTTP handler
- `prefab.WithStaticFiles(prefix, dir)` - Serve static files
- `prefab.WithPlugin(plugin)` - Add a plugin
- `prefab.WithGRPCService(desc, impl)` - Register gRPC service without HTTP gateway

## Service Registration

Register gRPC services with automatic HTTP gateway:

```go
s.RegisterService(
    &pb.MyService_ServiceDesc,      // gRPC service descriptor
    pb.RegisterMyServiceHandler,     // gRPC-gateway registration
    &myServiceImpl{},                // Implementation
)
```

## Graceful Shutdown

The server handles shutdown signals automatically. `s.Start()` blocks until:
- SIGINT or SIGTERM is received
- An unrecoverable error occurs

## Multiple Services

```go
s := prefab.New(prefab.WithPort(8080))

// Register multiple services
s.RegisterService(&pb.UserService_ServiceDesc, pb.RegisterUserServiceHandler, &userImpl{})
s.RegisterService(&pb.OrderService_ServiceDesc, pb.RegisterOrderServiceHandler, &orderImpl{})

if err := s.Start(); err != nil {
    log.Fatal(err)
}
```
