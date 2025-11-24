# Server-Sent Events (SSE)

Bridge gRPC streaming services to web clients using Server-Sent Events. Prefab automatically handles connection management, stream reading, and event formatting.

## Basic Usage

```go
s := prefab.New(
    prefab.WithGRPCService(&CounterService_ServiceDesc, counterService),

    prefab.WithSSEStream(
        "/counter",
        func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (CounterService_StreamClient, error) {
            client := NewCounterServiceClient(cc)
            return client.Stream(ctx, &CounterRequest{})
        },
    ),
)
```

Prefab automatically handles:
- Stream reading
- Protobuf-to-JSON conversion
- SSE formatting
- Cleanup when clients disconnect

## Path Parameters

```go
prefab.WithSSEStream(
    "/notes/{id}/updates",
    func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (NotesStreamService_StreamUpdatesClient, error) {
        client := NewNotesStreamServiceClient(cc)
        return client.StreamUpdates(ctx, &StreamRequest{NoteId: params["id"]})
    },
)
```

## Query Parameters

Access query parameters as `params["query.paramName"]`:

```go
prefab.WithSSEStream(
    "/notes/{id}/updates",
    func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (NotesStreamService_StreamUpdatesClient, error) {
        req := &StreamRequest{NoteId: params["id"]}
        if since := params["query.since"]; since != "" {
            req.Since = parseTimestamp(since)
        }
        client := NewNotesStreamServiceClient(cc)
        return client.StreamUpdates(ctx, req)
    },
)
```

## Client Usage

### JavaScript

```javascript
const eventSource = new EventSource('http://localhost:8080/counter');

eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};

eventSource.onerror = (error) => {
    console.error('SSE error:', error);
    eventSource.close();
};
```

### curl

```bash
curl -N http://localhost:8080/counter
```

### With Path Parameters

```javascript
const noteId = '12345';
const eventSource = new EventSource(`http://localhost:8080/notes/${noteId}/updates`);
```

### With Query Parameters

```javascript
const eventSource = new EventSource('http://localhost:8080/notes/123/updates?since=2024-01-01');
```

## Complete Example

Server:

```go
// Define streaming service
type counterService struct{}

func (s *counterService) Stream(req *CounterRequest, stream CounterService_StreamServer) error {
    count := 0
    for {
        select {
        case <-stream.Context().Done():
            return nil
        case <-time.After(time.Second):
            count++
            if err := stream.Send(&CounterResponse{Count: int32(count)}); err != nil {
                return err
            }
        }
    }
}

// Setup server
s := prefab.New(
    prefab.WithPort(8080),
    prefab.WithGRPCService(&CounterService_ServiceDesc, &counterService{}),
    prefab.WithSSEStream("/counter", func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (CounterService_StreamClient, error) {
        return NewCounterServiceClient(cc).Stream(ctx, &CounterRequest{})
    }),
)
```

See `examples/ssestream/` for a complete working example.
