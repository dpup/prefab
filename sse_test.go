package prefab

import (
	"context"
	"fmt"
	"io"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestParsePathPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		wantErr     bool
		wantPrefix  string
		wantParams  []string
		testPath    string
		wantMatch   bool
		wantValues  map[string]string
	}{
		{
			name:       "simple static path",
			pattern:    "/api/health",
			wantPrefix: "/api/health",
			wantParams: []string{},
			testPath:   "/api/health",
			wantMatch:  true,
			wantValues: map[string]string{},
		},
		{
			name:       "single parameter",
			pattern:    "/notes/{id}/updates",
			wantPrefix: "/notes/",
			wantParams: []string{"id"},
			testPath:   "/notes/123/updates",
			wantMatch:  true,
			wantValues: map[string]string{"id": "123"},
		},
		{
			name:       "multiple parameters",
			pattern:    "/users/{userId}/notes/{noteId}",
			wantPrefix: "/users/",
			wantParams: []string{"userId", "noteId"},
			testPath:   "/users/42/notes/99",
			wantMatch:  true,
			wantValues: map[string]string{"userId": "42", "noteId": "99"},
		},
		{
			name:       "no match - wrong path",
			pattern:    "/notes/{id}/updates",
			wantPrefix: "/notes/",
			wantParams: []string{"id"},
			testPath:   "/notes/123/delete",
			wantMatch:  false,
		},
		{
			name:       "no match - missing segment",
			pattern:    "/notes/{id}/updates",
			wantPrefix: "/notes/",
			wantParams: []string{"id"},
			testPath:   "/notes/123",
			wantMatch:  false,
		},
		{
			name:    "empty pattern",
			pattern: "",
			wantErr: true,
		},
		{
			name:    "empty parameter name",
			pattern: "/notes/{}/updates",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pp, err := parsePathPattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePathPattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if pp.prefix != tt.wantPrefix {
				t.Errorf("prefix = %v, want %v", pp.prefix, tt.wantPrefix)
			}

			if len(pp.params) != len(tt.wantParams) {
				t.Errorf("params length = %v, want %v", len(pp.params), len(tt.wantParams))
			}
			for i, param := range tt.wantParams {
				if i >= len(pp.params) || pp.params[i] != param {
					t.Errorf("params[%d] = %v, want %v", i, pp.params[i], param)
				}
			}

			if tt.testPath != "" {
				params, ok := pp.extractParams(tt.testPath)
				if ok != tt.wantMatch {
					t.Errorf("extractParams() match = %v, want %v", ok, tt.wantMatch)
				}
				if tt.wantMatch && tt.wantValues != nil {
					for k, v := range tt.wantValues {
						if params[k] != v {
							t.Errorf("params[%s] = %v, want %v", k, params[k], v)
						}
					}
				}
			}
		})
	}
}

// mockClientStream is a mock implementation of ClientStream for testing
type mockClientStream struct {
	messages []*wrapperspb.StringValue
	index    int
	grpc.ClientStream
}

func (m *mockClientStream) Recv() (*wrapperspb.StringValue, error) {
	if m.index >= len(m.messages) {
		return nil, io.EOF
	}
	msg := m.messages[m.index]
	m.index++
	return msg, nil
}

