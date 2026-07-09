// Command etag demonstrates the etag plugin for conditional-request handling.
//
// The Echo handler treats its `ping` argument as a tiny "document" whose ETag
// is a hash of the content, and uses etag.Guard to short-circuit response
// generation when the caller already holds a current copy.
//
// Try it with curl:
//
//	# First request: 200 OK with an Etag header. Note the "cache miss" log line.
//	curl -i 'http://0.0.0.0:8000/api/echo?ping=hello'
//
//	# Repeat with the returned validator: 304 Not Modified, empty body, and NO
//	# "cache miss" log line — the handler skipped generation.
//	curl -i -H 'If-None-Match: W/"<etag-value>"' 'http://0.0.0.0:8000/api/echo?ping=hello'
//
//	# A different ping produces a different Etag, so the same If-None-Match misses.
//	curl -i -H 'If-None-Match: W/"<etag-value>"' 'http://0.0.0.0:8000/api/echo?ping=world'
package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/examples/simpleserver/simpleservice"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/etag"
)

func main() {
	s := prefab.New(
		prefab.WithPlugin(etag.Plugin()),
	)

	s.RegisterService(
		&simpleservice.SimpleService_ServiceDesc,
		simpleservice.RegisterSimpleServiceHandler,
		&etagServer{},
	)

	fmt.Println("")
	fmt.Println("Fetch a document (200 + Etag):")
	fmt.Println("  curl -i 'http://0.0.0.0:8000/api/echo?ping=hello'")
	fmt.Println("")
	fmt.Println("Re-fetch with the returned validator (304, no body, no 'cache miss' log):")
	fmt.Println(`  curl -i -H 'If-None-Match: W/"<etag>"' 'http://0.0.0.0:8000/api/echo?ping=hello'`)
	fmt.Println("")

	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

// etagServer implements simpleservice.SimpleServiceServer with ETag support on
// Echo.
type etagServer struct {
	simpleservice.UnimplementedSimpleServiceServer
}

// Health is a plain, unconditional endpoint.
func (s *etagServer) Health(ctx context.Context, in *simpleservice.HealthRequest) (*simpleservice.HealthResponse, error) {
	return &simpleservice.HealthResponse{Status: "OK"}, nil
}

// Echo returns the ping value, but first advertises an ETag derived from the
// content and short-circuits with a 304 if the caller already has it.
func (s *etagServer) Echo(ctx context.Context, in *simpleservice.EchoRequest) (*simpleservice.EchoResponse, error) {
	// A real handler would derive this from a version/updated-at without loading
	// the full resource. Here the ping value *is* the resource, so we hash it.
	if err := etag.Guard(ctx, etag.Weak(version(in.Ping))); err != nil {
		return nil, err // 304: the generation below is skipped entirely.
	}

	logging.Infow(ctx, "cache miss: generating response", "ping", in.Ping)
	return &simpleservice.EchoResponse{Pong: in.Ping}, nil
}

// version returns a short, stable content hash used as the ETag value.
func version(content string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(content))
	return strconv.FormatUint(uint64(h.Sum32()), 16)
}
