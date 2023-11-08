package main

import (
	"fmt"
	"strings"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/auth/magiclink"
	"github.com/dpup/prefab/email"
	"github.com/dpup/prefab/server"
	"github.com/dpup/prefab/templates"
	"github.com/spf13/viper"
)

func main() {
	// TODO: Consider centralizing this, maybe in server.
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	// Initialize the server with the auth, email, and magiclink plugins, this
	// should be enough to request a magic link and authenticate a client as that
	// email account. There is no application logic or persistance.
	s := server.New(
		server.WithPlugin(auth.Plugin()),
		server.WithPlugin(email.Plugin()),
		server.WithPlugin(templates.Plugin()),
		server.WithPlugin(magiclink.Plugin()),
	)

	// Guidance for people who don't read the example code.
	fmt.Println("")
	fmt.Println("Request a magic link using:")
	fmt.Println(`curl -X POST -d '{"provider":"magiclink", "creds":{"email": "me@me.com"}}' 'http://0.0.0.0:8000/v1/auth/login'`)
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}
