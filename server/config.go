package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/dpup/prefab/server/serverutil"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// LoadDefaultConfig reads the default config file, config.yaml, from the
// current working directory, and and sets up environment variable overrides.
func LoadDefaultConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
}

// Injects request scoped configuration from plugins.
func configInterceptor(injectors []ConfigInjector) func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		ctx = serverutil.WithAddress(ctx, viper.GetString("address"))
		for _, injector := range injectors {
			ctx = injector(ctx)
		}
		return handler(ctx, req)
	}
}

// ConfigInjector is a function that injects configuration into a context.
type ConfigInjector func(context.Context) context.Context

func configString(key, d string) string {
	if v := viper.GetString(key); v != "" {
		return v
	}
	return d
}

func configInt(key string, d int) int {
	if v := viper.GetInt(key); v != 0 {
		return v
	}
	return d
}
