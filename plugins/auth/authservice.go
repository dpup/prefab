package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/eventbus"
	"github.com/dpup/prefab/serverutil"
	"google.golang.org/grpc/codes"
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
		return nil, errors.NewC("auth: `issue_token` not compatible with `redirect_uri`", codes.InvalidArgument)
	}

	// TODO: Verify redirect_uri is a path or has a valid host.

	if h, ok := s.handlers[in.Provider]; ok {
		resp, err := h(ctx, in)

		// TODO: If the handler returns an error we may still want to send to the
		// redirect_uri with an error message, so the user doesn't end on a raw JSON
		// response.

		if resp != nil && resp.RedirectUri != "" {
			// Send a 302 redirect.
			logging.Infow(ctx, "Sending redirect", "redirectUri", resp.RedirectUri)
			if e := serverutil.SendStatusCode(ctx, http.StatusFound); e != nil {
				logging.Errorw(ctx, "auth: failed to send status code", "error", e)
			}
			if e := serverutil.SendHeader(ctx, "location", resp.RedirectUri); e != nil {
				logging.Errorw(ctx, "auth: failed to send header", "error", e)
			}
		}

		return resp, err
	}

	return nil, errors.NewC("auth: unknown or unregistered provider", codes.InvalidArgument)
}

func (s *impl) Logout(ctx context.Context, in *LogoutRequest) (*LogoutResponse, error) {
	id, err := identityFromCookie(ctx)
	if err != nil {
		// TODO: Should double logout be idempotent?
		return nil, err
	}

	// If enabled, block this token from future use.
	if err := MaybeBlock(ctx, id.SessionID); err != nil {
		logging.Errorw(ctx, "auth: failed to block tokenfor logout", "error", err)
	}

	address := serverutil.AddressFromContext(ctx)
	isSecure := strings.HasPrefix(address, "https")

	// Try to clear the cookie.
	if err := serverutil.SendCookie(ctx, &http.Cookie{
		Name:     IdentityTokenCookieName,
		Value:    "[invalidated]",
		Path:     "/",
		Secure:   isSecure,
		HttpOnly: true,
		Expires:  time.Now().Add(-24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	}); err != nil {
		return nil, err
	}

	r := in.RedirectUri
	if r == "" {
		r = address
	}

	if bus := eventbus.FromContext(ctx); bus != nil {
		bus.Publish(LogoutEvent, NewAuthEvent(id))
	}

	// For gateway requests, send the HTTP headers.
	serverutil.SendStatusCode(ctx, http.StatusFound)
	serverutil.SendHeader(ctx, "location", r)
	logging.Infow(ctx, "Sending logout redirect", "redirectUri", r)

	return &LogoutResponse{
		RedirectUri: r,
	}, nil
}

func (s *impl) Identity(ctx context.Context, in *IdentityRequest) (*IdentityResponse, error) {
	i, err := IdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return &IdentityResponse{
		Provider:      i.Provider,
		Subject:       i.Subject,
		Email:         i.Email,
		EmailVerified: i.EmailVerified,
		Name:          i.Name,
	}, nil
}
