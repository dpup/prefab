# Prefab Quickstart Guide

This guide walks you through creating a simple gRPC server with a REST/JSON Gateway using Prefab.

## Installation

First, add the Prefab library to your Go project:

```bash
go get github.com/dpup/prefab
```

## Step 1: Define your Service

Create a proto file for your service (e.g., `proto/helloservice/helloservice.proto`):

```protobuf
syntax = "proto3";
package helloservice;
option go_package = "./helloservice";

import "google/api/annotations.proto";

service HelloService {
  rpc SayHello(HelloRequest) returns (HelloResponse) {
    option (google.api.http) = {
      get: "/api/hello/{name}"
    };
  }
}

message HelloRequest {
  string name = 1;
}

message HelloResponse {
  string greeting = 1;
}
```

## Step 2: Generate gRPC Code

Install the required protoc plugins and generate the Go code:

```bash
# Install protoc plugins (if not already installed)
go install google.golang.org/protobuf/cmd/protoc-gen-go
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway

# Generate code
protoc -I./proto \
  -I./proto/third_party/googleapis \
  --go_out=. \
  --go-grpc_out=. \
  --grpc-gateway_out=. \
  ./proto/helloservice/helloservice.proto
```

## Step 3: Implement the Service

Create an implementation of your service:

```go
// helloservice/service.go
package helloservice

import (
    "context"
    "fmt"
)

type Server struct {
    UnimplementedHelloServiceServer
}

func NewServer() *Server {
    return &Server{}
}

func (s *Server) SayHello(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
    return &HelloResponse{
        Greeting: fmt.Sprintf("Hello, %s!", req.Name),
    }, nil
}
```

## Step 4: Create the Prefab Server

Now create your main server file:

```go
// main.go
package main

import (
    "fmt"
    "log"
    
    "github.com/dpup/prefab"
    "yourmodule/helloservice"
)

func main() {
    // Create a new Prefab server
    server := prefab.New(
        prefab.WithPort(8080),
    )

    // Register the service and gateway
    server.RegisterService(
        &helloservice.HelloService_ServiceDesc,
        helloservice.RegisterHelloServiceHandler,
        helloservice.NewServer(),
    )

    // Start the server
    fmt.Println("Server starting on :8080")
    if err := server.Start(); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}
```

## Step 5: Run the Server

Build and run your server:

```bash
go build
./yourapp
```

## Step 6: Test the API

Test your API using curl:

```bash
# Using gRPC Gateway (REST)
curl http://localhost:8080/api/hello/world

# Using gRPC (requires a gRPC client)
# Example using grpcurl:
grpcurl -plaintext -d '{"name": "world"}' localhost:8080 helloservice.HelloService/SayHello
```

## Next Steps

Now that you have a basic server running, you can:

1. Add authentication with `auth.Plugin()`
2. Implement authorization with `authz.Plugin()`
3. Add persistent storage with `storage.Plugin()`
4. Serve static files with `WithStaticFiles()`
5. Add custom HTTP handlers with `WithHTTPHandler()`

Check the documentation for detailed information on these features.