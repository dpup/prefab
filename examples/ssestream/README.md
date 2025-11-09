# Server-Sent Events (SSE) Example

This example demonstrates how to use Server-Sent Events with Prefab to stream data from gRPC services to web clients.

## Overview

Server-Sent Events (SSE) provide a way to push real-time updates from a server to web clients over HTTP. Prefab's SSE support bridges the gap between gRPC streaming and web clients by automatically:

- Creating gRPC client connections
- Calling streaming methods
- Reading from streams and converting to SSE
- Handling context cancellation and errors
- Managing all cleanup

You only need to provide a function that calls your gRPC streaming method. Prefab does the rest.

## Architecture

```
┌─────────────┐      HTTP/SSE      ┌──────────────┐      gRPC Stream     ┌──────────────┐
│   Browser   │ ←─────────────────→ │ SSE Adapter  │ ←──────────────────→ │  gRPC Service│
│  (EventSrc) │                     │ (HTTP Handler)│                      │              │
└─────────────┘                     └──────────────┘                      └──────────────┘
```

The SSE adapter:
- Registers as an HTTP handler (bypassing grpc-gateway)
- Extracts path parameters from the URL
- Creates a gRPC client and calls your streaming method
- Reads from the stream automatically
- Converts protobuf messages to JSON
- Sends them as SSE events
- Handles all cancellation and cleanup

## Running the Example

1. Start the server:

```bash
go run examples/ssestream/main.go
```

2. Test with curl:

```bash
# Stream note updates
curl -N http://localhost:8080/notes/123/updates

# Stream counter
curl -N http://localhost:8080/counter

# Stream live edits
curl -N http://localhost:8080/users/42/notes/99/live
```

3. Or open the HTML client:

```bash
# Open examples/ssestream/client.html in a browser
open examples/ssestream/client.html
```

## Usage

### Step 1: Define Your gRPC Streaming Service

```protobuf
service NotesStreamService {
  rpc StreamUpdates(StreamRequest) returns (stream NoteUpdate) {
    option (google.api.http) = {
      get: "/api/notes/{note_id}/stream"
    };
  }
}
```

### Step 2: Implement the Service

```go
func (s *NotesService) StreamUpdates(req *StreamRequest, stream NotesStreamService_StreamUpdatesServer) error {
    for update := range s.getUpdates(req.NoteId) {
        if err := stream.Send(update); err != nil {
            return err
        }
    }
    return nil
}
```

### Step 3: Register with Prefab + SSE

```go
server := prefab.New(
    // Register the gRPC service
    prefab.WithGRPCService(&NotesStreamService_ServiceDesc, notesService),
    prefab.WithGRPCGateway(RegisterNotesStreamServiceHandlerFromEndpoint),

    // Register SSE endpoint - Prefab handles everything!
    prefab.WithSSEStream(
        "/notes/{id}/updates",
        func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (NotesStreamService_StreamUpdatesClient, error) {
            client := NewNotesStreamServiceClient(cc)
            return client.StreamUpdates(ctx, &StreamRequest{NoteId: params["id"]})
        },
    ),
)
```

That's it! Three simple steps:
1. Define your streaming service in proto
2. Implement it
3. Call `WithSSEStream()` with your streaming method

## What Prefab Handles Automatically

- ✅ gRPC client connection creation
- ✅ Stream reading loop (calls `Recv()` automatically)
- ✅ Protobuf to JSON conversion
- ✅ SSE event formatting
- ✅ Context cancellation when client disconnects
- ✅ Error handling and cleanup
- ✅ EOF detection
- ✅ HTTP response flushing

## Features

### Path Parameters

Extract parameters from URL patterns:

```go
prefab.WithSSEStream(
    "/notes/{id}/updates",
    func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (...) {
        noteID := params["id"]  // ← Extracted from URL
        // ...
    },
)
```

### Multiple Parameters

```go
prefab.WithSSEStream(
    "/users/{userId}/notes/{noteId}/live",
    func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (...) {
        userID := params["userId"]
        noteID := params["noteId"]
        // ...
    },
)
```

### Query Parameters

```go
// Access via params["query.paramName"]
if since := params["query.since"]; since != "" {
    req.Since = parseTimestamp(since)
}

// Client usage: /notes/123/updates?since=2025-01-01T00:00:00Z
```

### Type Safety

Uses Go generics to ensure type safety:

```go
func WithSSEStream[T proto.Message](
    path string,
    starter func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (ClientStream[T], error),
) ServerOption
```

The compiler ensures you return the correct stream type.

## Client Usage

### JavaScript (Browser)

```javascript
const eventSource = new EventSource('http://localhost:8080/notes/123/updates');

eventSource.onopen = () => {
    console.log('Connected');
};

eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};

eventSource.onerror = (error) => {
    console.error('Error:', error);
    eventSource.close();
};
```

### curl

```bash
curl -N http://localhost:8080/notes/123/updates
```

## Comparison with grpc-gateway

| Feature | grpc-gateway | SSE Adapter |
|---------|--------------|-------------|
| Streaming | ❌ No SSE support | ✅ Full SSE support |
| Browser Support | ⚠️ Requires fetch API | ✅ Native EventSource API |
| Reconnection | ❌ Manual | ✅ Automatic |
| Path Parameters | ✅ Yes | ✅ Yes |
| Custom Logic | ❌ Limited | ✅ Full control |
| Setup Complexity | High (annotations) | Low (1 function) |

## Best Practices

1. **Authentication**: Use Prefab's auth middleware to protect endpoints
2. **Rate Limiting**: Consider rate limiting to prevent abuse
3. **Error Handling**: Let errors bubble up - Prefab handles them
4. **Monitoring**: Add metrics to track active connections

## Limitations

- SSE is unidirectional (server → client only)
- Not suitable for large binary payloads
- Browsers have connection limits (typically 6 per domain)
- Doesn't work through some corporate proxies

For bidirectional communication, consider WebSockets instead.

## Next Steps

- Add authentication to SSE endpoints
- Implement reconnection with event IDs
- Add metrics and monitoring
- Create a pub/sub backend for broadcasting
