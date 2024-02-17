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

func configInjector(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	// TODO: For now the address is considered global, but could be request
	// specific in the future. I also don't love these stringly typed configs.
	ctx = serverutil.WithAddress(ctx, viper.GetString("address"))
	return handler(ctx, req)
}
