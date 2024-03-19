// Package pwdauth provides an authentication service plugin that allows users
// to authenticate via a email and password.
package pwdauth

import (
	"context"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
Possible additional features:
- Facilitate updating a user's password
- Password policy enforcement
- Forgotton password flow
- Password storage separate from user storage
*/

const (
	PluginName   = "auth_pwdauth"
	ProviderName = "password"
)

// PwdAuthOption allows configuration of the PwdAuthPlugin.
type PwdAuthOption func(*PwdAuthPlugin)

// WithHasher overrides the default hasher used by the PwdAuthPlugin.
func WithHasher(h Hasher) PwdAuthOption {
	return func(p *PwdAuthPlugin) {
		p.hasher = h
	}
}

// WithAccountFinder tells the pwdauth plugin how to find user accounts.
func WithAccountFinder(f AccountFinder) PwdAuthOption {
	return func(p *PwdAuthPlugin) {
		p.accountFinder = f
	}
}

// Plugin for handling password based authentication.
func Plugin(opts ...PwdAuthOption) *PwdAuthPlugin {
	p := &PwdAuthPlugin{
		hasher: DefaultHasher,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type PwdAuthPlugin struct {
	hasher        Hasher
	accountFinder AccountFinder
}

// From plugin.Plugin
func (p *PwdAuthPlugin) Name() string {
	return PluginName
}

// From plugin.DependentPlugin
func (p *PwdAuthPlugin) Deps() []string {
	return []string{auth.PluginName}
}

// From plugin.InitializablePlugin.
func (p *PwdAuthPlugin) Init(ctx context.Context, r *plugin.Registry) error {
	if p.accountFinder == nil {
		return status.Error(codes.FailedPrecondition, "pwdauth: plugin requires an account finder")
	}
	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)
	ap.AddLoginHandler(ProviderName, p.handleLogin)
	return nil
}

func (p *PwdAuthPlugin) handleLogin(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Provider != ProviderName {
		return nil, status.Error(codes.InvalidArgument, "pwdauth login handler called for wrong provider")
	}
	if req.Creds["email"] == "" || req.Creds["password"] == "" {
		return nil, status.Error(codes.InvalidArgument, "missing credentials, pwdauth login requires an `email` and `password`")
	}

	a, err := p.accountFinder.FindAccount(ctx, req.Creds["email"])
	if status.Code(err) == codes.NotFound {
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	} else if err != nil {
		return nil, err
	}

	if err := p.hasher.Compare(a.HashedPassword, []byte(req.Creds["password"])); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}

	idt, err := auth.IdentityToken(ctx, identityFromAccount(a))
	if err != nil {
		return nil, err
	}

	if req.IssueToken {
		return &auth.LoginResponse{
			Issued: true,
			Token:  idt,
		}, nil
	}

	if err := auth.SendIdentityCookie(ctx, idt); err != nil {
		return nil, err
	}

	return &auth.LoginResponse{
		Issued:      true,
		RedirectUri: req.RedirectUri,
	}, nil
}
