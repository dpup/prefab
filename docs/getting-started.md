# Getting Started with Prefab

Prefab is a Go library that streamlines creating gRPC servers with a JSON/REST
Gateway. It provides sensible defaults to get your server running with minimal
boilerplate, while offering configuration via environment variables, config
files, or programmatic options.

## Basic Server Setup

To create a minimal Prefab server:

```go
package main

import (
    "fmt"
    "github.com/dpup/prefab"
    "yourpackage/yourservice"
)

func main() {
    // Create a new server with default options
    s := prefab.New()

    // Register your gRPC service and gateway
    s.RegisterService(
        &yourservice.YourService_ServiceDesc,
        yourservice.RegisterYourServiceHandler,
        &yourServiceImpl{},
    )

    // Start the server
    if err := s.Start(); err != nil {
        fmt.Println(err)
    }
}
```

## Core Concepts

### Server Initialization with Options

You can customize the server with options:

```go
s := prefab.New(
    prefab.WithHTTPHandler("/", http.HandlerFunc(homeHandler)),
    prefab.WithStaticFiles("/static/", "./static/"),
    // Other options as needed
)
```

### Registering Services

For gRPC services defined in your proto files, use the `RegisterService` method:

```go
// With gateway
s.RegisterService(
    &yourservice.YourService_ServiceDesc,
    yourservice.RegisterYourServiceHandler,
    &yourServiceImpl{},
)

// Without gateway (pass nil for the registerGateway parameter)
s.RegisterService(
    &yourservice.YourService_ServiceDesc,
    nil,
    &yourServiceImpl{},
)
```

This single call replaces the traditional two-step pattern and handles both the gRPC service registration and the optional HTTP/JSON gateway registration.

### Starting the Server

```go
if err := s.Start(); err != nil {
    // Handle error
}
```

The `Start()` method blocks until the server is shut down.