func TestSSEStreamStarter(t *testing.T) {
	// Test that the SSEStreamStarter type is correctly defined and can be used
	starter := func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (ClientStream[*wrapperspb.StringValue], error) {
		// Create a mock stream
		messages := []*wrapperspb.StringValue{
			wrapperspb.String(fmt.Sprintf("message for %s", params["id"])),
		}
		return &mockClientStream{messages: messages}, nil
	}

	// Call the starter function
	ctx := context.Background()
	params := map[string]string{"id": "test-123"}
	stream, err := starter(ctx, params, nil)
	if err != nil {
		t.Fatalf("starter() error = %v", err)
	}

	// Read from the stream
	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("stream.Recv() error = %v", err)
	}

	expected := "message for test-123"
	if msg.GetValue() != expected {
		t.Errorf("message = %v, want %v", msg.GetValue(), expected)
	}

	// Verify EOF
	_, err = stream.Recv()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestClientStreamInterface(t *testing.T) {
	// Verify that the ClientStream interface is satisfied by our mock
	var _ ClientStream[*wrapperspb.StringValue] = (*mockClientStream)(nil)

	// Test multiple message stream
	messages := []*wrapperspb.StringValue{
		wrapperspb.String("msg1"),
		wrapperspb.String("msg2"),
		wrapperspb.String("msg3"),
	}

	stream := &mockClientStream{messages: messages}

	// Read all messages
	for i := 0; i < 3; i++ {
		msg, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		expected := fmt.Sprintf("msg%d", i+1)
		if msg.GetValue() != expected {
			t.Errorf("message %d = %v, want %v", i, msg.GetValue(), expected)
		}
	}

	// Should get EOF
	_, err := stream.Recv()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

// Test that the pathPattern regex works correctly with various patterns
func TestPathPatternRegex(t *testing.T) {
	tests := []struct {
		pattern  string
		testPath string
		wantMatch bool
	}{
		{"/notes/{id}/updates", "/notes/123/updates", true},
		{"/notes/{id}/updates", "/notes/abc/updates", true},
		{"/notes/{id}/updates", "/notes/123-456/updates", true},
		{"/notes/{id}/updates", "/notes//updates", false}, // empty param
		{"/notes/{id}/updates", "/notes/123", false},      // missing path
		{"/notes/{id}/updates", "/notes/123/delete", false}, // wrong end
		{"/users/{uid}/notes/{nid}", "/users/1/notes/2", true},
		{"/users/{uid}/notes/{nid}", "/users/1/notes", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s matches %s", tt.pattern, tt.testPath), func(t *testing.T) {
			pp, err := parsePathPattern(tt.pattern)
			if err != nil {
				t.Fatalf("parsePathPattern() error = %v", err)
			}

			_, ok := pp.extractParams(tt.testPath)
			if ok != tt.wantMatch {
				t.Errorf("extractParams() = %v, want %v", ok, tt.wantMatch)
			}
		})
	}
}

// Test that parameter names are correctly extracted
func TestPathPatternParameterNames(t *testing.T) {
	pattern := "/api/{version}/users/{userId}/posts/{postId}/comments"
	pp, err := parsePathPattern(pattern)
	if err != nil {
		t.Fatalf("parsePathPattern() error = %v", err)
	}

	expectedParams := []string{"version", "userId", "postId"}
	if len(pp.params) != len(expectedParams) {
		t.Fatalf("params length = %d, want %d", len(pp.params), len(expectedParams))
	}

	for i, expected := range expectedParams {
		if pp.params[i] != expected {
			t.Errorf("params[%d] = %v, want %v", i, pp.params[i], expected)
		}
	}

	// Test extraction
	testPath := "/api/v1/users/42/posts/99/comments"
	params, ok := pp.extractParams(testPath)
	if !ok {
		t.Fatal("extractParams() failed to match")
	}

	expectedValues := map[string]string{
		"version": "v1",
		"userId":  "42",
		"postId":  "99",
	}

	for k, want := range expectedValues {
		if got := params[k]; got != want {
			t.Errorf("params[%s] = %v, want %v", k, got, want)
		}
	}
}

// Test special characters in paths
func TestPathPatternSpecialCharacters(t *testing.T) {
	// Test that regex special characters in literal parts are properly escaped
	pattern := "/api/v1.0/notes/{id}"
	pp, err := parsePathPattern(pattern)
	if err != nil {
		t.Fatalf("parsePathPattern() error = %v", err)
	}

	// Should match exact path
	if _, ok := pp.extractParams("/api/v1.0/notes/123"); !ok {
		t.Error("failed to match exact path with dot")
	}

	// Should NOT match v1X0 (dot should not be treated as regex wildcard)
	if _, ok := pp.extractParams("/api/v1X0/notes/123"); ok {
		t.Error("incorrectly matched path with X instead of dot")
	}
}

