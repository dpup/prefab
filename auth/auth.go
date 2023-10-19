// Package auth provides utilities for authenticating requests. It is intended
// to be used in conjunction with identity providers and ACLs. On its own, it
// should not be used for authorization of access to resources.
package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"
)

var (
	// No identity was found within the incoming context.
	ErrNotFound = errors.New("identity not found")

	// The token's expiration date was in the past.
	ErrExpired = errors.New("token has expired")

	// The token was not signed correctly.
	ErrInvalid = errors.New("token is invalid")

	// TODO: Move to pluggable configuration, support multiple registed signing
	// keys.
	jwtSigningKey = []byte("In a world of prefab dreams, authenticity gleams.")

	// Only allow tokens created by prefab.
	jwtIssuer = "prefab"

	// TODO: Make pluggable with hostname for server.
	jwtAudience = "access"

	// TODO: Customize token expiry.
	tokenExpiration = time.Hour * 24

	// Allows for time to be stubbed in tests.
	timeFunc = time.Now
)

const (
	// Cookie name used for storing the prefab auth token.
	AuthTokenCookieName = "pfat"
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

// Login creates a signed JWT and attaches it to the outgoing GRPC metadata such
// that it will be propagated as a `Set-Cookie` HTTP header by the Gateway.
//
// Login should be called by an identity provider that has verified the user's
// identity.
func Login(ctx context.Context, identity Identity) error {
	return nil
}

// IssueToken creates a signed JWT for the given identity.
func IssueToken(ctx context.Context, identity Identity) (string, error) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Audience:  jwt.ClaimStrings{jwtAudience},
			ExpiresAt: jwt.NewNumericDate(timeFunc().Add(tokenExpiration)),
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

// ParseToken takes a signed JWT, validates it, and returns the identity
// information encoded within. Invalid and expired tokens will error.
func ParseToken(ctx context.Context, tokenString string) (Identity, error) {
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
		return Identity{}, err
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

// GetIdentity parses and verifies a JWT received from incoming GRPC metadata.
// An `Authorization` header will take precedence over a `Cookie`,
func GetIdentity(ctx context.Context) (Identity, error) {
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
		return ParseToken(ctx, a[0])
	}

	switch strings.ToLower(auth[0]) {
	case "bearer":
		// Standard bearer token.
		return ParseToken(ctx, auth[1])

	case "basic":
		// Basic auth is the method preferred for curl based CLI clients.
		// By convention, we expect the username to be the "Access Token", and for
		// there to be no password.
		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[1] != "" {
			return Identity{}, ErrInvalid
		}
		return ParseToken(ctx, pair[0])

	default:
		return Identity{}, ErrInvalid
	}
}

// TODO: Should we support multiple identities from cookies?
func identityFromCookie(ctx context.Context) (Identity, error) {
	cookies := CookiesFromIncomingContext(ctx)
	c, ok := cookies[AuthTokenCookieName]
	if !ok {
		return Identity{}, ErrNotFound
	}
	identity, err := ParseToken(ctx, c.Value)
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}

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
