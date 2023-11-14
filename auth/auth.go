// Package auth provides utilities for authenticating requests. It is intended
// to be used in conjunction with identity providers and ACLs. On its own, it
// should not be used for authorization of access to resources.
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
	"github.com/spf13/viper"
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
	ErrInvalid = status.Error(codes.InvalidArgument, "token is invalid")

	// TODO: Move to pluggable configuration, support multiple registed signing
	// keys.
	jwtSigningKey = []byte("In a world of prefab dreams, authenticity gleams.")

	// Only allow tokens created by prefab.
	jwtIssuer = "prefab"

	// TODO: Make pluggable with hostname for server.
	jwtAudience = "access"

	// TODO: Customize token expiry.
	identityExpiration = time.Hour * 24 * 30

	// Allows for time to be stubbed in tests.
	timeFunc = time.Now
)

const (
	// Cookie name used for storing the prefab identity token.
	IdentityTokenCookieName = "pfid"
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
	// TODO: Can/should config be injected in via the context to avoid utility
	// methods reaching into Viper?
	isSecure := strings.HasPrefix(viper.GetString("address"), "https")

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
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Audience:  jwt.ClaimStrings{jwtAudience},
			ExpiresAt: jwt.NewNumericDate(timeFunc().Add(identityExpiration)),
			IssuedAt:  jwt.NewNumericDate(timeFunc()),
			Issuer:    jwtIssuer,
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
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return jwtSigningKey, nil
		},
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(jwtAudience),
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

	return Identity{}, ErrInvalid
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
	a, ok := md["authorization"] // GRPC Gateway forwards header without prefix.

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
			return Identity{}, ErrInvalid
		}
		return ParseIdentityToken(ctx, pair[0])

	default:
		return Identity{}, ErrInvalid
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
