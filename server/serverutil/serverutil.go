package serverutil

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// CookiesFromIncomingContext reads a standard HTTP cookie header from the GRPC
// metadata and parses the contents.
func CookiesFromIncomingContext(ctx context.Context) map[string]*http.Cookie {
	md, _ := metadata.FromIncomingContext(ctx)
	r := &http.Request{Header: http.Header{}}
	for _, v := range md[runtime.MetadataPrefix+"cookie"] {
		r.Header.Add("Cookie", v)
	}
	cookies := map[string]*http.Cookie{}
	for _, c := range r.Cookies() {
		cookies[c.Name] = c
	}
	return cookies
}

// SendCookie adds an http-set-cookie header for the provided cookie to the
// outgoing GRPC metadata.
func SendCookie(ctx context.Context, cookie *http.Cookie) error {
	if err := cookie.Valid(); err != nil {
		return err
	}
	if err := grpc.SetHeader(ctx, metadata.New(map[string]string{
		"grpc-metadata-set-cookie": cookie.String(),
	})); err != nil {
		return err
	}
	return nil
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
