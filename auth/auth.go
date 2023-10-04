// Package auth provides utilities for authenticating requests. It is intended
// to be used in conjunction with identity providers and ACLs. On its own, it
// should not be used for authorization of access to resources.
package auth

import (
	"context"
	"errors"
)

var (
	// No identity was found within the incoming context.
	ErrNotFound = errors.New("identity not found")
)

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
	return "", nil
}

// GetIdentity parses and verifies a JWT received from incoming GRPC metadata.
// An `Authorization` header will take precedence over a `Cookie`,
func GetIdentity(ctx context.Context) (Identity, error) {
	i, err := identityFromAuthHeader(ctx)
	if err != nil || err != ErrNotFound {
		return i, err
	}
	return identityFromCookie(ctx)
}

func identityFromAuthHeader(ctx context.Context) (Identity, error) {
	return Identity{}, nil
}

func identityFromCookie(ctx context.Context) (Identity, error) {
	return Identity{}, nil
}
