package serverutil

import (
	"context"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"
)

const (
	// GRPC Metadata prefix that is added to allowed headers specified with
	// WithIncomingHeaders.
	MetadataHeaderPrefix = "pf-header-"

	// GRPC Metadata prefix that is added to metadata keys that are extracted from
	// the HTTP request. These keys will only be present for Gateway requests.
	MetadataHTTPPrefix = "pf-http-"
)

// HTTPHeader returns the value of a "permanent HTTP header" or a header that
// was added to the allow-list by a HeaderMatcher.
//
// For permanent headers, see https://github.com/grpc-ecosystem/grpc-gateway/blob/main/runtime/context.go#L328
//
// This will only ever return a value for requests coming via the GRPC Gateway.
func HTTPHeader(ctx context.Context, header string) string {
	header = strings.ToLower(header)
	md, _ := metadata.FromIncomingContext(ctx)
	if v := md.Get(MetadataHeaderPrefix + header); len(v) > 0 {
		return v[0]
	}
	if v := md.Get(runtime.MetadataPrefix + header); len(v) > 0 {
		return v[0]
	}
	return ""
}

// HTTPMethod returns the HTTP method of the request that was made to the
// Gateway. This will only ever return a value for requests coming via the GRPC
// Gateway.
func HTTPMethod(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	v := md.Get(MetadataHTTPPrefix + "method")
	if len(v) == 1 {
		return v[0]
	}
	return ""
}

// HeaderMatcher appends the given headers to the allow-list for incoming
// requests. This is used to allow certain headers to be passed through the
// Gateway and into the GRPC server.
//
// See: runtime.WithIncomingHeaderMatcher.
func HeaderMatcher(headers []string) func(string) (string, bool) {
	headerMap := map[string]bool{}
	for _, h := range headers {
		headerMap[textproto.CanonicalMIMEHeaderKey(h)] = true
	}
	return func(key string) (string, bool) {
		key = textproto.CanonicalMIMEHeaderKey(key)
		if headerMap[key] {
			return MetadataHeaderPrefix + key, true
		}
		return runtime.DefaultHeaderMatcher(key)
	}
}

// HttpMetadataAnnotator is a gateway option that maps certain HTTP request
// fields to incoming GRPC metadata.
func HttpMetadataAnnotator(_ context.Context, r *http.Request) metadata.MD {
	md := map[string]string{}
	md[MetadataHTTPPrefix+"method"] = r.Method
	return metadata.New(md)
}
