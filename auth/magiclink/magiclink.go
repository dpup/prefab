// Package magiclink provides and authentication service pluging that allows
// users to authenticate using a magic link that is sent to their email address.
package magiclink

import (
	"context"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/email"
	"github.com/dpup/prefab/plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
Plan

- Login endpoint:
  - takes email
	- generates signed JWT with email and expiration
	- sends email with the magic-link
- Login endpoint:
	- takes token from magic-link
	- extracts email from token and creates auth token
	- sets cookie or returns an auth token
  - if cookie is set, needs a redirect
    - 302 with cookie should work, but won't on localhost
    - could use a meta-redirect if dest is localhost or different domain

What's needed:
- AuthService plugin for handling basic login endpoint, required by magiclink plugin
		- Login RPC
		- LoginRequest has key/value pairs (proto3 doesn't support extend) OR has a
		  details field which accepts google.protobuf.Any
- EmailSender, could just use SMTP or could be an abstract interface.
- Template for the email


Questions:
- should cookies only be set if there's a valid user?
- if so, what is the underlying source of user data?
- can it just be a pluggin?
- can this just say: this user has verified they have access to this email? And
  then ACL code should verify that the authenticated user is authorized to
  access the resource.
- can a browser have multiple authenticated identities
*/

const (
	// Constant name for the Magic Link auth plugin.
	PluginName = "auth_magiclink"

	// Constant name used as the auth provider in API requests.
	ProviderName = "magiclink"
)

func Plugin() *MagicLinkPlugin {
	return &MagicLinkPlugin{}
}

type MagicLinkPlugin struct {
	emailer *email.EmailPlugin
}

// From plugin.Plugin
func (p *MagicLinkPlugin) Name() string {
	return PluginName
}

// From plugin.DependentPlugin
func (p *MagicLinkPlugin) Deps() []string {
	return []string{auth.PluginName, email.PluginName}
}

// From plugin.InitializablePlugin.
func (p *MagicLinkPlugin) Init(ctx context.Context, r *plugin.Registry) error {
	p.emailer = r.Get(email.PluginName).(*email.EmailPlugin)
	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)
	ap.AddLoginHandler(ProviderName, p.handleLogin)
	return nil
}

// LoginHandler processes magiclink login requests.
func (p *MagicLinkPlugin) handleLogin(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Provider != ProviderName {
		return nil, status.Error(codes.InvalidArgument, "magiclink login handler called for wrong provider")
	}
	if req.Creds["token"] != "" {
		return p.handleToken(ctx, req.Creds["token"])
	}
	if req.Creds["email"] != "" {
		return p.handleEmail(ctx, req.Creds["email"])
	}
	return nil, status.Error(codes.InvalidArgument, "missing credentials, magiclink login requires an `email` or `token`")
}

func (p *MagicLinkPlugin) handleEmail(ctx context.Context, email string) (*auth.LoginResponse, error) {
	return nil, nil
}

func (p *MagicLinkPlugin) handleToken(ctx context.Context, token string) (*auth.LoginResponse, error) {
	return nil, nil
}
