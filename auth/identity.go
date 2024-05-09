package auth

import "time"

type Identity struct {

	// Unique identifier for the session that authenticated the identity. Maps to
	// the `jti` JWT claim.
	SessionID string

	// The time at which the identity was authenticated. Maps to `auth_time` JWT
	// claim. May differ from IssuedAt if a token is refreshed.
	AuthTime time.Time

	// Identity provider specific identifier. Maps to `sub` JWT claim.
	Subject string

	// name of the identity provider used to authenticate the user. Maps to
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
