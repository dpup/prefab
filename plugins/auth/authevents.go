package auth

import "time"

const (
	LoginEvent  = "auth.login"
	LogoutEvent = "auth.logout"
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
