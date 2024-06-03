package auth

const (
	LoginEvent  = "auth.login"
	LogoutEvent = "auth.logout"
)

// AuthEvent is an event that is emitted when an authentication event occurs.
type AuthEvent struct {
	Identity Identity
}
