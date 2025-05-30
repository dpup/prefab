package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/storage/memstore"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestTokenRoundTrip(t *testing.T) {
	ctx := t.Context()

	original := Identity{
		Subject:  "1",
		Provider: "test",
		// Use JWT time library to make sure the dates have the same precision when comparing in tests.
		AuthTime:      jwt.NewNumericDate(time.Now()).Time,
		Email:         "andor@aldhani-rebels.org",
		EmailVerified: true,
		Name:          "Casian Andor",
	}

	tokenString, err := IdentityToken(ctx, original)
	require.NoError(t, err, "failed to issue token")

	parsed, err := ParseIdentityToken(ctx, tokenString)
	require.NoError(t, err, "failed to parse token")

	assert.Equal(t, original, parsed, "Parsed and original identities do not match")
}

func TestTokenExpiration(t *testing.T) {
	ctx := t.Context()
	identity := Identity{Subject: "2", Provider: "test"}

	tokenString, err := IdentityToken(ctx, identity)
	require.NoError(t, err, "failed to issue token")

	// Stub time to return a time in the future.
	timeFunc = func() time.Time {
		return time.Now().Add(time.Hour * 24 * 365)
	}
	defer func() {
		timeFunc = time.Now
	}()

	_, err = ParseIdentityToken(ctx, tokenString)
	assert.EqualError(t, err, "token has invalid claims: token is expired")
}

func TestTokenSigning(t *testing.T) {
	ctx := t.Context()
	identity := Identity{Subject: "2", Provider: "test"}

	tokenString, err := IdentityToken(injectSigningKey("evil")(ctx), identity)
	require.NoError(t, err, "failed to issue token")

	_, err = ParseIdentityToken(injectSigningKey("actual")(ctx), tokenString)
	assert.EqualError(t, err, "token signature is invalid: signature is invalid")
}
func TestIdentityFromEmptyContext(t *testing.T) {
	ctx := WithIdentityExtractorsForTest(t.Context())
	_, err := IdentityFromContext(ctx)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestIdentityFromCookie(t *testing.T) {
	ctx := WithIdentityExtractorsForTest(t.Context())

	expected := Identity{
		Subject:  "3",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
		Provider: "test",
	}
	tokenString, err := IdentityToken(ctx, expected)
	require.NoError(t, err, "failed to issue token")

	md := metadata.Pairs("grpcgateway-cookie", fmt.Sprintf("%s=%s", IdentityTokenCookieName, tokenString))
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	require.NoError(t, err, "failed to extract identity: %v", err)

	assert.Equal(t, expected, actual, "identity from cookie does not match")
}

func TestIdentityFromAuthHeader(t *testing.T) {
	ctx := WithIdentityExtractorsForTest(t.Context())

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
		Provider: "test",
	}
	tokenString, err := IdentityToken(ctx, expected)
	require.NoError(t, err, "failed to issue token")

	md := metadata.Pairs("authorization", tokenString)
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	require.NoError(t, err, "failed to extract identity")

	assert.Equal(t, expected, actual, "identity from header does not match")
}

func TestIdentityFromBearerToken(t *testing.T) {
	ctx := WithIdentityExtractorsForTest(t.Context())

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
		Provider: "test",
	}
	tokenString, err := IdentityToken(ctx, expected)
	require.NoError(t, err, "failed to issue token")

	md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", tokenString))
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	require.NoError(t, err, "failed to extract identity")

	assert.Equal(t, expected, actual, "identity from header does not match")
}

