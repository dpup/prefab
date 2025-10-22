package prefab

import (
	"context"
	"net"
	"time"

	"github.com/dpup/prefab/internal/config"
	"github.com/dpup/prefab/serverutil"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"google.golang.org/grpc"
)

// Filename of the standard configuration file.
const ConfigFile = "prefab.yaml"

// ConfigKeyInfo contains metadata about a known configuration key.
// This is re-exported from internal/config for public API use.
type ConfigKeyInfo = config.ConfigKeyInfo

// ConfigInjector is a function that injects configuration into a context.
type ConfigInjector func(context.Context) context.Context

// Config is a global koanf instance used to access application level
// configuration options.
//
// Config is loaded in the following order (later sources override earlier):
// 1. Built-in defaults (in init())
// 2. Auto-discovered prefab.yaml (in init())
// 3. Environment variables with PF__ prefix (in init())
// 4. Additional sources loaded via LoadConfigFile() or LoadConfigDefaults()
//
// Environment variable transformation:
//   - PF__SERVER__PORT → server.port
//   - PF__SERVER__INCOMING_HEADERS → server.incomingHeaders (underscores become camelCase)
//   - PF__FOO_BAR__BAZ → fooBar.baz
var Config = koanf.New(".")

const (
	defaultPort = "8000"
	defaultHost = "localhost"
)

func init() {
	// Register all core configuration keys with their defaults (loaded lazily).
	registerCoreConfigKeys()

	// Look for a prefab.yaml file in the current directory or any parent.
	if cfg := config.SearchForConfig(ConfigFile, "."); cfg != "" {
		if err := Config.Load(file.Provider(cfg), yaml.Parser()); err != nil {
			panic("error loading config: " + err.Error())
		}
	}

	// Load environment variables with the prefix PF__.
	if err := Config.Load(env.Provider("PF__", ".", config.TransformEnv), nil); err != nil {
		panic("error loading env config: " + err.Error())
	}
}

// RegisterConfigKey registers a known configuration key with metadata.
// This should be called by core code and plugins to document expected config keys.
//
// Example:
//
//	prefab.RegisterConfigKey(prefab.ConfigKeyInfo{
//	    Key:         "auth.signingKey",
//	    Description: "JWT signing key for identity tokens",
//	    Type:        "string",
//	})
func RegisterConfigKey(info ConfigKeyInfo) {
	config.RegisterConfigKey(info)
}

// RegisterConfigKeys registers multiple configuration keys at once.
func RegisterConfigKeys(infos ...ConfigKeyInfo) {
	config.RegisterConfigKeys(infos...)
}

// RegisterDeprecatedKey registers a deprecated configuration key and its replacement.
//
// Example:
//
//	prefab.RegisterDeprecatedKey("server.security.corsAllowMethods", "server.security.corsAllowedMethods")
func RegisterDeprecatedKey(oldKey, newKey string) {
	config.RegisterDeprecatedKey(oldKey, newKey)
}

// LoadConfigFile loads additional configuration from a YAML file into the
// global Config instance. Call this before creating the server to load
// application-specific configuration.
//
// Example:
//
//	prefab.LoadConfigFile("./app.yaml")
//	s := prefab.New()
//	value := prefab.ConfigString("myapp.setting")
func LoadConfigFile(path string) {
	if err := Config.Load(file.Provider(path), yaml.Parser()); err != nil {
		panic("error loading config file '" + path + "': " + err.Error())
	}
}

// LoadConfigDefaults loads default configuration values into the global
// Config instance. Call this before creating the server to provide
// application-specific defaults that can be overridden by files or env vars.
//
// Example:
//
//	prefab.LoadConfigDefaults(map[string]interface{}{
//	    "myapp.cacheRefreshInterval": "5m",
//	    "myapp.maxRetries": 3,
//	})
//	s := prefab.New()
func LoadConfigDefaults(defaults map[string]interface{}) {
	if err := Config.Load(confmap.Provider(defaults, "."), nil); err != nil {
		panic("error loading config defaults: " + err.Error())
	}
}

// Configuration Access Functions
//
// These functions provide a clean API for accessing configuration values.
// They delegate to the underlying Config instance.

// ConfigString returns the string value for the given key.
func ConfigString(key string) string {
	return Config.String(key)
}

// ConfigInt returns the int value for the given key.
func ConfigInt(key string) int {
	return Config.Int(key)
}

// ConfigInt64 returns the int64 value for the given key.
func ConfigInt64(key string) int64 {
	return Config.Int64(key)
}

// ConfigFloat64 returns the float64 value for the given key.
func ConfigFloat64(key string) float64 {
	return Config.Float64(key)
}

// ConfigBool returns the bool value for the given key.
func ConfigBool(key string) bool {
	return Config.Bool(key)
}

// ConfigDuration returns the duration value for the given key.
// Duration strings like "5m", "1h", "30s" are parsed automatically.
func ConfigDuration(key string) time.Duration {
	return Config.Duration(key)
}

