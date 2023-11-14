package server

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
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
