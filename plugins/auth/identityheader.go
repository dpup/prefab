package auth

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/dpup/prefab/errors"
	"google.golang.org/grpc/metadata"
)

func identityFromAuthHeader(ctx context.Context) (Identity, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	a, ok := md["authorization"] // GRPC Gateway forwards this header without prefix.

	if !ok || len(a) == 0 || a[0] == "" {
		return Identity{}, errors.Mark(ErrNotFound, 0)
	}

	auth := strings.SplitN(a[0], " ", 2)
	expectedParts := 2
	if len(auth) != expectedParts {
		// Relaxed fallback that allows tokens to be passed without the "bearer"
		// or "basic" prefix. Instead it takes the whole header.
		return ParseIdentityToken(ctx, a[0])
	}

	switch strings.ToLower(auth[0]) {
	case "bearer":
		// Standard bearer token.
		return ParseIdentityToken(ctx, auth[1])

	case "basic":
		// Basic auth is the method preferred for curl based CLI clients.
		// By convention, we expect the username to be the "Access Token", and for
		// there to be no password.
		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[1] != "" {
			return Identity{}, errors.Mark(ErrInvalidHeader, 0)
		}
		return ParseIdentityToken(ctx, pair[0])

	default:
		return Identity{}, errors.Mark(ErrInvalidHeader, 0)
	}
}
