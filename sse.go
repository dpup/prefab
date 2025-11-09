package prefab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// SSEStreamFunc is a function that produces a stream of protobuf messages.
// The function should send messages to the provided channel and close it when done.
// The context will be cancelled if the client disconnects.
type SSEStreamFunc func(ctx context.Context, params map[string]string, ch chan<- proto.Message) error

// SSEConfig configures a Server-Sent Events endpoint.
type SSEConfig struct {
	// Path pattern for the SSE endpoint. Can include path parameters like "/notes/{id}/updates"
	Path string

	// StreamFunc is called to generate the stream of messages.
	StreamFunc SSEStreamFunc
}

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

// createSSEHandler creates an HTTP handler that serves Server-Sent Events.
func createSSEHandler(config SSEConfig) (http.Handler, error) {
	pattern, err := parsePathPattern(config.Path)
	if err != nil {
		return nil, err
	}

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
				"path", r.URL.Path,
				"pattern", config.Path)
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

		// Create a channel for messages
		messageCh := make(chan proto.Message, 10)

		// Create a context that will be cancelled when the client disconnects
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Start the stream function in a goroutine
		errCh := make(chan error, 1)
		go func() {
			defer close(messageCh)
			if err := config.StreamFunc(ctx, params, messageCh); err != nil {
				errCh <- err
			}
			close(errCh)
		}()

		// Marshal options for JSON conversion
		marshaler := protojson.MarshalOptions{
			EmitUnpopulated: true,
			UseProtoNames:   false,
		}

		logging.Infow(ctx, "sse: client connected", "path", r.URL.Path, "params", params)

		// Stream messages to the client
		for {
			select {
			case msg, ok := <-messageCh:
				if !ok {
					// Channel closed, stream is complete
					logging.Infow(ctx, "sse: stream completed", "path", r.URL.Path)
					return
				}

				// Convert proto message to JSON
				var data []byte
				var err error
				if msg != nil {
					data, err = marshaler.Marshal(msg)
					if err != nil {
						logging.Errorw(ctx, "sse: failed to marshal message", "error", err)
						continue
					}
				}

				// Write SSE event
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					logging.Errorw(ctx, "sse: failed to write event", "error", err)
					return
				}

				// Flush the data immediately
				flusher.Flush()

			case err := <-errCh:
				if err != nil {
					logging.Errorw(ctx, "sse: stream error", "error", err)
					// Send error as SSE comment (not visible to EventSource API but visible in raw stream)
					fmt.Fprintf(w, ": error: %s\n\n", err.Error())
					flusher.Flush()
				}
				return

			case <-ctx.Done():
				// Client disconnected
				logging.Infow(ctx, "sse: client disconnected", "path", r.URL.Path)
				return
			}
		}
	}), nil
}

// WithSSE registers a Server-Sent Events endpoint.
//
// The path can include parameters in curly braces, e.g., "/notes/{id}/updates".
// These parameters will be extracted and passed to the stream function.
//
// Example:
//
//	server := prefab.New(
//	    prefab.WithSSE(prefab.SSEConfig{
//	        Path: "/notes/{id}/updates",
//	        StreamFunc: func(ctx context.Context, params map[string]string, ch chan<- proto.Message) error {
//	            noteID := params["id"]
//	            // Stream updates for the note...
//	            return nil
//	        },
//	    }),
//	)
func WithSSE(config SSEConfig) ServerOption {
	return func(b *builder) {
		h, err := createSSEHandler(config)
		if err != nil {
			panic(err)
		}

		// Register the handler with the appropriate prefix
		pattern, _ := parsePathPattern(config.Path)
		b.handlers = append(b.handlers, handler{
			prefix:      pattern.prefix,
			httpHandler: h,
		})
	}
}

// SSEEvent represents a typed SSE event that can be sent to clients.
// This is a helper type for applications that want to send structured events.
type SSEEvent struct {
	// Event type (optional, defaults to "message")
	Event string `json:"-"`
	// Data to send
	Data any `json:"data,omitempty"`
}

// MarshalSSE converts an SSEEvent to the SSE wire format.
func (e *SSEEvent) MarshalSSE() ([]byte, error) {
	var buf strings.Builder

	if e.Event != "" {
		buf.WriteString("event: ")
		buf.WriteString(e.Event)
		buf.WriteString("\n")
	}

	if e.Data != nil {
		data, err := json.Marshal(e.Data)
		if err != nil {
			return nil, err
		}
		buf.WriteString("data: ")
		buf.Write(data)
		buf.WriteString("\n")
	}

	buf.WriteString("\n")
	return []byte(buf.String()), nil
}
