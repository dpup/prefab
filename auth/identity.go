package auth

import "time"

type Identity struct {
	// Specifies the identity provider used. Maps to `iss` JWT claim.
	Issuer string

	// The time at which the identity was authenticated. Maps to `auth_time` JWT
	// claim.
	AuthTime time.Time

	// Identity provider specific identifier. Maps to `sub` JWT claim.
	Subject string

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
