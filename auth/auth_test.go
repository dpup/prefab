package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestTokenRoundTrip(t *testing.T) {
	ctx := context.Background()

	original := Identity{
		Subject: "1",
		// Use JWT time library to make sure the dates have the same precision when comparing in tests.
		AuthTime:      jwt.NewNumericDate(time.Now()).Time,
		Email:         "andor@aldhani-rebels.org",
		EmailVerified: true,
		Name:          "Casian Andor",
	}

	tokenString, err := IdentityToken(ctx, original)
	assert.Nil(t, err, "failed to issue token")

	parsed, err := ParseIdentityToken(ctx, tokenString)
	assert.Nil(t, err, "failed to parse token")

	assert.Equal(t, original, parsed, "Parsed and original identities do not match")
}

func TestTokenExpiration(t *testing.T) {
	ctx := context.Background()
	identity := Identity{Subject: "2"}

	tokenString, err := IdentityToken(ctx, identity)
	assert.Nil(t, err, "failed to issue token")

	// Stub time to return a time in the future.
	timeFunc = func() time.Time {
		return time.Now().Add(time.Hour * 24 * 365)
	}
	defer func() {
		timeFunc = time.Now
	}()

	_, err = ParseIdentityToken(ctx, tokenString)
	assert.EqualError(t, err, "rpc error: code = Unauthenticated desc = token has invalid claims: token is expired")
}

func TestTokenSigning(t *testing.T) {
	ctx := context.Background()
	identity := Identity{Subject: "2"}

	originalKey := jwtSigningKey
	jwtSigningKey = []byte("sneaky")

	tokenString, err := IdentityToken(ctx, identity)
	assert.Nil(t, err, "failed to issue token")

	jwtSigningKey = originalKey

	_, err = ParseIdentityToken(ctx, tokenString)
	assert.EqualError(t, err, "rpc error: code = Unauthenticated desc = token signature is invalid: signature is invalid")
}

func TestIdentityFromEmptyContext(t *testing.T) {
	ctx := context.Background()

	_, err := IdentityFromContext(ctx)
	assert.EqualError(t, err, "rpc error: code = Unauthenticated desc = identity not found")
}

func TestIdentityFromCookie(t *testing.T) {
	ctx := context.Background()

	expected := Identity{
		Subject:  "3",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
	}
	tokenString, err := IdentityToken(ctx, expected)
	assert.Nil(t, err, "failed to issue token")

	md := metadata.Pairs("grpcgateway-cookie", fmt.Sprintf("%s=%s", IdentityTokenCookieName, tokenString))
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	assert.Nil(t, err, "failed to extract identity: %v", err)

	assert.Equal(t, expected, actual, "identity from cookie does not match")
}

func TestIdentityFromAuthHeader(t *testing.T) {
	ctx := context.Background()

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
	}
	tokenString, err := IdentityToken(ctx, expected)
	assert.Nil(t, err, "failed to issue token")

	md := metadata.Pairs("authorization", tokenString)
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	assert.Nil(t, err, "failed to extract identity")

	assert.Equal(t, expected, actual, "identity from header does not match")
}

func TestIdentityFromBearerToken(t *testing.T) {
	ctx := context.Background()

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
	}
	tokenString, err := IdentityToken(ctx, expected)
	assert.Nil(t, err, "failed to issue token")

	md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", tokenString))
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	assert.Nil(t, err, "failed to extract identity")

	assert.Equal(t, expected, actual, "identity from header does not match")
}

func TestIdentityFromBasicAuth(t *testing.T) {
	ctx := context.Background()

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
	}
	tokenString, err := IdentityToken(ctx, expected)
	assert.Nil(t, err, "failed to issue token")

	basic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:", tokenString)))
	md := metadata.Pairs("authorization", fmt.Sprintf("basic %s", basic))
	ctx = metadata.NewIncomingContext(ctx, md)

	actual, err := IdentityFromContext(ctx)
	assert.Nil(t, err, "failed to extract identity")

	assert.Equal(t, expected, actual, "identity from header does not match")
}

func TestIdentityFromBasicAuth_invalidBasicAuth(t *testing.T) {
	ctx := context.Background()

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
	}
	tokenString, err := IdentityToken(ctx, expected)
	assert.Nil(t, err, "failed to issue token")

	basic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:password", tokenString)))
	md := metadata.Pairs("authorization", fmt.Sprintf("basic %s", basic))
	ctx = metadata.NewIncomingContext(ctx, md)

	_, err = IdentityFromContext(ctx)
	assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = bad authorization header")
}

func TestIdentityFromBasicAuth_invalidAuthorizationType(t *testing.T) {
	ctx := context.Background()

	expected := Identity{
		Subject:  "4",
		AuthTime: jwt.NewNumericDate(time.Now()).Time,
	}
	tokenString, err := IdentityToken(ctx, expected)
	assert.Nil(t, err, "failed to issue token")

	basic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:", tokenString)))
	md := metadata.Pairs("authorization", fmt.Sprintf("xxxxx %s", basic))
	ctx = metadata.NewIncomingContext(ctx, md)

	_, err = IdentityFromContext(ctx)
	assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = bad authorization header")
}
