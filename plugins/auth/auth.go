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
	"time"

	"github.com/dpup/prefab/errors"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
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