func TestIdentityFromBearerToken_missingProvider(t *testing.T) {
	ctx := WithIdentityExtractorsForTest(t.Context())
	idt := Identity{
		SessionID: "12345",
		Subject:   "4",
	}
	tokenString, err := IdentityToken(ctx, idt)
	require.NoError(t, err, "failed to issue token")

	md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", tokenString))
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	assert.Equal(t, Identity{}, actual, "expected zero Identity")
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestIdentityFromBearerToken_blocked(t *testing.T) {
	blocklist := NewBlocklist(memstore.New())
	_ = blocklist.Block("12345")

	ctx := WithBlockist(WithIdentityExtractorsForTest(t.Context()), blocklist)

	idt := Identity{
		SessionID: "12345",
		Subject:   "4",
		AuthTime:  jwt.NewNumericDate(time.Now()).Time,
		Provider:  "test",
	}
	tokenString, err := IdentityToken(ctx, idt)
	require.NoError(t, err, "failed to issue token")

	md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", tokenString))
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	assert.Equal(t, Identity{}, actual, "expected zero Identity")
	require.ErrorIs(t, err, ErrRevoked)
}

func TestIdentityFromBasicAuth(t *testing.T) {
	ctx := WithIdentityExtractorsForTest(t.Context())

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
		Provider: "test",
	}
	tokenString, err := IdentityToken(ctx, expected)
	require.NoError(t, err, "failed to issue token")

	basic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:", tokenString)))
	md := metadata.Pairs("authorization", fmt.Sprintf("basic %s", basic))
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	require.NoError(t, err, "failed to extract identity")

	assert.Equal(t, expected, actual, "identity from header does not match")
}

func TestIdentityFromBasicAuth_invalidBasicAuth(t *testing.T) {
	ctx := WithIdentityExtractorsForTest(t.Context())

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
		Provider: "test",
	}
	tokenString, err := IdentityToken(ctx, expected)
	require.NoError(t, err, "failed to issue token")

	basic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:password", tokenString)))
	md := metadata.Pairs("authorization", fmt.Sprintf("basic %s", basic))
	ctx = metadata.NewIncomingContext(ctx, md)

	_, err = IdentityFromContext(ctx)
	require.ErrorIs(t, err, ErrInvalidHeader)
}

func TestIdentityFromBasicAuth_invalidAuthorizationType(t *testing.T) {
	ctx := WithIdentityExtractorsForTest(t.Context())

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
		Provider: "test",
	}
	tokenString, err := IdentityToken(ctx, expected)
	require.NoError(t, err, "failed to issue token")

	basic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:", tokenString)))
	md := metadata.Pairs("authorization", fmt.Sprintf("xxxxx %s", basic))
	ctx = metadata.NewIncomingContext(ctx, md)

	_, err = IdentityFromContext(ctx)
	require.ErrorIs(t, err, ErrInvalidHeader)
}

