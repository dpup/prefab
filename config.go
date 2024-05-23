package prefab

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strings"
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
var Config = koanf.New(".")

const (
	defaultPort = "8000"
	defaultHost = "localhost"
	keyLength   = 32
)

func init() {
	// Provide fallbacks for cases when no configuration is loaded at all.
	if err := Config.Load(confmap.Provider(map[string]interface{}{
		"name":            "Prefab Server",
		"address":         "http://" + net.JoinHostPort(defaultHost, defaultPort),
		"server.host":     defaultHost,
		"server.port":     defaultPort,
		"auth.expiration": "24h",
		"auth.signingKey": randomString(keyLength), // Tokens will break with each restart.
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
//
//nolint:fatcontext // Lint complains about using context in a loop.
func configInterceptor(injectors []ConfigInjector) func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		ctx = serverutil.WithAddress(ctx, Config.String("address"))
		for _, injector := range injectors {
			ctx = injector(ctx)
		}
		return handler(ctx, req)
	}
}

// ConfigInjector is a function that injects configuration into a context.
type ConfigInjector func(context.Context) context.Context

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

func randomString(keySize int) string {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		panic(err) // Generation failed
	}
	return hex.EncodeToString(key)
}