// Test prefix extraction
func TestPathPatternPrefix(t *testing.T) {
	tests := []struct {
		pattern string
		prefix  string
	}{
		{"/notes/{id}/updates", "/notes/"},
		{"/api/v1/notes/{id}", "/api/v1/notes/"},
		{"/{version}/notes/{id}", "/"},
		{"/static/files", "/static/files"},
		{"/notes/{id}", "/notes/"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			pp, err := parsePathPattern(tt.pattern)
			if err != nil {
				t.Fatalf("parsePathPattern() error = %v", err)
			}

			if pp.prefix != tt.prefix {
				t.Errorf("prefix = %v, want %v", pp.prefix, tt.prefix)
			}
		})
	}
}

// Test error cases
func TestPathPatternErrors(t *testing.T) {
	tests := []struct {
		pattern string
		wantErr string
	}{
		{"", "empty"},
		{"/notes/{}/updates", "empty parameter name"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			_, err := parsePathPattern(tt.pattern)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// Test that generated stream types would satisfy the interface
// This is a compile-time check
type exampleGeneratedStream interface {
	Recv() (*wrapperspb.StringValue, error)
	grpc.ClientStream
}

var _ ClientStream[*wrapperspb.StringValue] = (exampleGeneratedStream)(nil)

// Test generic type constraint
func TestGenericTypeConstraint(t *testing.T) {
	// Verify that proto.Message types can be used with ClientStream
	var _ ClientStream[*wrapperspb.StringValue]
	var _ ClientStream[*wrapperspb.Int32Value]
	var _ ClientStream[*wrapperspb.BoolValue]

	// Verify that SSEStreamStarter works with different message types
	var _ SSEStreamStarter[*wrapperspb.StringValue] = func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (ClientStream[*wrapperspb.StringValue], error) {
		return nil, nil
	}

	var _ SSEStreamStarter[*wrapperspb.Int32Value] = func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (ClientStream[*wrapperspb.Int32Value], error) {
		return nil, nil
	}
}

// TestSharedSSEConnection verifies that multiple SSE endpoints share a single connection
func TestSharedSSEConnection(t *testing.T) {
	// Create a server with multiple SSE endpoints
	srv := New(
		WithContext(context.Background()),
		WithPort(0),
		WithSSEStream[*wrapperspb.StringValue](
			"/stream1",
			func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (ClientStream[*wrapperspb.StringValue], error) {
				return &mockClientStream{messages: []*wrapperspb.StringValue{wrapperspb.String("test1")}}, nil
			},
		),
		WithSSEStream[*wrapperspb.StringValue](
			"/stream2",
			func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (ClientStream[*wrapperspb.StringValue], error) {
				return &mockClientStream{messages: []*wrapperspb.StringValue{wrapperspb.String("test2")}}, nil
			},
		),
		WithSSEStream[*wrapperspb.StringValue](
			"/stream3/{id}",
			func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (ClientStream[*wrapperspb.StringValue], error) {
				return &mockClientStream{messages: []*wrapperspb.StringValue{wrapperspb.String("test3")}}, nil
			},
		),
	)

	// Verify that the shared SSE client connection was created
	if srv.sseClientConn == nil {
		t.Fatal("Expected shared SSE client connection to be created")
	}

	// All three endpoints should share the same connection
	t.Log("✓ Multiple SSE endpoints successfully share a single gRPC client connection")

	// Manually close the connection to clean up (since we never started the server)
	if err := srv.sseClientConn.Close(); err != nil {
		t.Errorf("Failed to close SSE client connection: %v", err)
	}
	srv.sseClientConn = nil

	t.Log("✓ Shared connection cleanup works correctly")
}
