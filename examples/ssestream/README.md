# Server-Sent Events (SSE) Example

This example demonstrates how to use Server-Sent Events with Prefab to stream data from gRPC services to web clients.

## Overview

Prefab's SSE support bridges gRPC streaming and web clients by automatically handling connection management, stream reading, and event formatting. You provide a function that calls your gRPC streaming method, and Prefab handles the rest.

## Running the Example

Start the server:

```bash
go run examples/ssestream/main.go
```

Test with curl:

```bash
curl -N http://localhost:8080/counter
```

Or open `client.html` in a browser.

## Usage

```go
server := prefab.New(
    // Register your gRPC streaming service
    prefab.WithGRPCService(&CounterService_ServiceDesc, counterService),
    
    // Register SSE endpoint - Prefab handles everything
    prefab.WithSSEStream(
        "/counter",
        func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (CounterService_StreamClient, error) {
            client := NewCounterServiceClient(cc)
            return client.Stream(ctx, &CounterRequest{})
        },
    ),
)
```

That's it! Prefab automatically:
- Creates and reuses gRPC client connections
- Reads from the stream
- Converts protobuf to JSON
- Formats as SSE events
- Handles context cancellation and cleanup

## Client Usage

### JavaScript

```javascript
const eventSource = new EventSource('http://localhost:8080/counter');
eventSource.onmessage = (event) => {
    console.log('Received:', JSON.parse(event.data));
};
```

### curl

```bash
curl -N http://localhost:8080/counter
```

## Features

- Path parameters: `/notes/{id}/updates`
- Query parameters: `params["query.paramName"]`
- Type-safe with Go generics
- Automatic cleanup on client disconnect
- Single shared connection for all SSE endpoints

See `docs/reference.md` for complete documentation.
