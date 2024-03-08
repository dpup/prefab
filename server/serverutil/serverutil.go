package serverutil

import (
	"context"
	"net/http"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// CookiesFromIncomingContext reads a standard HTTP cookie header from the GRPC
// metadata and parses the contents.
func CookiesFromIncomingContext(ctx context.Context) map[string]*http.Cookie {
	md, _ := metadata.FromIncomingContext(ctx)
	return ParseCookies(md[runtime.MetadataPrefix+"cookie"]...)
}

// SendCookie adds an http-set-cookie header for the provided cookie to the
// outgoing GRPC metadata.
func SendCookie(ctx context.Context, cookie *http.Cookie) error {
	if err := cookie.Valid(); err != nil {
		return err
	}
	return SendHeader(ctx, "set-cookie", cookie.String())
}

// SendHeader adds an http header to the outgoing GRPC metadata for forwarding.
func SendHeader(ctx context.Context, key, value string) error {
	if err := grpc.SetHeader(ctx, metadata.New(map[string]string{
		"grpc-metadata-" + key: value,
	})); err != nil {
		return err
	}
	return nil
}

// SendStatusCode adds an http status code header to the outgoing GRPC metadata.
//
// The GRPC Gateway will send this as the actual status code via the
// `statusCodeForwarder` function.
func SendStatusCode(ctx context.Context, code int) error {
	return grpc.SetHeader(ctx, metadata.Pairs("x-http-code", strconv.Itoa(code)))
}

// ParseCookies takes a cookie header string and returns a map of cookies.
func ParseCookies(headers ...string) map[string]*http.Cookie {
	r := &http.Request{Header: http.Header{}}
	for _, h := range headers {
		r.Header.Add("Cookie", h)
	}
	cookies := map[string]*http.Cookie{}
	for _, c := range r.Cookies() {
		cookies[c.Name] = c
	}
	return cookies
}
