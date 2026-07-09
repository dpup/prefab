package etag

import (
	"context"
	"testing"

	"github.com/dpup/prefab/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestStrongAndWeak(t *testing.T) {
	assert.Equal(t, `"v1"`, Strong("v1"))
	assert.Equal(t, `W/"v1"`, Weak("v1"))
}

// gatewayCtx builds a context that looks like a Gateway request carrying the
// given If-None-Match value (grpc-gateway maps permanent headers with the
// `grpcgateway-` prefix).
func gatewayCtx(inm string) context.Context {
	md := metadata.MD{}
	if inm != "" {
		md.Set("grpcgateway-if-none-match", inm)
	}
	return metadata.NewIncomingContext(context.Background(), md)
}

// nativeCtx builds a context that looks like a native GRPC request carrying the
// given If-None-Match value as a bare metadata key.
func nativeCtx(inm string) context.Context {
	md := metadata.MD{}
	if inm != "" {
		md.Set("if-none-match", inm)
	}
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestMatches(t *testing.T) {
	cases := []struct {
		name string
		inm  string
		tag  string
		want bool
	}{
		{"no header", "", Strong("v1"), false},
		{"exact strong", `"v1"`, Strong("v1"), true},
		{"different", `"v1"`, Strong("v2"), false},
		{"wildcard", "*", Strong("v1"), true},
		{"list contains", `"a", "v1", "b"`, Strong("v1"), true},
		{"list missing", `"a", "b"`, Strong("v1"), false},
		{"weak request vs strong tag", `W/"v1"`, Strong("v1"), true},
		{"strong request vs weak tag", `"v1"`, Weak("v1"), true},
		{"weak both", `W/"v1"`, Weak("v1"), true},
	}
	for _, c := range cases {
		t.Run("gateway/"+c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Matches(gatewayCtx(c.inm), c.tag))
		})
		t.Run("native/"+c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Matches(nativeCtx(c.inm), c.tag))
		})
	}
}

func TestGuard(t *testing.T) {
	t.Run("miss advertises etag and returns nil", func(t *testing.T) {
		transport := &mockTransport{}
		ctx := grpc.NewContextWithServerTransportStream(gatewayCtx(`"other"`), transport)

		err := Guard(ctx, Weak("v1"))
		require.NoError(t, err)
		assert.Equal(t, `W/"v1"`, transport.header("grpc-metadata-etag"))
	})

	t.Run("hit advertises etag and returns ErrNotModified", func(t *testing.T) {
		transport := &mockTransport{}
		ctx := grpc.NewContextWithServerTransportStream(gatewayCtx(`W/"v1"`), transport)

		err := Guard(ctx, Weak("v1"))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotModified))
		assert.Equal(t, `W/"v1"`, transport.header("grpc-metadata-etag"))
	})
}

func TestInterceptor(t *testing.T) {
	interceptor := Interceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Get"}

	t.Run("passthrough on success", func(t *testing.T) {
		transport := &mockTransport{}
		ctx := grpc.NewContextWithServerTransportStream(context.Background(), transport)
		want := &emptypb.Empty{}
		resp, err := interceptor(ctx, nil, info, func(context.Context, any) (any, error) {
			return want, nil
		})
		require.NoError(t, err)
		assert.Same(t, want, resp)
		assert.Nil(t, transport.md)
	})

	t.Run("passthrough on unrelated error", func(t *testing.T) {
		transport := &mockTransport{}
		ctx := grpc.NewContextWithServerTransportStream(context.Background(), transport)
		want := errors.New("boom")
		_, err := interceptor(ctx, nil, info, func(context.Context, any) (any, error) {
			return nil, want
		})
		assert.True(t, errors.Is(err, want))
		assert.Nil(t, transport.md)
	})

	t.Run("not-modified emits 304 and flag", func(t *testing.T) {
		transport := &mockTransport{}
		ctx := grpc.NewContextWithServerTransportStream(context.Background(), transport)
		resp, err := interceptor(ctx, nil, info, func(context.Context, any) (any, error) {
			return nil, ErrNotModified
		})
		require.NoError(t, err)
		assert.IsType(t, &emptypb.Empty{}, resp)
		assert.Equal(t, "304", transport.header("x-http-code"))
		assert.Equal(t, "1", transport.header(MetaNotModified))
	})
}

func TestClientHelpers(t *testing.T) {
	t.Run("IfNoneMatch sets outgoing metadata", func(t *testing.T) {
		ctx := IfNoneMatch(context.Background(), Strong("v1"))
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{`"v1"`}, md.Get("if-none-match"))
	})

	t.Run("IsNotModified and ETag read response metadata", func(t *testing.T) {
		md := metadata.MD{}
		assert.False(t, IsNotModified(md))
		assert.Empty(t, ETag(md))

		md.Set(MetaNotModified, "1")
		md.Set("etag", Weak("v1"))
		assert.True(t, IsNotModified(md))
		assert.Equal(t, `W/"v1"`, ETag(md))
	})
}

// mockTransport records header metadata set via grpc.SetHeader.
type mockTransport struct {
	md *metadata.MD
}

func (m *mockTransport) header(key string) string {
	if m.md == nil {
		return ""
	}
	if v := m.md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}

func (m *mockTransport) Method() string { return "test" }

func (m *mockTransport) SetHeader(md metadata.MD) error {
	if m.md == nil {
		m.md = &md
		return nil
	}
	for k, v := range md {
		(*m.md)[k] = v
	}
	return nil
}

func (m *mockTransport) SendHeader(md metadata.MD) error { return nil }
func (m *mockTransport) SetTrailer(md metadata.MD) error { return nil }
