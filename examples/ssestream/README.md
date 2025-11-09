# Server-Sent Events (SSE) Example

This example demonstrates how to use Server-Sent Events with Prefab to stream data from gRPC services to web clients.

## Overview

Server-Sent Events (SSE) provide a way to push real-time updates from a server to web clients over HTTP. This is useful for:

- Live notifications
- Real-time dashboards
- Collaborative editing
- Activity feeds
- Progress updates

Prefab's SSE support bridges the gap between gRPC streaming and web clients by providing an HTTP handler that:
1. Accepts HTTP requests
2. Calls gRPC streaming methods
3. Converts the stream to SSE format
4. Handles client disconnections gracefully

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
- Calls the appropriate gRPC streaming method
- Converts protobuf messages to JSON
- Sends them as SSE events

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
# Serve the HTML file (you can use any static server)
python3 -m http.server 8000

# Then open http://localhost:8000/examples/ssestream/client.html
```

## Usage

### Basic SSE Endpoint

```go
server := prefab.New(
    prefab.WithSSE(prefab.SSEConfig{
        Path: "/events",
        StreamFunc: func(ctx context.Context, params map[string]string, ch chan<- proto.Message) error {
            // Send events to the channel
            ch <- wrapperspb.String("Hello from SSE!")
            return nil
        },
    }),
)
```

### With Path Parameters

```go
prefab.WithSSE(prefab.SSEConfig{
    Path: "/notes/{id}/updates",
    StreamFunc: func(ctx context.Context, params map[string]string, ch chan<- proto.Message) error {
        noteID := params["id"]

        // Stream updates for this note
        for update := range getUpdates(ctx, noteID) {
            ch <- update
        }
        return nil
    },
})
```

### Integrating with gRPC Streaming

Here's how to adapt a gRPC streaming service to SSE:

```go
// Your gRPC streaming service
func (s *NotesService) StreamUpdates(req *StreamRequest, stream NotesStreamService_StreamUpdatesServer) error {
    for update := range s.getUpdates(req.NoteId) {
        if err := stream.Send(update); err != nil {
            return err
        }
    }
    return nil
}

// SSE adapter
prefab.WithSSE(prefab.SSEConfig{
    Path: "/notes/{id}/updates",
    StreamFunc: func(ctx context.Context, params map[string]string, ch chan<- proto.Message) error {
        noteID := params["id"]

        // Call your gRPC service (as a client)
        client := NewNotesStreamServiceClient(conn)
        stream, err := client.StreamUpdates(ctx, &StreamRequest{NoteId: noteID})
        if err != nil {
            return err
        }

        // Forward messages from gRPC stream to SSE channel
        for {
            update, err := stream.Recv()
            if err == io.EOF {
                return nil
            }
            if err != nil {
                return err
            }
            ch <- update
        }
    },
})
```

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

## Features

- **Path Parameters**: Extract parameters from URL patterns like `/notes/{id}/updates`
- **Query Parameters**: Access via `params["query.paramName"]`
- **Context Cancellation**: Automatically stops streaming when client disconnects
- **Error Handling**: Gracefully handles errors and closes connections
- **Protobuf Support**: Automatically converts protobuf messages to JSON
- **Middleware Integration**: Works with existing Prefab middleware (auth, CORS, etc.)

## Testing

Run the SSE tests:

```bash
go test -v -run TestSSE ./...
```

## Comparison with grpc-gateway

| Feature | grpc-gateway | SSE Adapter |
|---------|--------------|-------------|
| Streaming | ❌ No SSE support | ✅ Full SSE support |
| Browser Support | ⚠️ Requires fetch API | ✅ Native EventSource API |
| Reconnection | ❌ Manual | ✅ Automatic |
| Path Parameters | ✅ Yes | ✅ Yes |
| Custom Logic | ❌ Limited | ✅ Full control |

## Best Practices

1. **Handle Context Cancellation**: Always check `ctx.Done()` in your stream function
2. **Close Channels**: Always close the message channel when done
3. **Error Handling**: Return errors from StreamFunc to properly close connections
4. **Rate Limiting**: Consider rate limiting to prevent abuse
5. **Authentication**: Use Prefab's auth middleware to protect endpoints
6. **Buffering**: Use buffered channels to prevent blocking

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
