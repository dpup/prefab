package prefab

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

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
	// Provide fallbacks for cases when no configuration is loaded at all.
	// TODO: Allow plugins to extend the default configuration directly, so they
	// aren't all grouped here.
	if err := Config.Load(confmap.Provider(map[string]interface{}{
		"name":                  "Prefab Server",
		"address":               "http://" + net.JoinHostPort(defaultHost, defaultPort),
		"server.host":           defaultHost,
		"server.port":           defaultPort,
		"auth.expiration":       "24h",
		"upload.path":           "/upload",
		"upload.downloadPrefix": "/download",
		"upload.maxFiles":       10,
		"upload.maxMemory":      4 << 20,
		"upload.validTypes":     []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
	}, "."), nil); err != nil {
		panic("error loading default config: " + err.Error())
	}

	// Look for a prefab.yaml file in the current directory or any parent.
	if cfg := searchForConfig(ConfigFile, "."); cfg != "" {
		if err := Config.Load(file.Provider(cfg), yaml.Parser()); err != nil {
			panic("error loading config: " + err.Error())
		}
	}

	// Load environment variables with the prefix PF__.
	if err := Config.Load(env.Provider("PF__", ".", transformEnv), nil); err != nil {
		panic("error loading env config: " + err.Error())
	}
}

// Injects request scoped configuration from plugins.
func configInterceptor(injectors []ConfigInjector) func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		return handler(injectConfigs(ctx, injectors), req)
	}
}

// ConfigInjector is a function that injects configuration into a context.
type ConfigInjector func(context.Context) context.Context

//nolint:fatcontext // Lint complains about using context in a loop.
func injectConfigs(ctx context.Context, injectors []ConfigInjector) context.Context {
	ctx = serverutil.WithAddress(ctx, Config.String("address"))
	for _, injector := range injectors {
		ctx = injector(ctx)
	}
	return ctx
}

func searchForConfig(filename string, startDir string) string {
	d, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}

	p := filepath.Join(d, filename)
	if _, err = os.Stat(p); err == nil {
		return p
	}

	parentDir := filepath.Dir(d)
	if parentDir == d {
		return ""
	}
	return searchForConfig(filename, parentDir)
}

// Converts PF__SERVER__INCOMING_HEADERS to server.incomingHeaders.
func transformEnv(s string) string {
	s = strings.ToLower(strings.TrimPrefix(s, "PF__"))
	segments := strings.Split(s, "__")
	for i, segment := range segments {
		parts := strings.Split(segment, "_")
		for j := 1; j < len(parts); j++ {
			parts[j] = capitalize(parts[j])
		}
		segments[i] = strings.Join(parts, "")
	}

	return strings.Join(segments, ".")
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
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
