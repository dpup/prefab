// Package fake provides an authentication plugin for testing purposes.
//
// This plugin allows server integrations tests to easily authenticate as any identity
// without requiring actual authentication credentials or external dependencies.
package fakeauth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/eventbus"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
)

const (
	// PluginName is the name of the fake auth plugin.
	PluginName = "auth_fake"

	// ProviderName is the name of the fake auth provider.
	ProviderName = "fakeauth"

	// Default identity values if not provided.
	defaultSubject = "fake-user-123"
	defaultEmail   = "fake-user@example.com"
	defaultName    = "Fake User"
)

// FakeAuthOption allows configuration of the FakeAuthPlugin.
type FakeAuthOption func(*FakeAuthPlugin)

// WithIdentityValidator allows setting a custom validator for login requests.
// This can be used to restrict which identities can be created.
func WithIdentityValidator(validator IdentityValidator) FakeAuthOption {
	return func(p *FakeAuthPlugin) {
		p.validator = validator
	}
}

// WithDefaultIdentity sets the default identity to use when no credentials are provided.
func WithDefaultIdentity(id auth.Identity) FakeAuthOption {
	return func(p *FakeAuthPlugin) {
		p.defaultIdentity = id
	}
}

// IdentityValidator validates fake login credentials.
// Return an error to reject the login.
type IdentityValidator func(ctx context.Context, creds map[string]string) error

// Plugin returns a new FakeAuthPlugin for testing purposes.
func Plugin(opts ...FakeAuthOption) *FakeAuthPlugin {
	p := &FakeAuthPlugin{
		defaultIdentity: auth.Identity{
			Provider:      ProviderName,
			Subject:       defaultSubject,
			Email:         defaultEmail,
			EmailVerified: true,
			Name:          defaultName,
		},
		validator: func(ctx context.Context, creds map[string]string) error {
			return nil // Accept all by default
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// FakeAuthPlugin provides fake authentication for testing purposes.
// It allows creating arbitrary identities without real credentials.
type FakeAuthPlugin struct {
	defaultIdentity auth.Identity
	validator       IdentityValidator
}

// From prefab.Plugin.
func (p *FakeAuthPlugin) Name() string {
	return PluginName
}

// From prefab.DependentPlugin.
func (p *FakeAuthPlugin) Deps() []string {
	return []string{auth.PluginName}
}

// From prefab.InitializablePlugin.
func (p *FakeAuthPlugin) Init(ctx context.Context, r *prefab.Registry) error {
	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)
	ap.AddLoginHandler(ProviderName, p.handleLogin)
	return nil
}

// Handle login requests by creating a fake identity.
func (p *FakeAuthPlugin) handleLogin(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Provider != ProviderName {
		return nil, errors.NewC("fake login handler called for wrong provider", codes.InvalidArgument)
	}

	// Use default identity as the base
	id := p.defaultIdentity

	// Generate a unique session ID
	id.SessionID = uuid.New().String()
	id.AuthTime = time.Now()

	// Check if we should simulate an error
	if errorCode, ok := req.Creds["error_code"]; ok {
		code := codes.Unknown
		if c, err := strconv.Atoi(errorCode); err == nil {
			code = codes.Code(c)
		}

		errorMsg := "simulated error"
		if msg, ok := req.Creds["error_message"]; ok && msg != "" {
			errorMsg = msg
		}

		return nil, errors.NewC(errorMsg, code)
	}

	// Override identity with any provided credentials
	if userID, ok := req.Creds["id"]; ok && userID != "" {
		id.Subject = userID
	} else if subject, ok := req.Creds["subject"]; ok && subject != "" {
		// For backward compatibility
		id.Subject = subject
	}

	if email, ok := req.Creds["email"]; ok && email != "" {
		id.Email = email
	}
	if name, ok := req.Creds["name"]; ok && name != "" {
		id.Name = name
	}
	if emailVerified, ok := req.Creds["email_verified"]; ok {
		id.EmailVerified = emailVerified == "true"
	}

	// Validate the identity (could be used to enforce test restrictions)
	if err := p.validator(ctx, req.Creds); err != nil {
		return nil, err
	}

	// Create a token for the identity
	token, err := auth.IdentityToken(ctx, id)
	if err != nil {
		return nil, err
	}

	// Publish login event if event bus is available
	if bus := eventbus.FromContext(ctx); bus != nil {
		bus.Publish(auth.LoginEvent, auth.AuthEvent{Identity: id})
	}

	// Return token directly or set a cookie based on request
	if req.IssueToken {
		return &auth.LoginResponse{
			Issued: true,
			Token:  token,
		}, nil
	}

	if err := auth.SendIdentityCookie(ctx, token); err != nil {
		return nil, err
	}

	return &auth.LoginResponse{
		Issued:      true,
		RedirectUri: req.RedirectUri,
	}, nil
}

// FakeOptions provides a strongly-typed structure for configuring fake auth login.
// These options are converted to a credentials map when making login requests.
type FakeOptions struct {
	// User identity fields
	ID            string // Takes precedence over Subject
	Subject       string // Deprecated: Use ID instead
	Email         string
	Name          string
	EmailVerified *bool

	// Error simulation
	ErrorCode    codes.Code // If set, simulates an error with this code
	ErrorMessage string     // Custom error message (defaults to "simulated error")
}

// toCredsMap converts FakeOptions to a map[string]string for inclusion in LoginRequest
func (o FakeOptions) toCredsMap() map[string]string {
	creds := make(map[string]string)

	// Add identity fields
	if o.ID != "" {
		creds["id"] = o.ID
	} else if o.Subject != "" {
		creds["subject"] = o.Subject
	}

	if o.Email != "" {
		creds["email"] = o.Email
	}

	if o.Name != "" {
		creds["name"] = o.Name
	}

	if o.EmailVerified != nil {
		if *o.EmailVerified {
			creds["email_verified"] = "true"
		} else {
			creds["email_verified"] = "false"
		}
	}

	// Add error simulation fields if provided
	if o.ErrorCode != 0 {
		creds["error_code"] = strconv.Itoa(int(o.ErrorCode))
		if o.ErrorMessage != "" {
			creds["error_message"] = o.ErrorMessage
		}
	}

	return creds
}

// MustLogin is a convenience function for tests that will panic if login fails.
// This allows for concise test setup.
func MustLogin(ctx context.Context, authClient auth.AuthServiceClient, options FakeOptions) string {
	resp, err := authClient.Login(ctx, &auth.LoginRequest{
		Provider:   ProviderName,
		Creds:      options.toCredsMap(),
		IssueToken: true,
	})
	if err != nil {
		panic(fmt.Sprintf("fake auth login failed: %v", err))
	}
	return resp.Token
}
