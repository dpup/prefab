package auth

import (
	"context"
	"time"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/serverutil"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// Leeway for JWT expiration checks.
const jwtLeeway = 5 * time.Second

type Identity struct {

	// Unique identifier for the session that authenticated the identity. Maps to
	// the `jti` JWT claim.
	SessionID string

	// The time at which the identity was authenticated. Maps to `auth_time` JWT
	// claim. May differ from IssuedAt if a token is refreshed.
	AuthTime time.Time

	// Identity provider specific identifier. Maps to `sub` JWT claim.
	Subject string

	// Name of the identity provider used to authenticate the user. Maps to custom
	// `idp` JWT claim.
	Provider string

	// The email address received from the identity provider, if available. Maps
	// to `email` JWT claim.
	Email string

	// Whether the identity provider has verified the email address. Maps to
	// `email_verified` JWT claim.
	EmailVerified bool

	// Name received from the identity provider, if available. Maps to `name` JWT
	// claim.
	Name string
}

// IdentityExtractor is a function which returns a user identity from a given
// context. Providers should return ErrNotFound if no identity is found.
type IdentityExtractor func(ctx context.Context) (Identity, error)

type identityExtractorsKey struct{}

// WithIdentityExtractors attaches a list of identity providers to the context.
func WithIdentityExtractors(ctx context.Context, providers ...IdentityExtractor) context.Context {
	return context.WithValue(ctx, identityExtractorsKey{}, providers)
}

// IdentityFromContext parses and verifies a JWT received from the incoming
// request context (including GRPC metadata.) An `Authorization` header will
// take precedence over a `Cookie`, which in turn will take precedence over
// other identity extractors.
func IdentityFromContext(ctx context.Context) (Identity, error) {
	providers, ok := ctx.Value(identityExtractorsKey{}).([]IdentityExtractor)
	if !ok {
		return Identity{}, errors.New("auth: no identity extractors registered. See WithDefaultExtractorsForTest")
	}
	for _, provider := range providers {
		i, err := provider(ctx)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		return i, err
	}
	return Identity{}, ErrNotFound
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
		jwt.WithLeeway(jwtLeeway),
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

// WithIdentityForTest creates a new context with the given identity
// attached. This is useful for testing, where we want to simulate a request
// with a given identity.
func WithIdentityForTest(ctx context.Context, identity Identity) context.Context {
	ctx = WithIdentityExtractorsForTest(ctx)
	if identity == (Identity{}) {
		// Short-circuity to avoid serialization/deserialization of empty identity.
		return ctx
	}
	tokenString, _ := IdentityToken(ctx, identity)
	md := metadata.Pairs("authorization", tokenString)
	return metadata.NewIncomingContext(ctx, md)
}

// WithIdentityExtractorsForTest returns a context with the default identity
// extractors attached. This is useful for testing, where we want to simulate
// a request with a given identity.
func WithIdentityExtractorsForTest(ctx context.Context) context.Context {
	return WithIdentityExtractors(ctx,
		identityFromAuthHeader,
		identityFromCookie,
	)
}
