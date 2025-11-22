package auth

import "time"

const (
	LoginEvent      = "auth.login"
	LogoutEvent     = "auth.logout"
	DelegationEvent = "auth.delegation"
)

// AuthEvent is an event that is emitted when an authentication event occurs.
type AuthEvent struct {
	Identity  Identity
	Timestamp time.Time // When the event occurred
}

// NewAuthEvent creates an AuthEvent with the current timestamp.
func NewAuthEvent(identity Identity) AuthEvent {
	return AuthEvent{
		Identity:  identity,
		Timestamp: time.Now(),
	}
}

// DelegationEventData is emitted when an admin assumes another user's identity.
type DelegationEventData struct {
	// The admin user who is assuming the identity
	Admin Identity

	// The identity that was assumed (includes delegation metadata)
	AssumedIdentity Identity

	// Reason provided for the delegation
	Reason string
}
