package magiclink

import (
	"context"

	"github.com/dpup/prefab/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoginHandler processes magiclink login requests.
func LoginHandler(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Provider != ProviderName {
		return nil, status.Error(codes.InvalidArgument, "login handler called for wrong provider")
	}
	if req.Creds["token"] != "" {
		return handleToken(ctx, req.Creds["token"])
	}
	if req.Creds["email"] != "" {
		return handleEmail(ctx, req.Creds["email"])
	}
	return nil, status.Error(codes.InvalidArgument, "missing credentials, magiclink login requires an `email` or `token`")
}

func handleEmail(ctx context.Context, email string) (*auth.LoginResponse, error) {
	return nil, nil
}

func handleToken(ctx context.Context, token string) (*auth.LoginResponse, error) {
	return nil, nil
}
