// An example of how to use the ACL plugin.
//
// $ go run examples/acls/acls.go
package main

import (
	"fmt"

	"github.com/dpup/prefab/acl"
	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/server"
)

func main() {
	server.LoadDefaultConfig()

	s := server.New(
		server.WithPlugin(auth.Plugin()),
		server.WithPlugin(acl.Plugin()),
	)

	fmt.Println("")
	fmt.Println("Visit http://localhost:8000/ in your browser")
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}
