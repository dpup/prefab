package main

import (
	"fmt"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/server"
)

func main() {
	// Initialize the server with the auth and magiclink plugins, this should be
	// enough to request a magic link and authenticate a client as that email
	// account. There is no application logic or persistance.
	s := server.New(
		server.WithPlugin(auth.Plugin()),
	)

	// Guidance for people who don't read the example code.
	fmt.Println("")
	fmt.Println("Request a magic link using:")
	fmt.Println(`curl -X POST -D '{"provider":"magiclink", "creds":{"email": "me@me.com"}}' 'http://0.0.0.0:8000/v1/auth'`)
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}
