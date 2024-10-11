package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/serverutil"
)

// Cookie name used for storing the prefab identity token.
const IdentityTokenCookieName = "pf-id"

// SendIdentityCookie attaches the token to the outgoing GRPC metadata such
// that it will be propagated as a `Set-Cookie` HTTP header by the Gateway.
func SendIdentityCookie(ctx context.Context, token string) error {
	address := serverutil.AddressFromContext(ctx)
	isSecure := strings.HasPrefix(address, "https")
	return serverutil.SendCookie(ctx, &http.Cookie{
		Name:     IdentityTokenCookieName,
		Value:    token,
		Path:     "/",
		Secure:   isSecure,
		HttpOnly: true,
		Expires:  time.Now().Add(expirationFromContext(ctx)),
		SameSite: http.SameSiteLaxMode,
	})
}

func identityFromCookie(ctx context.Context) (Identity, error) {
	cookies := serverutil.CookiesFromIncomingContext(ctx)
	c, ok := cookies[IdentityTokenCookieName]
	if !ok {
		return Identity{}, errors.Mark(ErrNotFound, 0)
	}
	identity, err := ParseIdentityToken(ctx, c.Value)
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}
