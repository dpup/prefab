// Package main demonstrates how to use Server-Sent Events (SSE) with Prefab and gRPC streaming.
//
// Run the server:
//
//	go run examples/ssestream/main.go
//
// Test with curl:
//
//	curl -N http://localhost:8080/counter
//
// Or open client.html in a browser.
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

// mockCounterStream is a mock gRPC client stream for demonstration.
// In a real app, this would be the generated CounterService_StreamClient type.
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

func main() {
	server := prefab.New(
		prefab.WithPort(8080),

		// Register SSE endpoint - Prefab handles everything
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
	log.Println("Try: curl -N http://localhost:8080/counter")

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
