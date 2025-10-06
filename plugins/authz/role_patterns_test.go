package authz_test

import (
	"context"
	"testing"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/authz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

// Test types
type testDoc struct {
	id      string
	ownerID string
	orgID   string
	public  bool
}

func (d testDoc) AuthzType() string { return "document" }
func (d testDoc) OwnerID() string   { return d.ownerID }
func (d testDoc) ScopeID() string   { return d.orgID }

func TestConditionalRole(t *testing.T) {
	const customRole = authz.Role("custom")
	identity := auth.Identity{Subject: "user123", Email: "test@example.com"}
	doc := testDoc{id: "1", ownerID: "user123", orgID: "org1"}

	t.Run("grants role when predicate returns true", func(t *testing.T) {
		describer := authz.ConditionalRole(customRole,
			func(ctx context.Context, subject auth.Identity, obj testDoc, scope authz.Scope) (bool, error) {
				return obj.ownerID == subject.Subject, nil
			},
		)

		roles, err := describer(t.Context(), identity, doc, "org1")
		require.NoError(t, err)
		assert.Equal(t, []authz.Role{customRole}, roles)
	})

	t.Run("does not grant role when predicate returns false", func(t *testing.T) {
		describer := authz.ConditionalRole(customRole,
			func(ctx context.Context, subject auth.Identity, obj testDoc, scope authz.Scope) (bool, error) {
				return obj.ownerID != subject.Subject, nil
			},
		)

		roles, err := describer(t.Context(), identity, doc, "org1")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("propagates errors from predicate", func(t *testing.T) {
		expectedErr := errors.NewC("test error", codes.Internal)
		describer := authz.ConditionalRole(customRole,
			func(ctx context.Context, subject auth.Identity, obj testDoc, scope authz.Scope) (bool, error) {
				return false, expectedErr
			},
		)

		roles, err := describer(t.Context(), identity, doc, "org1")
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, roles)
	})
}

func TestStaticRole(t *testing.T) {
	const viewer = authz.Role("viewer")
	identity := auth.Identity{Subject: "user123"}
	publicDoc := testDoc{id: "1", ownerID: "user456", public: true}
	privateDoc := testDoc{id: "2", ownerID: "user456", public: false}

	describer := authz.StaticRole(viewer,
		func(ctx context.Context, subject auth.Identity, doc testDoc) bool {
			return doc.public
		},
	)

	t.Run("grants role for public document", func(t *testing.T) {
		roles, err := describer(t.Context(), identity, publicDoc, "")
		require.NoError(t, err)
		assert.Equal(t, []authz.Role{viewer}, roles)
	})

	t.Run("does not grant role for private document", func(t *testing.T) {
		roles, err := describer(t.Context(), identity, privateDoc, "")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

func TestStaticRoles(t *testing.T) {
	identity := auth.Identity{Subject: "user123"}
	doc := testDoc{id: "1", ownerID: "user123", orgID: "org1", public: true}

	describer := authz.StaticRoles(func(ctx context.Context, subject auth.Identity, obj testDoc) []authz.Role {
		var roles []authz.Role
		if obj.ownerID == subject.Subject {
			roles = append(roles, authz.RoleOwner)
		}
		if obj.public {
			roles = append(roles, authz.RoleViewer)
		}
		return roles
	})

	roles, err := describer(t.Context(), identity, doc, "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []authz.Role{authz.RoleOwner, authz.RoleViewer}, roles)
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const isSuperUserKey contextKey = "is_super"

func TestGlobalRole(t *testing.T) {
	const superuser = authz.Role("superuser")
	identity := auth.Identity{Subject: "user123"}
	doc := testDoc{id: "1"}

	t.Run("grants role when global predicate returns true", func(t *testing.T) {
		describer := authz.GlobalRole[testDoc](superuser,
			func(ctx context.Context) bool {
				return ctx.Value(isSuperUserKey) == true
			},
		)

		ctx := context.WithValue(t.Context(), isSuperUserKey, true)
		roles, err := describer(ctx, identity, doc, "")
		require.NoError(t, err)
		assert.Equal(t, []authz.Role{superuser}, roles)
	})

	t.Run("does not grant role when global predicate returns false", func(t *testing.T) {
		describer := authz.GlobalRole[testDoc](superuser,
			func(ctx context.Context) bool {
				return ctx.Value(isSuperUserKey) == true
			},
		)

		roles, err := describer(t.Context(), identity, doc, "")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

func TestOwnershipRole(t *testing.T) {
	ownerIdentity := auth.Identity{Subject: "user123"}
	otherIdentity := auth.Identity{Subject: "user456"}
	doc := testDoc{id: "1", ownerID: "user123"}

	describer := authz.OwnershipRole(authz.RoleOwner, func(d testDoc) string {
		return d.ownerID
	})

	t.Run("grants owner role to owner", func(t *testing.T) {
		roles, err := describer(t.Context(), ownerIdentity, doc, "")
		require.NoError(t, err)
		assert.Equal(t, []authz.Role{authz.RoleOwner}, roles)
	})

	t.Run("does not grant owner role to non-owner", func(t *testing.T) {
		roles, err := describer(t.Context(), otherIdentity, doc, "")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

func TestMembershipRoles(t *testing.T) {
	identity := auth.Identity{Subject: "user123"}
	doc := testDoc{id: "1", orgID: "org1"}

	describer := authz.MembershipRoles(
		func(d testDoc) string { return d.orgID },
		func(ctx context.Context, orgID string, subject auth.Identity) ([]authz.Role, error) {
			if orgID == "org1" && subject.Subject == "user123" {
				return []authz.Role{authz.RoleAdmin, authz.RoleEditor}, nil
			}
			return []authz.Role{}, nil
		},
	)

	t.Run("grants roles when scope matches", func(t *testing.T) {
		roles, err := describer(t.Context(), identity, doc, "org1")
		require.NoError(t, err)
		assert.ElementsMatch(t, []authz.Role{authz.RoleAdmin, authz.RoleEditor}, roles)
	})

	t.Run("returns error when scope does not match", func(t *testing.T) {
		roles, err := describer(t.Context(), identity, doc, "org2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scope mismatch")
		assert.Nil(t, roles)
	})

	t.Run("returns empty roles for anonymous users", func(t *testing.T) {
		roles, err := describer(t.Context(), auth.Identity{}, doc, "org1")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

func TestCompose(t *testing.T) {
	ownerIdentity := auth.Identity{Subject: "user123"}
	doc := testDoc{id: "1", ownerID: "user123", orgID: "org1", public: true}

	t.Run("combines multiple describers", func(t *testing.T) {
		describer := authz.Compose(
			authz.OwnershipRole(authz.RoleOwner, func(d testDoc) string { return d.ownerID }),
			authz.StaticRole(authz.RoleViewer, func(_ context.Context, _ auth.Identity, d testDoc) bool { return d.public }),
		)

		roles, err := describer.DescribeRoles(t.Context(), ownerIdentity, doc, "org1")
		require.NoError(t, err)
		assert.ElementsMatch(t, []authz.Role{authz.RoleOwner, authz.RoleViewer}, roles)
	})

	t.Run("type assertion fails with helpful error", func(t *testing.T) {
		describer := authz.Compose(
			authz.OwnershipRole(authz.RoleOwner, func(d testDoc) string { return d.ownerID }),
		)

		// Pass wrong type
		wrongType := "not a testDoc"
		roles, err := describer.DescribeRoles(t.Context(), ownerIdentity, wrongType, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected type")
		assert.Nil(t, roles)
	})

	t.Run("automatically validates scope for ScopedObject", func(t *testing.T) {
		describer := authz.Compose(
			authz.OwnershipRole(authz.RoleOwner, func(d testDoc) string { return d.ownerID }),
		)

		// Scope mismatch - should return empty roles
		roles, err := describer.DescribeRoles(t.Context(), ownerIdentity, doc, "wrong-scope")
		require.NoError(t, err)
		assert.Empty(t, roles)

		// Scope matches - should return roles
		roles, err = describer.DescribeRoles(t.Context(), ownerIdentity, doc, "org1")
		require.NoError(t, err)
		assert.Equal(t, []authz.Role{authz.RoleOwner}, roles)
	})

	t.Run("propagates errors from describers", func(t *testing.T) {
		expectedErr := errors.NewC("test error", codes.Internal)
		describer := authz.Compose(
			authz.OwnershipRole(authz.RoleOwner, func(d testDoc) string { return d.ownerID }),
			authz.ConditionalRole(authz.RoleEditor, func(_ context.Context, _ auth.Identity, _ testDoc, _ authz.Scope) (bool, error) {
				return false, expectedErr
			}),
		)

		roles, err := describer.DescribeRoles(t.Context(), ownerIdentity, doc, "org1")
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, roles)
	})
}

func TestValidateScope(t *testing.T) {
	identity := auth.Identity{Subject: "user123"}
	doc := testDoc{id: "1", ownerID: "user123", orgID: "org1"}

	baseDescriber := authz.OwnershipRole(authz.RoleOwner, func(d testDoc) string { return d.ownerID })
	wrappedDescriber := authz.ValidateScope(baseDescriber)

	t.Run("grants roles when scope matches", func(t *testing.T) {
		roles, err := wrappedDescriber(t.Context(), identity, doc, "org1")
		require.NoError(t, err)
		assert.Equal(t, []authz.Role{authz.RoleOwner}, roles)
	})

	t.Run("returns empty roles when scope does not match", func(t *testing.T) {
		roles, err := wrappedDescriber(t.Context(), identity, doc, "wrong-scope")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

// Integration test: Real-world scenario
func TestRealWorldScenario(t *testing.T) {
	const (
		superuser = authz.Role("superuser")
		author    = authz.Role("author")
	)

	// Simulate a student note with workspace scope
	type studentNote struct {
		id          string
		workspaceID string
		createdBy   *string
	}

	createdBy := "user123"
	note := studentNote{
		id:          "note1",
		workspaceID: "workspace1",
		createdBy:   &createdBy,
	}

	superuserIdentity := auth.Identity{Subject: "admin"}
	authorIdentity := auth.Identity{Subject: "user123"}
	otherIdentity := auth.Identity{Subject: "user456"}

	describer := authz.Compose(
		// Superuser override
		authz.GlobalRole[studentNote](superuser, func(ctx context.Context) bool {
			return ctx.Value(isSuperUserKey) == true
		}),

		// Author role
		authz.ConditionalRole(author, func(_ context.Context, subject auth.Identity, n studentNote, _ authz.Scope) (bool, error) {
			return n.createdBy != nil && *n.createdBy == subject.Subject, nil
		}),

		// Workspace roles would go here
		authz.StaticRoles(func(_ context.Context, subject auth.Identity, _ studentNote) []authz.Role {
			// Simplified - in real code would check workspace membership
			if subject.Subject == "user123" {
				return []authz.Role{authz.RoleEditor}
			}
			return []authz.Role{authz.RoleViewer}
		}),
	)

	t.Run("superuser gets superuser role", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), isSuperUserKey, true)
		roles, err := describer.DescribeRoles(ctx, superuserIdentity, note, "workspace1")
		require.NoError(t, err)
		assert.Contains(t, roles, superuser)
	})

	t.Run("author gets author and editor roles", func(t *testing.T) {
		roles, err := describer.DescribeRoles(t.Context(), authorIdentity, note, "workspace1")
		require.NoError(t, err)
		assert.ElementsMatch(t, []authz.Role{author, authz.RoleEditor}, roles)
	})

	t.Run("other user gets only viewer role", func(t *testing.T) {
		roles, err := describer.DescribeRoles(t.Context(), otherIdentity, note, "workspace1")
		require.NoError(t, err)
		assert.Equal(t, []authz.Role{authz.RoleViewer}, roles)
	})
}
