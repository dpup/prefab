package serverutil

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestSendCookie(t *testing.T) {
	t.Run("ValidCookie", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		cookie := &http.Cookie{
			Name:     "test-cookie",
			Value:    "test-value",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		}

		err := SendCookie(ctx, cookie)
		require.NoError(t, err)

		// Verify cookie was set in metadata
		require.NotNil(t, mockTransport.md)
		setCookieHeaders := (*mockTransport.md)["grpc-metadata-set-cookie"]
		require.Len(t, setCookieHeaders, 1)

		cookieStr := setCookieHeaders[0]
		assert.Contains(t, cookieStr, "test-cookie=test-value")
		assert.Contains(t, cookieStr, "Path=/")
		assert.Contains(t, cookieStr, "HttpOnly")
		assert.Contains(t, cookieStr, "Secure")
		assert.Contains(t, cookieStr, "SameSite=Lax")
	})

	t.Run("InvalidCookie", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		// Cookie with invalid name (contains space)
		cookie := &http.Cookie{
			Name:  "invalid name",
			Value: "value",
		}

		err := SendCookie(ctx, cookie)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("CookieWithExpiration", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		expires := time.Now().Add(24 * time.Hour)
		cookie := &http.Cookie{
			Name:    "expiring-cookie",
			Value:   "value",
			Expires: expires,
		}

		err := SendCookie(ctx, cookie)
		require.NoError(t, err)

		setCookieHeaders := (*mockTransport.md)["grpc-metadata-set-cookie"]
		require.Len(t, setCookieHeaders, 1)
		assert.Contains(t, setCookieHeaders[0], "Expires=")
	})
}

func TestSendHeader(t *testing.T) {
	t.Run("SingleHeader", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		err := SendHeader(ctx, "x-custom-header", "custom-value")
		require.NoError(t, err)

		require.NotNil(t, mockTransport.md)
		headerValues := (*mockTransport.md)["grpc-metadata-x-custom-header"]
		require.Len(t, headerValues, 1)
		assert.Equal(t, "custom-value", headerValues[0])
	})

	t.Run("MultipleHeaders", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		err := SendHeader(ctx, "x-header-1", "value-1")
		require.NoError(t, err)

		err = SendHeader(ctx, "x-header-2", "value-2")
		require.NoError(t, err)

		require.NotNil(t, mockTransport.md)
		assert.Equal(t, "value-1", (*mockTransport.md)["grpc-metadata-x-header-1"][0])
		assert.Equal(t, "value-2", (*mockTransport.md)["grpc-metadata-x-header-2"][0])
	})

	t.Run("EmptyValue", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		err := SendHeader(ctx, "x-empty", "")
		require.NoError(t, err)

		headerValues := (*mockTransport.md)["grpc-metadata-x-empty"]
		require.Len(t, headerValues, 1)
		assert.Empty(t, headerValues[0])
	})
}

func TestSendStatusCode(t *testing.T) {
	t.Run("StandardStatusCode", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		err := SendStatusCode(ctx, 200)
		require.NoError(t, err)

		require.NotNil(t, mockTransport.md)
		statusValues := (*mockTransport.md)["x-http-code"]
		require.Len(t, statusValues, 1)
		assert.Equal(t, "200", statusValues[0])
	})

	t.Run("RedirectStatusCode", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		err := SendStatusCode(ctx, 302)
		require.NoError(t, err)

		statusValues := (*mockTransport.md)["x-http-code"]
		require.Len(t, statusValues, 1)
		assert.Equal(t, "302", statusValues[0])
	})

	t.Run("ErrorStatusCode", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)

		err := SendStatusCode(ctx, 404)
		require.NoError(t, err)

		statusValues := (*mockTransport.md)["x-http-code"]
		require.Len(t, statusValues, 1)
		assert.Equal(t, "404", statusValues[0])
	})
}

