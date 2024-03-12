package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/server/serverutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// Header name used by XHR requests to pass CSRF checks.
	// See https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html#employing-custom-request-headers-for-ajaxapi
	csrfHeader = "x-csrf-protection"

	// Cookie name used for storing the CSRF token.
	csrfCookie = "pf-ct"

	// Query param and metadata key used for double-submit cookie pattern.
	csrfParam = "csrf-token"

	// Duration for which the CSRF token is valid.
	csrfExpiration = time.Hour * 6
)

// SendCSRFToken sends a CSRF token in the response cookies and returns the
// value for use in the response body.
func SendCSRFToken(ctx context.Context, signingKey []byte) string {
	ct := csrfTokenFromCookie(ctx)
	if ct == "" {
		ct = generateCSRFToken(signingKey)
	}

	// Resend the cookie so we can push out expiration.
	isSecure := strings.HasPrefix(serverutil.AddressFromContext(ctx), "https")
	err := serverutil.SendCookie(ctx, &http.Cookie{
		Name:     csrfCookie,
		Value:    ct,
		Path:     "/",
		Secure:   isSecure,
		HttpOnly: false, // Per OWASP recommendation.
		Expires:  time.Now().Add(csrfExpiration),
		SameSite: http.SameSiteLaxMode,
	})
	if err != nil {
		// This error will occur when the ctx hasn't gone through the GRPC stack,
		// since that is a configuration error, we panic.
		panic("csrf: failed to send cookie: " + err.Error())
	}

	return ct
}

// VerifyCSRF checks the incoming request for a CSRF token. It looks for the
// token in the query params and cookies, and verifies that they match. If the
// token is missing or invalid, an error is returned.
func VerifyCSRF(ctx context.Context, signingKey []byte) error {
	if h := serverutil.HttpHeader(ctx, csrfHeader); h != "" {
		// Simply the presence of the header is enough.
		return nil
	}

	md, _ := metadata.FromIncomingContext(ctx)
	params := md.Get(csrfParam)
	if len(params) == 0 || params[0] == "" {
		return status.Errorf(codes.FailedPrecondition, "csrf: missing token in request")
	}

	fromCookie := csrfTokenFromCookie(ctx)
	if fromCookie == "" {
		return status.Errorf(codes.FailedPrecondition, "csrf: missing token in cookies")
	}

	if params[0] != fromCookie {
		return status.Errorf(codes.FailedPrecondition, "csrf: token mismatch")
	}

	return verifyCSRFToken(fromCookie, signingKey)
}

func csrfTokenFromCookie(ctx context.Context) string {
	cookies := serverutil.CookiesFromIncomingContext(ctx)
	c, ok := cookies[csrfCookie]
	if !ok {
		return ""
	}
	return c.Value
}

func generateCSRFToken(signingKey []byte) string {
	randomData := make([]byte, 32)
	if _, err := rand.Read(randomData); err != nil {
		// Errors should not occur under normal operation and are unlikely to be
		// recoverable. So let it fail hard.
		panic("csrf: random number generation failed: " + err.Error())
	}

	hasher := hmac.New(sha256.New, []byte(signingKey))
	hasher.Write(randomData)
	mac := hex.EncodeToString(hasher.Sum(nil))

	return mac + "_" + hex.EncodeToString(randomData)
}

func verifyCSRFToken(token string, signingKey []byte) error {
	parts := strings.SplitN(token, "_", 2)
	if len(parts) != 2 {
		return status.Errorf(codes.FailedPrecondition, "csrf: invalid token")
	}

	actualMac, err := hex.DecodeString(parts[0])
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "csrf: invalid signature")
	}

	randomData, err := hex.DecodeString(parts[1])
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "csrf: invalid data")
	}

	hasher := hmac.New(sha256.New, []byte(signingKey))
	hasher.Write(randomData)
	expectedMac := hasher.Sum(nil)

	if !hmac.Equal(actualMac, expectedMac) {
		return status.Errorf(codes.FailedPrecondition, "csrf: signature mismatch")
	}

	return nil
}

// Gateway option that maps the CSRF query-param to incoming GRPC metadata.
func csrfMetadataAnnotator(_ context.Context, r *http.Request) metadata.MD {
	md := map[string]string{}
	ct := r.URL.Query().Get(csrfParam)
	if ct != "" {
		md[csrfParam] = ct
	}
	return metadata.New(md)
}

// GRPC interceptor that handles CSRF checks.
func csrfInterceptor(signingKey []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		mode := "auto"
		if v, ok := serverutil.MethodOption(info, E_CsrfMode); ok {
			mode = strings.ToLower(v.(string))
		}

		if mode == "off" {
			logging.Track(ctx, "server.csrf_mode", "off")
			return handler(ctx, req)
		}

		if mode == "auto" {
			if httpMethod := serverutil.HttpMethod(ctx); httpMethod == "" {
				// If no HTTP method, assume non-browser client and skip checks.
				logging.Track(ctx, "server.csrf_mode", "auto-off")
				return handler(ctx, req)
			} else if httpMethod == "GET" || httpMethod == "HEAD" || httpMethod == "OPTIONS" {
				// None mutating requests, generally don't need protection.
				logging.Track(ctx, "server.csrf_mode", "auto-off")
				return handler(ctx, req)
			}
			logging.Track(ctx, "server.csrf_mode", "auto-on")
		} else {
			logging.Track(ctx, "server.csrf_mode", "on")
		}

		// If we're here, we need to verify the CSRF token.
		if err := VerifyCSRF(ctx, signingKey); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}
