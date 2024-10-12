// Package apikey provides an authentication plugin that allows for
// authentication via apikeys.
package apikey

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

const (
	// PluginName is the name of this plugin
	PluginName = "auth_apikey"

	// Constant nae used as the auth provider in API requests.
	ProviderName = "apikey"
)

// KeyOwner is used by the application to provide details about the owner of the
// API key.
type KeyOwner struct {
	UserID        string
	Email         string
	EmailVerified bool
	Name          string
	KeyCreatedAt  time.Time
}

// KeyFunc is a function that returns the owner of an API key. Should be
// implemented by the application.
type KeyFunc func(ctx context.Context, key string) (*KeyOwner, error)

// APIOptions allow configuration of the APIPlugin.
type APIOption func(*APIPlugin)

// WithKeyFunc sets the function used to fetch the owner of an API key.
func WithKeyFunc(f KeyFunc) APIOption {
	return func(p *APIPlugin) {
		p.keyOwnerFunc = f
	}
}

// WithKeyPrefix sets the prefix used to identify API keys.
func WithKeyPrefix(prefix string) APIOption {
	return func(p *APIPlugin) {
		p.keyPrefix = prefix
	}
}

// Plugin for allowing requests to be authorized by an API key.
func Plugin(opts ...APIOption) *APIPlugin {
	p := &APIPlugin{
		PluginName: PluginName,
		keyPrefix:  "pak",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// APIPlugin is an authentication plugin that allows for api-key based
// authentication.
type APIPlugin struct {
	PluginName   string
	keyOwnerFunc KeyFunc
	keyPrefix    string
}

// From prefab.Plugin.
func (p *APIPlugin) Name() string {
	return p.PluginName
}

// From prefab.DependentPlugin.
func (p *APIPlugin) Deps() []string {
	return []string{auth.PluginName}
}

// From prefab.Plugin.
func (p *APIPlugin) Init(ctx context.Context, r *prefab.Registry) error {
	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)
	ap.AddIdentityExtractor(p.fetchIdentity)
	return nil
}

// NewKey can be used by an application to create a new key for storage.
func (p *APIPlugin) NewKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return p.keyPrefix + "_" + hex.EncodeToString(b)
}

func (p *APIPlugin) fetchIdentity(ctx context.Context) (auth.Identity, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	a, ok := md["authorization"] // GRPC Gateway forwards this header without prefix.

	// No authorization header.
	if !ok || len(a) == 0 || a[0] == "" {
		return auth.Identity{}, errors.Mark(auth.ErrNotFound, 0)
	}

	// Ignore non-api keys.
	if !strings.HasPrefix(a[0], p.keyPrefix+"_") {
		return auth.Identity{}, errors.Mark(auth.ErrNotFound, 0)
	}

	// Verify the key and fetch the owner.
	key := strings.TrimPrefix(a[0], p.keyPrefix+"_")
	owner, err := p.keyOwnerFunc(ctx, key)
	if err != nil {
		return auth.Identity{}, errors.Wrap(err, 0).WithCode(codes.Unauthenticated)
	}

	// Return an identity that matches the key owner.
	return auth.Identity{
		Subject:       owner.UserID,
		Name:          owner.Name,
		Email:         owner.Email,
		EmailVerified: owner.EmailVerified,
		Provider:      ProviderName,
		AuthTime:      owner.KeyCreatedAt,
	}, nil
}
