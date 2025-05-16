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

	t, err := findToken(md)
	if err != nil {
		return Identity{}, err
	}

	if t == "" {
		return Identity{}, errors.Mark(ErrNotFound, 0)
	}

	// If it doesn't look like a JWT, we consider this to be not found since another identity
	// extractor may be able to handle it.
	parts := strings.Split(t, ".")
	if len(parts) != 3 {
		return Identity{}, errors.Mark(ErrNotFound, 0)
	}

	// If it looks like a JWT, we try to parse it and propagate the errors.
	return ParseIdentityToken(ctx, t)
}

func findToken(md metadata.MD) (string, error) {
	a, ok := md["authorization"] // GRPC Gateway forwards this header without prefix.

	if !ok || len(a) == 0 || a[0] == "" {
		return "", nil
	}

	auth := strings.SplitN(a[0], " ", 2)
	expectedParts := 2
	if len(auth) != expectedParts {
		// Relaxed fallback that allows tokens to be passed without the "bearer"
		// or "basic" prefix. Instead it takes the whole header.
		return a[0], nil
	}

	switch strings.ToLower(auth[0]) {
	case "bearer":
		// Standard bearer token.
		return auth[1], nil

	case "basic":
		// Basic auth is the method preferred for curl based CLI clients.
		// By convention, we expect the username to be the "Access Token", and for
		// there to be no password.
		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[1] != "" {
			return "", errors.Mark(ErrInvalidHeader, 0)
		}
		return pair[0], nil

	default:
		return "", errors.Mark(ErrInvalidHeader, 0)
	}
}
