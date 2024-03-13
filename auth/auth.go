// Package auth provides utilities for authenticating requests.
//
// Authentication is delegated to an identity provider, which is responsible for
// verifying the identity of the client and returning an identity token. The
// identity token is then signed and returned to the client as a JWT — either as
// a cookie or in the response.
//
// The client then uses the cookie, or token, to authenticate future requests.
//
// Identity Providers should implement the LoginHandler interface, and register
// it with AddLoginHandler. This will hook the provider into the Login endpoint
// and allow it to handle login requests.
//
// Observe the google or magiclink example for a complete example of how to use
// this package.
package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/dpup/prefab/server/serverutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	// No identity was found within the incoming context.
	ErrNotFound = status.Error(codes.NotFound, "identity not found")

	// The token's expiration date was in the past.
	ErrExpired = status.Error(codes.FailedPrecondition, "token has expired")

	// The token was not signed correctly.
	ErrInvalidToken = status.Error(codes.InvalidArgument, "token is invalid")

	// Invalid authorization header.
	ErrInvalidHeader = status.Error(codes.InvalidArgument, "bad authorization header")

	// TODO: Move to pluggable configuration, support multiple registed signing
	// keys.
	jwtSigningKey = []byte("In a world of prefab dreams, authenticity gleams.")

	// TODO: Customize token expiry.
	identityExpiration = time.Hour * 24 * 30

	// Allows for time to be stubbed in tests.
	timeFunc = time.Now
)

const (
	// Cookie name used for storing the prefab identity token.
	IdentityTokenCookieName = "pf-id"
)

type Claims struct {
	jwt.RegisteredClaims
	Name          string           `json:"name"`
	Email         string           `json:"email"`
	EmailVerified bool             `json:"email_verified"`
	AuthTime      *jwt.NumericDate `json:"auth_time,omitempty"`
}

func (c *Claims) Validate() error {
	// No custom validation for now.
	return nil
}

// SendIdentityCookie attaches the token to the outgoing GRPC metadata such
// that it will be propagated as a `Set-Cookie` HTTP header by the Gateway.
func SendIdentityCookie(ctx context.Context, token string) error {
	address := serverutil.AddressFromContext(ctx)
	isSecure := strings.HasPrefix(address, "https")

	return serverutil.SendCookie(ctx, &http.Cookie{
		Name:     IdentityTokenCookieName,
		Value:    token,
		Path:     "/",
		Secure:   isSecure,
		HttpOnly: true,
		Expires:  time.Now().Add(identityExpiration),
		SameSite: http.SameSiteLaxMode,
	})
}

// IdentityToken creates a signed JWT for the given identity.
func IdentityToken(ctx context.Context, identity Identity) (string, error) {
	// Both issuer and audience are set to the current server, indicating that the
	// token was created by this server and is only intended to be used for this
	// server.
	address := serverutil.AddressFromContext(ctx)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Audience:  jwt.ClaimStrings{address},
			Issuer:    address,
			IssuedAt:  jwt.NewNumericDate(timeFunc()),
			ExpiresAt: jwt.NewNumericDate(timeFunc().Add(identityExpiration)),
			Subject:   identity.Subject,
		},
		Name:          identity.Name,
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
		AuthTime:      jwt.NewNumericDate(identity.AuthTime),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(jwtSigningKey)
	if err != nil {
		return "", err
	}
	return ss, nil
}

// ParseIdentityToken takes a signed JWT, validates it, and returns the identity
// information encoded within. Invalid and expired tokens will error.
func ParseIdentityToken(ctx context.Context, tokenString string) (Identity, error) {
	address := serverutil.AddressFromContext(ctx)

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return jwtSigningKey, nil
		},
		jwt.WithIssuer(address), // TODO: Possibly relax to allow tokens created by other issuers.
		jwt.WithAudience(address),
		jwt.WithLeeway(5*time.Second),
		jwt.WithTimeFunc(timeFunc),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return Identity{}, status.Error(codes.InvalidArgument, err.Error())
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return Identity{
			AuthTime:      claims.AuthTime.Time,
			Subject:       claims.Subject,
			Email:         claims.Email,
			EmailVerified: claims.EmailVerified,
			Name:          claims.Name,
		}, nil
	}

	return Identity{}, ErrInvalidToken
}

// IdentityFromContext parses and verifies a JWT received from incoming GRPC
// metadata. An `Authorization` header will take precedence over a `Cookie`,
func IdentityFromContext(ctx context.Context) (Identity, error) {
	i, err := identityFromAuthHeader(ctx)
	if err != ErrNotFound {
		return i, err
	}
	return identityFromCookie(ctx)
}

func identityFromAuthHeader(ctx context.Context) (Identity, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	a, ok := md["authorization"] // GRPC Gateway forwards this header without prefix.

	if !ok || len(a) == 0 || a[0] == "" {
		return Identity{}, ErrNotFound
	}

	auth := strings.SplitN(a[0], " ", 2)

	if len(auth) != 2 {
		// Relaxed fallback that allows tokens to be passed without the "bearer"
		// or "basic" prefix. Instead it takes the whole header.
		return ParseIdentityToken(ctx, a[0])
	}

	switch strings.ToLower(auth[0]) {
	case "bearer":
		// Standard bearer token.
		return ParseIdentityToken(ctx, auth[1])

	case "basic":
		// Basic auth is the method preferred for curl based CLI clients.
		// By convention, we expect the username to be the "Access Token", and for
		// there to be no password.
		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[1] != "" {
			return Identity{}, ErrInvalidHeader
		}
		return ParseIdentityToken(ctx, pair[0])

	default:
		return Identity{}, ErrInvalidHeader
	}
}

// TODO: Should we support multiple identities from cookies?
func identityFromCookie(ctx context.Context) (Identity, error) {
	cookies := serverutil.CookiesFromIncomingContext(ctx)
	c, ok := cookies[IdentityTokenCookieName]
	if !ok {
		return Identity{}, ErrNotFound
	}
	identity, err := ParseIdentityToken(ctx, c.Value)
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}
