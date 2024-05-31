package pwdauth

import (
	"context"
	"time"

	"github.com/dpup/prefab/plugins/auth"
	"github.com/google/uuid"
)

type AccountFinder interface {
	// FindAccount looks up a user by their email.
	FindAccount(ctx context.Context, email string) (*Account, error)
}

// Account contains minimal information needed by the pwdauth plugin to
// authenticate a user. The application should map it's own user model to this
// via the AccountFinder interface.
type Account struct {
	ID             string
	Email          string
	Name           string
	EmailVerified  bool
	HashedPassword []byte
}

func identityFromAccount(a *Account) auth.Identity {
	return auth.Identity{
		Provider:      ProviderName,
		SessionID:     uuid.NewString(),
		AuthTime:      time.Now(),
		Subject:       a.ID,
		Email:         a.Email,
		EmailVerified: a.EmailVerified,
		Name:          a.Name,
	}
}
