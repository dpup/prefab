package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

const (
	// DelegationAction is the authz action required to assume identities.
	// When using the authz plugin, admins must be granted this action to use AssumeIdentity.
	DelegationAction = "auth.assume_identity"

	// DelegationResource is a synthetic resource type for delegation authorization.
	// Used with authz plugin to check if a user has permission to assume identities.
	DelegationResource = "auth:delegation"
)

// AdminChecker is a function that determines if an identity has admin privileges
// for identity delegation. This is used as a fallback when the authz plugin is
// not available, or can be provided as a custom implementation.
//
// When the authz plugin is available, an AdminChecker is automatically created
// that wraps the authz.Authorize method to check for the DelegationAction permission.
//
// Return true if the identity is an admin, false otherwise. Return an error if
// the check cannot be completed.
type AdminChecker func(ctx context.Context, identity Identity) (bool, error)

// Authorizer is an interface for authorization plugins that can verify permissions.
// This allows the auth plugin to use the authz plugin for delegation authorization
// without creating an import cycle.
//
// The authz plugin implements this interface, allowing the auth plugin to check
// if a user has permission to assume other identities.
type Authorizer interface {
	// Authorize checks if the current user (from context) has permission to perform
	// the specified action on the specified resource.
	//
	// The params argument should be an AuthorizeParams struct. It's declared as any
	// to avoid import cycles between auth and authz plugins.
	//
	// Returns nil if authorized, or an error with codes.PermissionDenied if denied.
	Authorize(ctx context.Context, params any) error
}

// AuthorizeParams mirrors authz.AuthorizeParams to avoid import cycle.
// This struct is passed to Authorizer.Authorize() for delegation permission checks.
//
// When checking delegation permissions, use:
//   - ObjectKey: DelegationResource ("auth:delegation")
//   - Action: DelegationAction ("auth.assume_identity")
//   - DefaultEffect: 0 (Deny - fail closed)
type AuthorizeParams struct {
	// ObjectKey identifies the type of resource (e.g., "auth:delegation")
	ObjectKey string

	// ObjectID is the specific resource instance, if applicable
	ObjectID any

	// Scope limits the authorization check to a specific scope
	Scope string

	// Action is the operation being performed (e.g., "auth.assume_identity")
	Action string

	// DefaultEffect specifies what to do if no policy matches (0=Deny, 1=Allow)
	DefaultEffect int

	// Info provides additional context for logging/debugging
	Info string
}

// IsDelegated returns true if the identity was assumed by an admin user.
func IsDelegated(identity Identity) bool {
	return identity.Delegation != nil
}

// GetDelegator returns the original admin's details when an identity has been
// delegated. Returns ok=false if the identity is not delegated.
func GetDelegator(identity Identity) (sub, provider, sessionID string, ok bool) {
	if identity.Delegation == nil {
		return "", "", "", false
	}
	return identity.Delegation.DelegatorSub,
		identity.Delegation.DelegatorProvider,
		identity.Delegation.DelegatorSessionId,
		true
}

// generateSessionID creates a unique session identifier for delegated identities.
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate session ID: " + err.Error())
	}
	return "delegated-" + hex.EncodeToString(b)
}