func TestParseCookies(t *testing.T) {
	t.Run("SingleCookie", func(t *testing.T) {
		cookies := ParseCookies("session=abc123")

		require.Len(t, cookies, 1)
		assert.Equal(t, "session", cookies["session"].Name)
		assert.Equal(t, "abc123", cookies["session"].Value)
	})

	t.Run("MultipleCookies", func(t *testing.T) {
		cookies := ParseCookies("session=abc123; user=john")

		require.Len(t, cookies, 2)
		assert.Equal(t, "abc123", cookies["session"].Value)
		assert.Equal(t, "john", cookies["user"].Value)
	})

	t.Run("MultipleHeaderStrings", func(t *testing.T) {
		cookies := ParseCookies("session=abc123", "user=john")

		require.Len(t, cookies, 2)
		assert.Equal(t, "abc123", cookies["session"].Value)
		assert.Equal(t, "john", cookies["user"].Value)
	})

	t.Run("EmptyCookie", func(t *testing.T) {
		cookies := ParseCookies("")

		assert.Empty(t, cookies)
	})

	t.Run("NoCookies", func(t *testing.T) {
		cookies := ParseCookies()

		assert.Empty(t, cookies)
	})

	t.Run("CookieWithSpecialChars", func(t *testing.T) {
		cookies := ParseCookies("token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")

		require.Len(t, cookies, 1)
		assert.Equal(t, "token", cookies["token"].Name)
		assert.Equal(t, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", cookies["token"].Value)
	})

	t.Run("DuplicateCookieNames", func(t *testing.T) {
		// Last cookie with same name wins
		cookies := ParseCookies("name=first; name=second")

		require.Len(t, cookies, 1)
		assert.Equal(t, "second", cookies["name"].Value)
	})
}

func TestCookiesFromIncomingContext(t *testing.T) {
	t.Run("WithCookies", func(t *testing.T) {
		md := metadata.Pairs("grpcgateway-cookie", "session=abc123; user=john")
		ctx := metadata.NewIncomingContext(t.Context(), md)

		cookies := CookiesFromIncomingContext(ctx)

		require.Len(t, cookies, 2)
		assert.Equal(t, "abc123", cookies["session"].Value)
		assert.Equal(t, "john", cookies["user"].Value)
	})

	t.Run("WithoutCookies", func(t *testing.T) {
		ctx := t.Context()

		cookies := CookiesFromIncomingContext(ctx)

		assert.Empty(t, cookies)
	})

	t.Run("WithEmptyCookie", func(t *testing.T) {
		md := metadata.Pairs("grpcgateway-cookie", "")
		ctx := metadata.NewIncomingContext(t.Context(), md)

		cookies := CookiesFromIncomingContext(ctx)

		assert.Empty(t, cookies)
	})

	t.Run("MultipleCookieHeaders", func(t *testing.T) {
		md := metadata.MD{
			"grpcgateway-cookie": []string{"session=abc123", "user=john"},
		}
		ctx := metadata.NewIncomingContext(t.Context(), md)

		cookies := CookiesFromIncomingContext(ctx)

		require.Len(t, cookies, 2)
		assert.Equal(t, "abc123", cookies["session"].Value)
		assert.Equal(t, "john", cookies["user"].Value)
	})
}

// mockServerTransportStream implements grpc.ServerTransportStream for testing
type mockServerTransportStream struct {
	md *metadata.MD
}

func (m *mockServerTransportStream) Method() string {
	return "test"
}

func (m *mockServerTransportStream) SetHeader(md metadata.MD) error {
	if m.md == nil {
		m.md = &md
	} else {
		// Merge metadata
		for k, v := range md {
			(*m.md)[k] = v
		}
	}
	return nil
}

func (m *mockServerTransportStream) SendHeader(md metadata.MD) error {
	panic("Not implemented")
}

func (m *mockServerTransportStream) SetTrailer(md metadata.MD) error {
	panic("Not implemented")
}
