// Package main demonstrates how to use Server-Sent Events (SSE) with Prefab and gRPC streaming.
//
// This example shows how to:
// 1. Define a gRPC streaming service
// 2. Register SSE endpoints that stream from gRPC methods
// 3. Handle client disconnections gracefully
//
// The SSE adapter automatically:
// - Creates a gRPC client connection
// - Calls your streaming method
// - Reads from the stream and converts to SSE
// - Handles context cancellation and errors
//
// Run the server:
//
//	go run examples/ssestream/main.go
//
// Test with curl:
//
//	curl -N http://localhost:8080/notes/123/updates
//
// Or with an HTML client (see client.html in this directory).
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/dpup/prefab"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// NOTE: In a real application, you would:
// 1. Define your service in a .proto file (see proto/examples/notesstream/notesstream.proto)
// 2. Generate the Go code with `make gen-proto`
// 3. Implement the service interface
// 4. Register it with prefab.WithGRPCService()
//
// For this example, we'll create a mock streaming service to demonstrate the pattern.

// mockNotesStreamClient is a mock gRPC client stream for demonstration.
// In a real app, this would be the generated NotesStreamService_StreamUpdatesClient type.
type mockNotesStreamClient struct {
	ctx       context.Context
	noteID    string
	count     int
	maxCount  int
	ticker    *time.Ticker
	grpc.ClientStream
}

func (m *mockNotesStreamClient) Recv() (*wrapperspb.StringValue, error) {
	if m.count >= m.maxCount {
		return nil, io.EOF
	}

	select {
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	case <-m.ticker.C:
		m.count++
		msg := fmt.Sprintf(`{"noteId": "%s", "update": %d, "timestamp": "%s", "message": "Note updated"}`,
			m.noteID, m.count, time.Now().Format(time.RFC3339))
		return wrapperspb.String(msg), nil
	}
}

// mockNotesServiceClient simulates a generated gRPC client.
// In a real app, this would be NewNotesStreamServiceClient(cc).
type mockNotesServiceClient struct {
	cc grpc.ClientConnInterface
}

func (c *mockNotesServiceClient) StreamUpdates(ctx context.Context, noteID string) (*mockNotesStreamClient, error) {
	// In a real app, this would call the gRPC method:
	// return c.cc.Invoke(ctx, "/NotesStreamService/StreamUpdates", req, ...)
	return &mockNotesStreamClient{
		ctx:      ctx,
		noteID:   noteID,
		maxCount: 10,
		ticker:   time.NewTicker(1 * time.Second),
	}, nil
}

// This demonstrates the ACTUAL usage pattern you would use in your application:
//
// prefab.WithSSEStream(
//     "/notes/{id}/updates",
//     func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (NotesStreamService_StreamUpdatesClient, error) {
//         client := NewNotesStreamServiceClient(cc)
//         return client.StreamUpdates(ctx, &StreamRequest{NoteId: params["id"]})
//     },
// )

func main() {
	// Create a server with SSE endpoints
	server := prefab.New(
		prefab.WithPort(8080),

		// Example 1: Stream note updates
		// In a real app with generated code, you would write:
		//
		// prefab.WithSSEStream(
		//     "/notes/{id}/updates",
		//     func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (NotesStreamService_StreamUpdatesClient, error) {
		//         client := NewNotesStreamServiceClient(cc)
		//         return client.StreamUpdates(ctx, &StreamRequest{NoteId: params["id"]})
		//     },
		// ),
		//
		// For this demo, we use a mock:
		prefab.WithSSEStream(
			"/notes/{id}/updates",
			func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (*mockNotesStreamClient, error) {
				noteID := params["id"]
				client := &mockNotesServiceClient{cc: cc}
				return client.StreamUpdates(ctx, noteID)
			},
		),

		// Example 2: Multiple path parameters
		prefab.WithSSEStream(
			"/users/{userId}/notes/{noteId}/live",
			func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (*mockNotesStreamClient, error) {
				noteID := fmt.Sprintf("user-%s-note-%s", params["userId"], params["noteId"])
				client := &mockNotesServiceClient{cc: cc}
				stream, err := client.StreamUpdates(ctx, noteID)
				if err != nil {
					return nil, err
				}
				stream.maxCount = 6 // Shorter stream for this demo
				return stream, nil
			},
		),

		// Example 3: Simple counter stream
		prefab.WithSSEStream(
			"/counter",
			func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (*mockCounterStream, error) {
				return &mockCounterStream{
					ctx:    ctx,
					ticker: time.NewTicker(500 * time.Millisecond),
				}, nil
			},
		),
	)

	log.Println("Starting SSE example server on :8080")
	log.Println("")
	log.Println("The SSE adapter automatically handles:")
	log.Println("  ✓ Creating gRPC client connections")
	log.Println("  ✓ Calling streaming methods")
	log.Println("  ✓ Reading from streams")
	log.Println("  ✓ Converting protobuf to JSON")
	log.Println("  ✓ Formatting as SSE events")
	log.Println("  ✓ Context cancellation when clients disconnect")
	log.Println("  ✓ Error handling and cleanup")
	log.Println("")
	log.Println("Try these endpoints:")
	log.Println("  curl -N http://localhost:8080/notes/123/updates")
	log.Println("  curl -N http://localhost:8080/users/42/notes/99/live")
	log.Println("  curl -N http://localhost:8080/counter")
	log.Println("")
	log.Println("Or open: examples/ssestream/client.html in a browser")

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// mockCounterStream demonstrates a simple counter stream.
type mockCounterStream struct {
	ctx    context.Context
	count  int
	ticker *time.Ticker
	grpc.ClientStream
}

func (m *mockCounterStream) Recv() (*wrapperspb.StringValue, error) {
	if m.count >= 20 {
		return nil, io.EOF
	}

	select {
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	case <-m.ticker.C:
		m.count++
		now := timestamppb.Now()
		msg := fmt.Sprintf(`{"count": %d, "timestamp": "%s"}`, m.count, now.AsTime().Format(time.RFC3339))
		return wrapperspb.String(msg), nil
	}
}

// Real-world usage example (commented out since it requires generated code):
//
// func setupRealSSE() prefab.ServerOption {
//     return prefab.WithSSEStream(
//         "/notes/{id}/updates",
//         func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (notesstream.NotesStreamService_StreamUpdatesClient, error) {
//             // Create the generated client
//             client := notesstream.NewNotesStreamServiceClient(cc)
//
//             // Build the request from path parameters
//             req := &notesstream.StreamRequest{
//                 NoteId: params["id"],
//             }
//
//             // Optionally add query parameters
//             if since := params["query.since"]; since != "" {
//                 // Parse and add to request
//                 req.Since = parseSince(since)
//             }
//
//             // Call the streaming method
//             return client.StreamUpdates(ctx, req)
//         },
//     )
// }
