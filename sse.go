package prefab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ClientStream represents a gRPC client stream that can receive messages.
// This interface is satisfied by all generated gRPC client stream types.
type ClientStream[T proto.Message] interface {
	Recv() (T, error)
	grpc.ClientStream
}

// SSEStreamStarter is a function that starts a gRPC client stream.
// It receives the request context, path/query parameters, and a gRPC client connection.
// It should create a client and call the streaming method, returning the stream.
//
// Example:
//
//	func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (NotesStreamService_StreamUpdatesClient, error) {
//	    client := NewNotesStreamServiceClient(cc)
//	    return client.StreamUpdates(ctx, &StreamRequest{NoteId: params["id"]})
//	}
type SSEStreamStarter[T proto.Message] func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (ClientStream[T], error)

// pathPattern represents a parsed path pattern with parameter extraction.
type pathPattern struct {
	pattern *regexp.Regexp
	params  []string
	prefix  string
}

// parsePathPattern converts a path pattern like "/notes/{id}/updates" into a regex
// that can match requests and extract parameters.
func parsePathPattern(pattern string) (*pathPattern, error) {
	if pattern == "" {
		return nil, errors.NewC("sse: path pattern cannot be empty", codes.InvalidArgument)
	}

	// Extract parameter names and build regex
	var params []string
	var regexPattern strings.Builder
	regexPattern.WriteString("^")

	// Find the prefix (everything before the first parameter)
	prefix := pattern
	if idx := strings.Index(pattern, "{"); idx != -1 {
		prefix = pattern[:idx]
	}

	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}

		if i > 0 {
			regexPattern.WriteString("/")
		}

		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// Extract parameter name
			paramName := part[1 : len(part)-1]
			if paramName == "" {
				return nil, errors.NewC("sse: empty parameter name in pattern", codes.InvalidArgument)
			}
			params = append(params, paramName)
			// Match any non-slash characters
			regexPattern.WriteString("([^/]+)")
		} else {
			// Literal path component
			regexPattern.WriteString(regexp.QuoteMeta(part))
		}
	}
	regexPattern.WriteString("$")

	re, err := regexp.Compile(regexPattern.String())
	if err != nil {
		return nil, errors.WrapPrefix(err, "sse: invalid path pattern", 0)
	}

	return &pathPattern{
		pattern: re,
		params:  params,
		prefix:  prefix,
	}, nil
}

// extractParams extracts parameter values from a request path.
func (p *pathPattern) extractParams(path string) (map[string]string, bool) {
	matches := p.pattern.FindStringSubmatch(path)
	if matches == nil {
		return nil, false
	}

	params := make(map[string]string)
	for i, name := range p.params {
		params[name] = matches[i+1]
	}
	return params, true
}

// createSSEHandler creates an HTTP handler that serves Server-Sent Events from a gRPC stream.
func createSSEHandler[T proto.Message](pattern *pathPattern, starter SSEStreamStarter[T], s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := logging.EnsureLogger(r.Context())

		// Only allow GET requests
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract path parameters
		params, ok := pattern.extractParams(r.URL.Path)
		if !ok {
			logging.Errorw(ctx, "sse: path does not match pattern",
				"path", r.URL.Path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Add query parameters to params map
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				params["query."+key] = values[0]
			}
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

		// Check if the ResponseWriter supports flushing
		flusher, ok := w.(http.Flusher)
		if !ok {
			logging.Error(ctx, "sse: streaming not supported")
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Create a context that will be cancelled when the client disconnects
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Use the shared gRPC client connection
		cc := s.sseClientConn

		// Start the gRPC stream
		stream, err := starter(ctx, params, cc)
		if err != nil {
			logging.Errorw(ctx, "sse: failed to start stream", "error", err)
			http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
			return
		}

		// Marshal options for JSON conversion
		marshaler := protojson.MarshalOptions{
			EmitUnpopulated: true,
			UseProtoNames:   false,
		}

		logging.Infow(ctx, "sse: client connected", "path", r.URL.Path, "params", params)

		// Stream messages to the client
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				// Stream completed normally
				logging.Infow(ctx, "sse: stream completed", "path", r.URL.Path)
				return
			}
			if err != nil {
				logging.Errorw(ctx, "sse: stream error", "error", err)
				// Send error as SSE comment (not visible to EventSource API but visible in raw stream)
				fmt.Fprintf(w, ": error: %s\n\n", err.Error())
				flusher.Flush()
				return
			}

			// Convert proto message to JSON
			data, err := marshaler.Marshal(msg)
			if err != nil {
				logging.Errorw(ctx, "sse: failed to marshal message", "error", err)
				continue
			}

			// Write SSE event
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				logging.Errorw(ctx, "sse: failed to write event", "error", err)
				return
			}

			// Flush the data immediately
			flusher.Flush()
		}
	})
}

// WithSSEStream registers a Server-Sent Events endpoint that streams from a gRPC streaming method.
//
// The path can include parameters in curly braces, e.g., "/notes/{id}/updates".
// These parameters will be extracted and passed to the stream starter function.
//
// The starter function receives:
//   - ctx: Request context (cancelled when client disconnects)
//   - params: Map of path and query parameters
//   - cc: gRPC client connection (connected to this server)
//
// The starter function should create a gRPC client and call the streaming method.
//
// Example:
//
//	server := prefab.New(
//	    prefab.WithSSEStream(
//	        "/notes/{id}/updates",
//	        func(ctx context.Context, params map[string]string, cc grpc.ClientConnInterface) (NotesStreamService_StreamUpdatesClient, error) {
//	            client := NewNotesStreamServiceClient(cc)
//	            return client.StreamUpdates(ctx, &StreamRequest{NoteId: params["id"]})
//	        },
//	    ),
//	)
//
// All stream management (reading, cancellation, error handling, SSE formatting) is handled automatically.
//
// Multiple SSE endpoints share a single gRPC client connection for efficiency.
func WithSSEStream[T proto.Message](path string, starter SSEStreamStarter[T]) ServerOption {
	return func(b *builder) {
		pattern, err := parsePathPattern(path)
		if err != nil {
			panic(err)
		}

		// Capture the server reference to access the shared connection
		var server *Server

		// Register a server builder that:
		// 1. Creates the shared SSE client connection if not already created
		// 2. Stores the server reference for handlers
		b.serverBuilders = append(b.serverBuilders, func(s *Server) {
			server = s

			// Create the shared SSE client connection if this is the first SSE endpoint
			if s.sseClientConn == nil {
				_, _, endpoint, opts := s.GatewayArgs()
				conn, err := grpc.NewClient(endpoint, opts...)
				if err != nil {
					panic(fmt.Sprintf("sse: failed to create shared client connection: %v", err))
				}
				s.sseClientConn = conn
				logging.Infow(s.baseContext, "sse: created shared gRPC client connection", "endpoint", endpoint)
			}
		})

		// Register the HTTP handler
		b.handlers = append(b.handlers, handler{
			prefix: pattern.prefix,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Use the server's shared connection
				h := createSSEHandler(pattern, starter, server)
				h.ServeHTTP(w, r)
			}),
		})
	}
}
