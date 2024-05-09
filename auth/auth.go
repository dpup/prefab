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

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/serverutil"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

var (
	// No identity was found within the incoming context.
	ErrNotFound = errors.NewC("identity not found", codes.Unauthenticated)

	// The token's expiration date was in the past.
	ErrExpired = errors.NewC("token has expired", codes.Unauthenticated)

	// The token was not signed correctly.
	ErrInvalidToken = errors.NewC("token is invalid", codes.InvalidArgument)

	// Invalid authorization header.
	ErrInvalidHeader = errors.NewC("bad authorization header", codes.InvalidArgument)

	// Identity token has been revoked or blocked.
	ErrRevoked = errors.NewC("token has been revoked", codes.Unauthenticated)

	// Allows for time to be stubbed in tests.
	timeFunc = time.Now
)

const (
	// Cookie name used for storing the prefab identity token.
	IdentityTokenCookieName = "pf-id"
)

// Claims registered as part of a prefab identity token.
type Claims struct {
	// Standard public JWT claims per https://www.iana.org/assignments/jwt/jwt.xhtml
	jwt.RegisteredClaims
	Name          string           `json:"name"`
	Email         string           `json:"email"`
	EmailVerified bool             `json:"email_verified"`
	AuthTime      *jwt.NumericDate `json:"auth_time,omitempty"`

	// Custom claims.
	Provider string `json:"idp"`
}

func (c *Claims) Validate() error {
	if c.Provider == "" {
		return errors.Mark(ErrInvalidToken, 0).Append("missing provider")
	}
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
		Expires:  time.Now().Add(expirationFromContext(ctx)),
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
			ID:        identity.SessionID,
			Subject:   identity.Subject,
			Audience:  jwt.ClaimStrings{address},
			Issuer:    address,
			IssuedAt:  jwt.NewNumericDate(timeFunc()),
			ExpiresAt: jwt.NewNumericDate(timeFunc().Add(expirationFromContext(ctx))),
		},
		Name:          identity.Name,
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
		Provider:      identity.Provider,
		AuthTime:      jwt.NewNumericDate(identity.AuthTime),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(signingKeyFromContext(ctx))
	if err != nil {
		return "", errors.Wrap(err, 0).WithCode(codes.Unauthenticated)
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
			return signingKeyFromContext(ctx), nil
		},
		jwt.WithIssuer(address), // TODO: Possibly relax to allow tokens created by other issuers.
		jwt.WithAudience(address),
		jwt.WithLeeway(5*time.Second),
		jwt.WithTimeFunc(timeFunc),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return Identity{}, errors.Wrap(err, 0).WithCode(codes.Unauthenticated)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if err := claims.Validate(); err != nil {
			return Identity{}, err
		}

		// Check to see if the token has been revoked or blocked.
		if blocked, err := IsBlocked(ctx, claims.ID); blocked || err != nil {
			if err != nil {
				return Identity{}, err
			}
			return Identity{}, ErrRevoked
		}

		return Identity{
			Provider:      claims.Provider,
			SessionID:     claims.ID,
			AuthTime:      claims.AuthTime.Time,
			Subject:       claims.Subject,
			Email:         claims.Email,
			EmailVerified: claims.EmailVerified,
			Name:          claims.Name,
		}, nil
	}

	return Identity{}, errors.Mark(ErrInvalidToken, 0).Append("invalid claims")
}

// IdentityFromContext parses and verifies a JWT received from incoming GRPC
// metadata. An `Authorization` header will take precedence over a `Cookie`,
func IdentityFromContext(ctx context.Context) (Identity, error) {
	i, err := identityFromAuthHeader(ctx)
	if !errors.Is(err, ErrNotFound) {
		return i, err
	}
	return identityFromCookie(ctx)
}

func identityFromAuthHeader(ctx context.Context) (Identity, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	a, ok := md["authorization"] // GRPC Gateway forwards this header without prefix.

	if !ok || len(a) == 0 || a[0] == "" {
		return Identity{}, errors.Mark(ErrNotFound, 0)
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
			return Identity{}, errors.Mark(ErrInvalidHeader, 0)
		}
		return ParseIdentityToken(ctx, pair[0])

	default:
		return Identity{}, errors.Mark(ErrInvalidHeader, 0)
	}
}

func identityFromCookie(ctx context.Context) (Identity, error) {
	cookies := serverutil.CookiesFromIncomingContext(ctx)
	c, ok := cookies[IdentityTokenCookieName]
	if !ok {
		return Identity{}, errors.Mark(ErrNotFound, 0)
	}
	identity, err := ParseIdentityToken(ctx, c.Value)
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}

// ContextWithIdentityForTest creates a new context with the given identity
// attached. This is useful for testing, where we want to simulate a request
// with a given identity.
func ContextWithIdentityForTest(ctx context.Context, identity Identity) context.Context {
	if identity == (Identity{}) {
		// Short-circuity to avoid serialization/deserialization of empty identity.
		return ctx
	}
	tokenString, _ := IdentityToken(ctx, identity)
	md := metadata.Pairs("authorization", tokenString)
	return metadata.NewIncomingContext(ctx, md)
}
