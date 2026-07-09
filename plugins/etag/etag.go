// Package etag provides conditional-request (ETag / If-None-Match) support for
// Prefab handlers.
//
// The package is transport agnostic: it works for requests that arrive via the
// GRPC Gateway (using the HTTP If-None-Match request header and a 304 Not
// Modified response) and for native GRPC callers (using an `if-none-match`
// request metadata key and a `prefab-not-modified` response metadata flag). A
// handler written against these helpers behaves identically on both.
//
// The typical, "smart" usage lets a handler skip generating an expensive
// response when the caller already holds a current copy:
//
//	func (s *server) GetDoc(ctx context.Context, req *pb.GetDocRequest) (*pb.Doc, error) {
//	    ver, err := s.store.DocVersion(ctx, req.Id) // cheap: no body materialization
//	    if err != nil {
//	        return nil, err
//	    }
//	    if err := etag.Guard(ctx, etag.Weak(ver)); err != nil {
//	        return nil, err // short-circuits: the expensive load below never runs
//	    }
//	    return s.store.LoadDoc(ctx, req.Id) // the expensive part
//	}
//
// Guard advertises the ETag on the response and, when the caller's validator
// matches, returns ErrNotModified. The plugin's interceptor (see Plugin)
// translates that sentinel into a 304 (Gateway) or a not-modified metadata flag
// (native GRPC). Both the caller opting in (by sending a validator) and the
// handler opting in (by calling Guard) are required, so un-instrumented
// handlers and older callers are unaffected.
package etag

import (
	"context"
	"strings"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/serverutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

const (
	// headerIfNoneMatch is the (lower-cased) conditional request header/metadata
	// key read from the caller.
	headerIfNoneMatch = "if-none-match"

	// headerETag is the response header/metadata key used to advertise the tag.
	headerETag = "etag"

	// MetaNotModified is the response metadata key set on a not-modified result.
	// Native GRPC callers should check for this key (see IsNotModified); the
	// status of the RPC itself remains OK. It is deliberately not a
	// `grpc-metadata-` prefixed key so it is not forwarded as an HTTP response
	// header on the Gateway path (where the 304 status carries the signal
	// instead).
	MetaNotModified = "prefab-not-modified"
)

// ErrNotModified is a sentinel returned by Guard (or that a handler may return
// directly) to signal that the caller already holds a current representation.
//
// It only produces a correct 304 / not-modified response when the etag Plugin
// is registered; the plugin's interceptor swallows this sentinel before it can
// reach the client as an error. Without the plugin it surfaces as an ordinary
// error, so always register the plugin when handlers use Guard.
var ErrNotModified = errors.NewC("etag: resource not modified", codes.Aborted)

// Strong formats value as a strong ETag validator: "value".
//
// value should be derived from a stable domain value — a row version, an
// updated-at timestamp, or a hash you compute over your own inputs. Do NOT
// derive it from a marshaled protobuf: protojson intentionally injects
// randomized whitespace, so the serialized bytes (and any hash of them) differ
// between otherwise-identical responses and would never match.
func Strong(value string) string {
	return `"` + value + `"`
}

// Weak formats value as a weak ETag validator: W/"value".
//
// The same guidance as Strong applies to choosing value: base it on a domain
// value, never on marshaled protobuf bytes. Weak is usually the right choice
// for API responses, whose semantic equivalence does not require byte-for-byte
// identical representations.
func Weak(value string) string {
	return `W/"` + value + `"`
}

// Matches reports whether the caller supplied an If-None-Match validator that
// matches tag. It reads the validator from the HTTP If-None-Match header (for
// Gateway requests) or from `if-none-match` request metadata (for native GRPC
// requests), and follows RFC 7232 weak comparison (the W/ prefix is ignored)
// and the "*" wildcard.
//
// Matches does not modify the response; use Guard for the common case, or use
// Matches directly when a handler needs custom behavior before short-circuiting.
func Matches(ctx context.Context, tag string) bool {
	inm := ifNoneMatch(ctx)
	if inm == "" {
		return false
	}
	if strings.TrimSpace(inm) == "*" {
		return true
	}
	want := normalize(tag)
	for _, candidate := range strings.Split(inm, ",") {
		if normalize(candidate) == want {
			return true
		}
	}
	return false
}

// Guard advertises tag as the response ETag and, if the caller already holds a
// matching validator, returns ErrNotModified so the handler can skip generating
// its response. On a miss it returns nil and the handler should proceed
// normally; the ETag has already been set on the response either way.
func Guard(ctx context.Context, tag string) error {
	if err := SetETag(ctx, tag); err != nil {
		return err
	}
	if Matches(ctx, tag) {
		return ErrNotModified
	}
	return nil
}

// SetETag sets the ETag on the response. For Gateway requests this becomes the
// HTTP `Etag` header; native GRPC callers can read it from the response header
// metadata via ETag.
func SetETag(ctx context.Context, tag string) error {
	return serverutil.SendHeader(ctx, headerETag, tag)
}

// IfNoneMatch returns a copy of ctx with tag set as the `if-none-match`
// outgoing metadata, so a native GRPC client can make a conditional request.
//
//	ctx = etag.IfNoneMatch(ctx, cachedTag)
//	var header metadata.MD
//	resp, err := client.GetDoc(ctx, req, grpc.Header(&header))
//	if etag.IsNotModified(header) { /* use cached copy */ }
func IfNoneMatch(ctx context.Context, tag string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, headerIfNoneMatch, tag)
}

// IsNotModified reports whether response header metadata indicates the resource
// was not modified. Intended for native GRPC clients; Gateway/HTTP clients
// observe a 304 status instead.
func IsNotModified(md metadata.MD) bool {
	return len(md.Get(MetaNotModified)) > 0
}

// ETag returns the ETag advertised in response header metadata, or "" if none.
// Intended for native GRPC clients; Gateway/HTTP clients read the `Etag`
// response header directly.
func ETag(md metadata.MD) string {
	if v := md.Get(headerETag); len(v) > 0 {
		return v[0]
	}
	return ""
}

// ifNoneMatch reads the caller's validator from either transport: the Gateway
// permanent header (via serverutil.HTTPHeader) or a bare `if-none-match`
// request metadata key set by a native GRPC caller.
func ifNoneMatch(ctx context.Context) string {
	if v := serverutil.HTTPHeader(ctx, headerIfNoneMatch); v != "" {
		return v
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get(headerIfNoneMatch); len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// normalize reduces a single validator to its opaque value for weak comparison:
// surrounding whitespace, an optional weak W/ prefix, and the enclosing quotes
// are removed.
func normalize(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "W/")
	return strings.Trim(tag, `"`)
}
