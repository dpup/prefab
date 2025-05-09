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
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/auth/google"
	"github.com/dpup/prefab/plugins/storage"
	"github.com/dpup/prefab/plugins/storage/sqlite"
)

func main() {
	s := prefab.New(
		prefab.WithPlugin(auth.Plugin()),
		prefab.WithPlugin(google.Plugin()),
		// Register an SQLite store to persist revoked tokens.
		prefab.WithPlugin(storage.Plugin(sqlite.New("example_googleauth.s3db"))),
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
