// An example using google auth.
//
// Edit config.yaml or set AUTH_GOOGLE_ID and AUTH_GOOGLE_SECRET in your
// environment.
//
// $ go run examples/googleauth/googleauth.go
package main

import (
	"fmt"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/auth/google"
	"github.com/dpup/prefab/storage/memorystore"
)

func main() {
	prefab.LoadDefaultConfig()

	s := prefab.New(
		prefab.WithPlugin(auth.Plugin(
			auth.WithBlocklist(auth.NewBlocklist(memorystore.New())), // Keep track of revoked tokens.
		)),
		prefab.WithPlugin(google.Plugin()),
		prefab.WithStaticFiles("/", "./examples/googleauth/static/"),
	)

	fmt.Println("")
	fmt.Println("Visit http://localhost:8000/ in your browser")
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}