func TestCustomIdentityExtractor(t *testing.T) {
	// Create a mock session lookup table
	sessionTokens := map[string]Identity{
		"session-123": {
			Subject:       "user-456",
			Provider:      "custom-provider",
			AuthTime:      jwt.NewNumericDate(time.Now()).Time,
			Email:         "custom@example.com",
			EmailVerified: true,
			Name:          "Custom User",
		},
		"invalid-token": {}, // Empty identity to simulate validation failure
	}

	// Create a custom identity extractor that simulates session token lookup
	sessionTokenExtractor := func(ctx context.Context) (Identity, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return Identity{}, errors.Mark(ErrNotFound, 0)
		}

		headers, ok := md["x-session-token"]
		if !ok || len(headers) == 0 {
			return Identity{}, errors.Mark(ErrNotFound, 0)
		}

		token := headers[0]
		identity, exists := sessionTokens[token]
		if !exists {
			return Identity{}, errors.Mark(ErrNotFound, 0).Append("session token not found")
		}

		// Validate the identity (simulating token validation)
		if identity == (Identity{}) {
			return Identity{}, errors.Mark(ErrInvalidToken, 0).Append("invalid session token")
		}

		return identity, nil
	}

	// Test the custom extractor
	baseCtx := t.Context()
	// Create a context with default extractors plus our custom one
	ctx := WithIdentityExtractors(baseCtx,
		identityFromAuthHeader,
		identityFromCookie,
		sessionTokenExtractor,
	)

	// Test with valid session token
	md := metadata.Pairs("x-session-token", "session-123")
	ctx = metadata.NewIncomingContext(ctx, md)
	actual, err := IdentityFromContext(ctx)
	require.NoError(t, err, "failed to extract identity from custom extractor")
	assert.Equal(t, sessionTokens["session-123"], actual, "identity from custom extractor does not match")

	// Test with invalid session token
	md = metadata.Pairs("x-session-token", "invalid-token")
	ctx = metadata.NewIncomingContext(ctx, md)
	_, err = IdentityFromContext(ctx)
	require.ErrorIs(t, err, ErrInvalidToken, "expected invalid token error")

	// Test with unknown session token
	md = metadata.Pairs("x-session-token", "non-existent-token")
	ctx = metadata.NewIncomingContext(ctx, md)
	_, err = IdentityFromContext(ctx)
	require.ErrorIs(t, err, ErrNotFound, "expected not found error")

	// Test that standard extractors still work when custom one is registered
	expected := Identity{
		Subject:  "auth-header-subject",
		Provider: "test-provider",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
	}
	tokenString, err := IdentityToken(baseCtx, expected)
	require.NoError(t, err, "failed to issue token")

	md = metadata.Pairs("authorization", tokenString)
	ctx = metadata.NewIncomingContext(
		WithIdentityExtractors(baseCtx, identityFromAuthHeader, sessionTokenExtractor),
		md,
	)

	actual, err = IdentityFromContext(ctx)
	require.NoError(t, err, "failed to extract identity from auth header with custom extractor present")
	assert.Equal(t, expected, actual, "identity from auth header with custom extractor present does not match")
}

func TestCustomIdentityExtractor_usingAuthHeader(t *testing.T) {
	// Create a mock session lookup table
	sessionTokens := map[string]Identity{
		"session-123": {
			Subject:       "user-456",
			Provider:      "custom-provider",
			AuthTime:      jwt.NewNumericDate(time.Now()).Time,
			Email:         "custom@example.com",
			EmailVerified: true,
			Name:          "Custom User",
		},
	}

	// Create a custom identity extractor that simulates session token lookup
	sessionTokenExtractor := func(ctx context.Context) (Identity, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return Identity{}, errors.Mark(ErrNotFound, 0)
		}

		headers, ok := md["authorization"]
		if !ok || len(headers) == 0 {
			return Identity{}, errors.Mark(ErrNotFound, 0)
		}

		token := headers[0]
		identity, exists := sessionTokens[token]
		if !exists {
			return Identity{}, errors.Mark(ErrNotFound, 0).Append("session token not found")
		}

		return identity, nil
	}

	// Test the custom extractor
	baseCtx := t.Context()
	// Create a context with default extractors plus our custom one
	ctx := WithIdentityExtractors(baseCtx,
		identityFromAuthHeader,
		identityFromCookie,
		sessionTokenExtractor,
	)

	// Test with valid session token -- should not trigger default extractors.
	md := metadata.Pairs("authorization", "session-123")
	ctx = metadata.NewIncomingContext(ctx, md)
	actual, err := IdentityFromContext(ctx)
	require.NoError(t, err, "failed to extract identity from custom extractor")
	assert.Equal(t, sessionTokens["session-123"], actual, "identity from custom extractor does not match")

	// Test that standard extractors still work when custom one is registered
	expected := Identity{
		Subject:  "auth-header-subject",
		Provider: "test-provider",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
	}
	tokenString, err := IdentityToken(baseCtx, expected)
	require.NoError(t, err, "failed to issue token")

	md = metadata.Pairs("authorization", tokenString)
	ctx = metadata.NewIncomingContext(
		WithIdentityExtractors(baseCtx, identityFromAuthHeader, sessionTokenExtractor),
		md,
	)

	actual, err = IdentityFromContext(ctx)
	require.NoError(t, err, "failed to extract identity from auth header with custom extractor present")
	assert.Equal(t, expected, actual, "identity from auth header with custom extractor present does not match")
}
