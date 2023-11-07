package auth

import (
	"github.com/dpup/prefab/server"
)

// Plugin returns a new AuthPlugin.
func Plugin() *AuthPlugin {
	return &AuthPlugin{}
}

// AuthPlugin exposes plugin interfaces that register and manage the AuthService
// and related functionality.
type AuthPlugin struct{}

// From plugin.Plugin
func (ap *AuthPlugin) Name() string {
	return "auth_service"
}

// From server.OptionProvider
func (ap *AuthPlugin) ServerOptions() []server.ServerOption {
	return []server.ServerOption{
		server.WithGRPCService(&AuthService_ServiceDesc, &impl{}),
		server.WithGRPCGateway(RegisterAuthServiceHandlerFromEndpoint),
	}
}