// ConfigMustDuration returns the duration value for the given key.
// It panics if the key doesn't exist or the value cannot be parsed.
func ConfigMustDuration(key string) time.Duration {
	return Config.MustDuration(key)
}

// ConfigStrings returns the string slice value for the given key.
func ConfigStrings(key string) []string {
	return Config.Strings(key)
}

// ConfigBytes returns the byte slice value for the given key.
func ConfigBytes(key string) []byte {
	return Config.Bytes(key)
}

// ConfigStringMap returns the string map value for the given key.
func ConfigStringMap(key string) map[string]string {
	return Config.StringMap(key)
}

// ConfigExists checks if the given key exists in the configuration.
func ConfigExists(key string) bool {
	return Config.Exists(key)
}

// ConfigAll returns all configuration as a map.
func ConfigAll() map[string]interface{} {
	return Config.All()
}

// Injects request scoped configuration from plugins.
func configInterceptor(injectors []ConfigInjector) func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		return handler(injectConfigs(ctx, injectors), req)
	}
}

//nolint:fatcontext // Lint complains about using context in a loop.
func injectConfigs(ctx context.Context, injectors []ConfigInjector) context.Context {
	ctx = serverutil.WithAddress(ctx, Config.String("address"))
	for _, injector := range injectors {
		ctx = injector(ctx)
	}
	return ctx
}

// registerCoreConfigKeys registers all core Prefab configuration keys with their defaults.
// This is called from init() before any config loading happens.
func registerCoreConfigKeys() {
	registerServerAndTLSConfigKeys()
	registerSecurityConfigKeys()
}

// registerServerAndTLSConfigKeys registers general server and TLS configuration keys.
func registerServerAndTLSConfigKeys() {
	config.RegisterConfigKeys(
		// General server configuration
		ConfigKeyInfo{
			Key:         "name",
			Description: "User-facing name that identifies the service",
			Type:        "string",
			Default:     "Prefab Server",
		},
		ConfigKeyInfo{
			Key:         "address",
			Description: "External address for the service (used in URL construction)",
			Type:        "string",
			Default:     "http://" + net.JoinHostPort(defaultHost, defaultPort),
		},

		// Server configuration
		ConfigKeyInfo{
			Key:         "server.host",
			Description: "Host to bind the server to",
			Type:        "string",
			Default:     defaultHost,
		},
		ConfigKeyInfo{
			Key:         "server.port",
			Description: "Port to bind the server to",
			Type:        "int",
			Default:     defaultPort,
		},
		ConfigKeyInfo{
			Key:         "server.incomingHeaders",
			Description: "Safe-list of headers to forward to gRPC services",
			Type:        "[]string",
		},
		ConfigKeyInfo{
			Key:         "server.maxMsgSizeBytes",
			Description: "Maximum gRPC message size in bytes",
			Type:        "int",
		},
		ConfigKeyInfo{
			Key:         "server.csrfSigningKey",
			Description: "Key used to sign CSRF tokens",
			Type:        "string",
		},

		// TLS configuration
		ConfigKeyInfo{
			Key:         "server.tls.certFile",
			Description: "Path to TLS certificate file",
			Type:        "string",
		},
		ConfigKeyInfo{
			Key:         "server.tls.keyFile",
			Description: "Path to TLS key file",
			Type:        "string",
		},
	)
}

// registerSecurityConfigKeys registers security headers and CORS configuration keys.
func registerSecurityConfigKeys() {
	config.RegisterConfigKeys(
		// Security headers configuration
		ConfigKeyInfo{
			Key:         "server.security.xFramesOptions",
			Description: "X-Frame-Options header value",
			Type:        "string",
		},
		ConfigKeyInfo{
			Key:         "server.security.hstsExpiration",
			Description: "HSTS max-age duration",
			Type:        "duration",
		},
		ConfigKeyInfo{
			Key:         "server.security.hstsIncludeSubdomains",
			Description: "Include subdomains in HSTS",
			Type:        "bool",
		},
		ConfigKeyInfo{
			Key:         "server.security.hstsPreload",
			Description: "Enable HSTS preload",
			Type:        "bool",
		},

		// CORS configuration
		ConfigKeyInfo{
			Key:         "server.security.corsOrigins",
			Description: "Allowed CORS origins",
			Type:        "[]string",
		},
		ConfigKeyInfo{
			Key:         "server.security.corsAllowMethods",
			Description: "Allowed CORS methods",
			Type:        "[]string",
		},
		ConfigKeyInfo{
			Key:         "server.security.corsAllowHeaders",
			Description: "Allowed CORS headers",
			Type:        "[]string",
		},
		ConfigKeyInfo{
			Key:         "server.security.corsExposeHeaders",
			Description: "CORS headers to expose to the browser",
			Type:        "[]string",
		},
		ConfigKeyInfo{
			Key:         "server.security.corsAllowCredentials",
			Description: "Allow credentials in CORS requests",
			Type:        "bool",
		},
		ConfigKeyInfo{
			Key:         "server.security.corsMaxAge",
			Description: "CORS preflight cache duration",
			Type:        "duration",
		},
	)
}
