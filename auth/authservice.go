package auth

import (
	"context"

	"github.com/dpup/prefab/logging"
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
	logging.Info(ctx, "🔑  Login attempt")

	return &LoginResponse{
		Issued: false,
	}, nil
}
