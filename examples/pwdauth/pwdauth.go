// An example using password auth.
//
// $ go run examples/pwdauth/pwdauth.go
package main

import (
	"context"
	"fmt"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/auth/pwdauth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	prefab.LoadDefaultConfig()

	s := prefab.New(
		prefab.WithPlugin(auth.Plugin()),
		prefab.WithPlugin(pwdauth.Plugin(
			pwdauth.WithAccountFinder(accountStore{}),
			pwdauth.WithHasher(pwdauth.TestHasher), // Doesn't hash passwords.
		)),
		prefab.WithStaticFiles("/", "./examples/pwdauth/static/"),
	)

	fmt.Println("")
	fmt.Println("Visit http://localhost:8000/ in your browser")
	fmt.Println("")
	fmt.Println("Then try logging in with one of these email addresses:")
	for _, acc := range testAccounts {
		fmt.Println("  ", acc.Email)
	}
	fmt.Println("")
	fmt.Println("All accounts have the password 'password'.")
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

type accountStore struct{}

func (a accountStore) FindAccount(ctx context.Context, email string) (*pwdauth.Account, error) {
	for _, acc := range testAccounts {
		if acc.Email == email {
			return acc, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "account not found")
}

var testAccounts = []*pwdauth.Account{
	{
		ID:             "1",
		Email:          "logan@example.com",
		Name:           "Logan",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
	{
		ID:             "2",
		Email:          "scott@example.com",
		Name:           "Scott Summers",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
	{
		ID:             "3",
		Email:          "jean@example.com",
		Name:           "Jean Grey",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
	{
		ID:             "4",
		Email:          "ororo@example.com",
		Name:           "Ororo Munroe",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
	{
		ID:             "5",
		Email:          "kurt@example.com",
		Name:           "Kurt Wagner",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
}
