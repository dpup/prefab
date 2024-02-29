// An example using google auth.
//
// Edit config.yaml or set AUTH_GOOGLE_ID and AUTH_GOOGLE_SECRET in your
// environment.
//
// $ go run examples/googleauth/googleauth.go
package main

import (
	"fmt"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/auth/google"
	"github.com/dpup/prefab/server"
)

func main() {
	server.LoadDefaultConfig()

	s := server.New(
		server.WithPlugin(auth.Plugin()),
		server.WithPlugin(google.Plugin()),
		server.WithStaticFiles("/", "./examples/googleauth/static/"),
	)

	fmt.Println("")
	fmt.Println("Visit http://localhost:8000/ in your browser")
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

// Google auth task list:
// TODO: Update login endpoint to accept id_token and decode it.
// TODO: Add google SDK to this example, and use it to trigger a client side login flow.
