package auth

import (
	"context"

	"github.com/dpup/prefab/logging"
)

func New() AuthServiceServer {
	return &impl{}
}

// Implements AuthServiceServer and Plugin interfaces.
type impl struct {
	UnimplementedAuthServiceServer
}

func (s *impl) Login(ctx context.Context, in *LoginRequest) (*LoginResponse, error) {
	logging.Info(ctx, "🔑  Login attempt")
	return &LoginResponse{
		Issued: false,
	}, nil
}
