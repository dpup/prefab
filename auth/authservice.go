package auth

import (
	"context"

	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/server/serverutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoginHandler is a function which allows delegation of login requests.
type LoginHandler func(ctx context.Context, req *LoginRequest) (*LoginResponse, error)

func New() AuthServiceServer {
	return &impl{}
}

// Implements AuthServiceServer and Plugin interfaces.
type impl struct {
	UnimplementedAuthServiceServer
	handlers map[string]LoginHandler
}

func (s *impl) AddLoginHandler(provider string, h LoginHandler) {
	if s.handlers == nil {
		s.handlers = map[string]LoginHandler{}
	}
	s.handlers[provider] = h
}

func (s *impl) Login(ctx context.Context, in *LoginRequest) (*LoginResponse, error) {
	logging.Track(ctx, "auth.provider", in.Provider)
	logging.Track(ctx, "auth.issueToken", in.IssueToken)
	logging.Track(ctx, "auth.redirectUri", in.RedirectUri)
	logging.Info(ctx, "Login attempt")

	if in.RedirectUri != "" && in.IssueToken {
		return nil, status.Error(codes.InvalidArgument, "auth: `issue_token` not compatible with `redirect_uri`")
	}

	// TODO: Verify redirect_uri is a path or has a valid host.

	if h, ok := s.handlers[in.Provider]; ok {
		resp, err := h(ctx, in)

		if resp != nil && resp.RedirectUri != "" {
			// Send a 302 redirect.
			serverutil.SendStatusCode(ctx, 302)
			serverutil.SendHeader(ctx, "location", resp.RedirectUri)
			logging.Infow(ctx, "Sending redirect", "redirectUri", resp.RedirectUri)
		}

		return resp, err
	}

	return nil, status.Error(codes.InvalidArgument, "auth: unknown or unregistered provider")
}

func (s *impl) Identity(ctx context.Context, in *IdentityRequest) (*IdentityResponse, error) {
	i, err := IdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return &IdentityResponse{
		Subject:       i.Subject,
		Email:         i.Email,
		EmailVerified: i.EmailVerified,
		Name:          i.Name,
	}, nil
}
