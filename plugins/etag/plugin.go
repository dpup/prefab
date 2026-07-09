package etag

import (
	"context"
	"net/http"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/serverutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// PluginName is the name of the etag plugin.
const PluginName = "etag"

// Plugin returns a Prefab plugin that enables conditional-request handling for
// handlers that use Guard / Matches. It registers a GRPC interceptor which
// turns the ErrNotModified sentinel into a 304 Not Modified response (Gateway)
// or a not-modified metadata flag (native GRPC).
//
//	s := prefab.New(prefab.WithPlugin(etag.Plugin()))
func Plugin() *EtagPlugin {
	return &EtagPlugin{}
}

// EtagPlugin implements the Prefab plugin interface for ETag handling.
type EtagPlugin struct{}

// Name implements prefab.Plugin.
func (p *EtagPlugin) Name() string {
	return PluginName
}

// ServerOptions implements prefab.OptionProvider, registering the interceptor.
func (p *EtagPlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithGRPCInterceptor(Interceptor()),
	}
}

// Interceptor returns the unary interceptor that translates ErrNotModified into
// the appropriate transport signals. It is exported so it can be composed
// directly (e.g. in tests) without registering the full plugin.
func Interceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil && errors.Is(err, ErrNotModified) {
			if serr := signalNotModified(ctx); serr != nil {
				return nil, serr
			}
			// The body is intentionally empty. On the Gateway path the 304
			// status means net/http (and the core cleanup middleware) drop it;
			// native GRPC callers key off the not-modified metadata and ignore
			// the body. An empty message unmarshals into any response type.
			return &emptypb.Empty{}, nil
		}
		return resp, err
	}
}

// signalNotModified sets the response signals for a not-modified result on both
// transports: an HTTP 304 status for the Gateway, and a metadata flag for
// native GRPC callers.
func signalNotModified(ctx context.Context) error {
	if err := serverutil.SendStatusCode(ctx, http.StatusNotModified); err != nil {
		return err
	}
	return grpc.SetHeader(ctx, metadata.Pairs(MetaNotModified, "1"))
}
