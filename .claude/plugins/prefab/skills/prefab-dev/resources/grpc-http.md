# gRPC Services and HTTP Handlers

## gRPC Service Registration

Register gRPC services with automatic HTTP gateway:

```go
s.RegisterService(
    &pb.MyService_ServiceDesc,      // Service descriptor from generated code
    pb.RegisterMyServiceHandler,     // Gateway handler from generated code
    &myServiceImpl{},                // Your implementation
)
```

## gRPC-Only Services

For services without HTTP gateway:

```go
s := prefab.New(
    prefab.WithGRPCService(&pb.InternalService_ServiceDesc, &internalImpl{}),
)
```

## Custom HTTP Handlers

Add custom HTTP handlers for non-gRPC endpoints:

```go
s := prefab.New(
    prefab.WithHTTPHandler("/health", healthCheckHandler),
    prefab.WithHTTPHandler("/webhook", webhookHandler),
)

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}
```

## Static Files

Serve static files from a directory:

```go
s := prefab.New(
    prefab.WithStaticFiles("/static/", "./static/"),
    prefab.WithStaticFiles("/assets/", "./public/assets/"),
)
```

## Proto HTTP Annotations

Define HTTP routes in your proto files:

```protobuf
import "google/api/annotations.proto";

service MyService {
    rpc GetResource(GetResourceRequest) returns (Resource) {
        option (google.api.http) = {
            get: "/api/v1/resources/{id}"
        };
    }

    rpc CreateResource(CreateResourceRequest) returns (Resource) {
        option (google.api.http) = {
            post: "/api/v1/resources"
            body: "*"
        };
    }

    rpc UpdateResource(UpdateResourceRequest) returns (Resource) {
        option (google.api.http) = {
            put: "/api/v1/resources/{id}"
            body: "*"
        };
    }
}
```

## Interceptors

Add gRPC interceptors:

```go
s := prefab.New(
    prefab.WithGRPCInterceptor(loggingInterceptor),
    prefab.WithGRPCInterceptor(metricsInterceptor),
)

func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    log.Printf("Method: %s", info.FullMethod)
    return handler(ctx, req)
}
```

## HTTP Middleware

Add HTTP middleware:

```go
s := prefab.New(
    prefab.WithHTTPMiddleware(corsMiddleware),
)
```
