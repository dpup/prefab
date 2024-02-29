// An example using email based magic-link auth.
//
// Edit config.yaml or set EMAIL_FROM, EMAIL_SMTP_USERNAME, and
// EMAIL_SMTP_PASSWORD in your environment
//
// $ go run examples/magiclinkauth/magiclinkauth.go
package main

import (
	"fmt"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/auth/magiclink"
	"github.com/dpup/prefab/email"
	"github.com/dpup/prefab/server"
	"github.com/dpup/prefab/templates"
)

func main() {
	server.LoadDefaultConfig()

	// Initialize the server with the auth, email, and magiclink plugins, this
	// should be enough to request a magic link and authenticate a client as that
	// email account. There is no application logic or persistance.
	s := server.New(
		server.WithPlugin(auth.Plugin()),
		server.WithPlugin(email.Plugin()),
		server.WithPlugin(templates.Plugin()),
		server.WithPlugin(magiclink.Plugin()),
		server.WithStaticFiles("/", "./examples/magiclinkauth/static/"),
	)

	fmt.Println("")
	fmt.Println("Request a magic link using:")
	fmt.Println(`curl -X POST -d '{"provider":"magiclink", "creds":{"email": "me@me.com"}}' 'http://localhost:8000/api/auth/login'`)
	fmt.Println("")
	fmt.Println("Or visit http://localhost:8000/ in your browser")
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}
