package auth

import (
	"github.com/dpup/prefab/server"
)

// Constant name for identifying the core auth plugin
const PluginName = "auth"

// Plugin returns a new AuthPlugin.
func Plugin() *AuthPlugin {
	return &AuthPlugin{
		authService: &impl{},
	}
}

// AuthPlugin exposes plugin interfaces that register and manage the AuthService
// and related functionality.
type AuthPlugin struct {
	authService *impl
}

// From plugin.Plugin
func (ap *AuthPlugin) Name() string {
	return PluginName
}

// From server.OptionProvider
func (ap *AuthPlugin) ServerOptions() []server.ServerOption {
	return []server.ServerOption{
		server.WithGRPCService(&AuthService_ServiceDesc, ap.authService),
		server.WithGRPCGateway(RegisterAuthServiceHandlerFromEndpoint),
	}
}

// AddLoginHandler can be called by other plugins to register login handlers.
func (ap *AuthPlugin) AddLoginHandler(provider string, h LoginHandler) {
	ap.authService.AddLoginHandler(provider, h)
}
