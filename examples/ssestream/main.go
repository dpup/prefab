// Package main demonstrates how to use Server-Sent Events (SSE) with Prefab.
//
// This example shows how to:
// 1. Create an SSE endpoint with path parameters
// 2. Stream protobuf messages to clients
// 3. Handle client disconnections gracefully
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
	"log"
	"time"

	"github.com/dpup/prefab"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func main() {
	// Create a server with SSE endpoints
	server := prefab.New(
		prefab.WithPort(8080),
		prefab.WithGRPCReflection(),

		// Example 1: Simple SSE stream with path parameters
		prefab.WithSSE(prefab.SSEConfig{
			Path: "/notes/{id}/updates",
			StreamFunc: streamNoteUpdates,
		}),

		// Example 2: SSE stream with multiple path parameters
		prefab.WithSSE(prefab.SSEConfig{
			Path: "/users/{userId}/notes/{noteId}/live",
			StreamFunc: streamLiveNoteEdits,
		}),

		// Example 3: Simple counter stream
		prefab.WithSSE(prefab.SSEConfig{
			Path: "/counter",
			StreamFunc: streamCounter,
		}),
	)

	log.Println("Starting SSE example server on :8080")
	log.Println("Try: curl -N http://localhost:8080/notes/123/updates")
	log.Println("Try: curl -N http://localhost:8080/users/42/notes/99/live")
	log.Println("Try: curl -N http://localhost:8080/counter")

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// streamNoteUpdates simulates streaming updates for a note.
// In a real application, this would connect to a gRPC streaming service
// or subscribe to a message queue.
func streamNoteUpdates(ctx context.Context, params map[string]string, ch chan<- proto.Message) error {
	noteID := params["id"]

	log.Printf("Client connected to note %s updates", noteID)
	defer log.Printf("Client disconnected from note %s updates", noteID)

	// Simulate periodic updates
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	updateCount := 0
	for {
		select {
		case <-ctx.Done():
			// Client disconnected or context cancelled
			return ctx.Err()

		case <-ticker.C:
			updateCount++

			// Send an update message
			// In a real app, this would be a proper protobuf message
			message := fmt.Sprintf("Update #%d for note %s at %s",
				updateCount, noteID, time.Now().Format(time.RFC3339))

			ch <- wrapperspb.String(message)

			// Stop after 10 updates for demo purposes
			if updateCount >= 10 {
				log.Printf("Completed streaming 10 updates for note %s", noteID)
				return nil
			}
		}
	}
}

// streamLiveNoteEdits demonstrates SSE with multiple path parameters.
// This simulates live collaborative editing updates.
func streamLiveNoteEdits(ctx context.Context, params map[string]string, ch chan<- proto.Message) error {
	userID := params["userId"]
	noteID := params["noteId"]

	log.Printf("Streaming live edits for user %s, note %s", userID, noteID)
	defer log.Printf("Stopped streaming for user %s, note %s", userID, noteID)

	// Simulate live editing events
	events := []string{
		"User started editing",
		"Typed: 'Hello'",
		"Typed: ' World'",
		"Added formatting",
		"Saved draft",
		"User stopped editing",
	}

	for i, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Create a structured message
			message := fmt.Sprintf(`{"userId": "%s", "noteId": "%s", "event": "%s", "sequence": %d}`,
				userID, noteID, event, i+1)

			ch <- wrapperspb.String(message)

			// Wait a bit between events
			time.Sleep(1 * time.Second)
		}
	}

	return nil
}

// streamCounter demonstrates a simple counter stream.
func streamCounter(ctx context.Context, params map[string]string, ch chan<- proto.Message) error {
	log.Println("Starting counter stream")
	defer log.Println("Counter stream ended")

	for i := 1; i <= 20; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Send a timestamped counter value
			now := timestamppb.Now()
			message := fmt.Sprintf(`{"count": %d, "timestamp": "%s"}`, i, now.AsTime().Format(time.RFC3339))
			ch <- wrapperspb.String(message)

			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}
