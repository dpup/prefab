// Package delegation provides an authentication plugin that allows for
// delegated authentication. Admin tools or internal requests can create signed
// tokens that can be used to authenticate as a specific user.
package delegation

import (
	"context"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"google.golang.org/grpc/codes"
)

const (
	// PluginName is the name of this plugin
	PluginName = "auth_delegation"

	// Constant nae used as the auth provider in API requests.
	ProviderName = "delegated"
)

// DelegationOptions allow configuration of the DelegationPlugin.
type DelegationOption func(*DelegationPlugin)

// WithSigningKey sets the key used to sign delegation tokens.
func WithSigningKey(key string) DelegationOption {
	return func(p *DelegationPlugin) {
		p.signingKey = key
	}
}

// Plugin for allowing delegated requests.
func Plugin(opts ...DelegationOption) *DelegationPlugin {
	p := &DelegationPlugin{
		PluginName: PluginName,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// DelegationPlugin is an authentication plugin that allows for delegated
// authentication.
type DelegationPlugin struct {
	PluginName string
	signingKey string
}

// From prefab.Plugin.
func (p *DelegationPlugin) Name() string {
	return p.PluginName
}

// From prefab.DependentPlugin.
func (p *DelegationPlugin) Deps() []string {
	return []string{auth.PluginName}
}

// From prefab.Plugin.
func (p *DelegationPlugin) Init(ctx context.Context, r *prefab.Registry) error {
	if p.signingKey == "" {
		return errors.NewC("delegation: signing key must be provided", codes.FailedPrecondition)
	}
	//	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)

	return nil
}
