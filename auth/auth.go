// Package auth provides utilities for authenticating requests. It can be used
// in conjunction with ACLs, but on its own should not be used for authorization
// of resources.
//
// At least one `identity` plugin should be registered.
package auth

// Get/Set Cookies
// Authorization header

// Should the auth-key be a JWT? Reference the provider that created it and
// avoid having to lookup the user identity?
// auth.Login(ctx, identity) --> Sets JWT with all the necessary info.

// identity, err := auth.Identity(ctx) --> Gets identity info out of JWT.
