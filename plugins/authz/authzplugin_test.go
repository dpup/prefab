package authz_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/authz"
	"github.com/dpup/prefab/plugins/authz/authztest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func TestAuthzPlugin_determineEffect(t *testing.T) {
	type args struct {
		action        authz.Action
		roles         []authz.Role
		defaultEffect authz.Effect
	}
	tests := []struct {
		name string
		args args
		want authz.Effect
	}{
		{
			name: "Allow admin write access",
			args: args{
				action:        "write",
				roles:         []authz.Role{"admin"},
				defaultEffect: authz.Deny,
			},
			want: authz.Allow,
		},
		{
			name: "Allow write access when one role matches",
			args: args{
				action:        "write",
				roles:         []authz.Role{"admin", "standard"},
				defaultEffect: authz.Deny,
			},
			want: authz.Allow,
		},
		{
			name: "Deny standard write access",
			args: args{
				action:        "write",
				roles:         []authz.Role{"standard"},
				defaultEffect: authz.Deny,
			},
			want: authz.Deny,
		},
		{
			name: "Deny no roles write access",
			args: args{
				action:        "write",
				roles:         []authz.Role{},
				defaultEffect: authz.Deny,
			},
			want: authz.Deny,
		},
		{
			name: "Deny write access role explicitly overrides",
			args: args{
				action:        "write",
				roles:         []authz.Role{"admin", "nyc-employee"},
				defaultEffect: authz.Deny,
			},
			want: authz.Deny,
		},
		{
			name: "Deny write access role overrides default allow",
			args: args{
				action:        "write",
				roles:         []authz.Role{"nyc-employee"},
				defaultEffect: authz.Allow,
			},
			want: authz.Deny,
		},
		{
			name: "AWS IAM: Explicit deny should block even with allow from another role (default allow)",
			args: args{
				action:        "delete",
				roles:         []authz.Role{"admin", "readonly"},
				defaultEffect: authz.Allow,
			},
			want: authz.Deny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ap := authz.Plugin(
				authz.WithPolicy(authz.Allow, authz.Role("admin"), authz.Action("write")),
				authz.WithPolicy(authz.Deny, authz.Role("nyc-employee"), authz.Action("write")),
				authz.WithPolicy(authz.Allow, authz.Role("admin"), authz.Action("delete")),
				authz.WithPolicy(authz.Deny, authz.Role("readonly"), authz.Action("delete")),
			)
			got, _ := ap.DetermineEffect(tt.args.action, tt.args.roles, tt.args.defaultEffect)
			if got != tt.want {
				t.Errorf("AuthzPlugin.determineEffect() = %v, want %v", got, tt.want)
			}
		})
	}
}

type testDocument struct {
	id     string
	author string
	title  string
	body   string
}

// Implement the authz interfaces.
func (d *testDocument) AuthzType() string { return "document" }
func (d *testDocument) OwnerID() string   { return d.author }

func TestInterceptor(t *testing.T) {
	// Create function-based role describers and object fetchers
	documentFetcher := func(ctx context.Context, key any) (any, error) {
		switch key.(string) {
		case "1":
			return &testDocument{id: "1", author: "bob@test.com", title: "Test Document", body: "This is a test document."}, nil
		case "2":
			return &testDocument{id: "2", author: "betty@test.com", title: "Another Document", body: "This is another test document."}, nil
		default:
			return nil, errors.Codef(codes.NotFound, "document not found")
		}
	}

	documentRoleDescriber := func(ctx context.Context, subject auth.Identity, object any, scope authz.Scope) ([]authz.Role, error) {
		doc := object.(*testDocument)
		if subject.Email == doc.author {
			return []authz.Role{"admin"}, nil
		} else if subject.Email != "" {
			return []authz.Role{"standard"}, nil
		} else {
			return []authz.Role{}, nil
		}
	}

	wildFetcher := func(ctx context.Context, key any) (any, error) {
		// For the test we don't care what gets returned, in reality the '*' might
		// return something like the identity for the user, a session object, or
		// a root entity such as a workspace. Key will be an empty string.
		return 1, nil
	}

	wildRoleDescriber := func(ctx context.Context, subject auth.Identity, object any, scope authz.Scope) ([]authz.Role, error) {
		if subject == (auth.Identity{}) {
			return []authz.Role{"anonymous"}, nil
		} else {
			return []authz.Role{"authenticated"}, nil
		}
	}

	ap := authz.Plugin(
		authz.WithPolicy(authz.Allow, authz.Role("admin"), authz.Action("documents.write")),
		authz.WithPolicy(authz.Allow, authz.Role("admin"), authz.Action("documents.view")),
		authz.WithPolicy(authz.Allow, authz.Role("standard"), authz.Action("documents.view")),
		authz.WithPolicy(authz.Deny, authz.Role("nyc-employee"), authz.Action("documents.write")),
		authz.WithPolicy(authz.Allow, authz.Role("authenticated"), authz.Action("self.inspect")),

		// Register function-based objects via the helpers
		authz.WithObjectFetcherFn("document", documentFetcher),
		authz.WithRoleDescriberFn("document", documentRoleDescriber),
		authz.WithObjectFetcherFn("*", wildFetcher),
		authz.WithRoleDescriberFn("*", wildRoleDescriber),
	)

	type args struct {
		identity auth.Identity
		req      interface{}
		method   string
	}
	tests := []struct {
		name          string
		args          args
		handlerCalled bool
		expectedErr   error
	}{
		{
			name: "Author should be able to access own document",
			args: args{
				identity: auth.Identity{Email: "bob@test.com", Provider: "test"},
				req:      &authztest.GetDocumentRequest{DocumentId: "1"},
				method:   authztest.AuthzTestService_GetDocument_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Other user with email should be able to access another document",
			args: args{
				identity: auth.Identity{Email: "betty@test.com", Provider: "test"},
				req:      &authztest.GetDocumentRequest{DocumentId: "1"},
				method:   authztest.AuthzTestService_GetDocument_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Zero identity should be blocked",
			args: args{
				identity: auth.Identity{},
				req:      &authztest.GetDocumentRequest{DocumentId: "1"},
				method:   authztest.AuthzTestService_GetDocument_FullMethodName,
			},
			handlerCalled: false,
			expectedErr:   authz.ErrUnauthenticated,
		},
		{
			name: "Identity with empty email should be blocked",
			args: args{
				identity: auth.Identity{Subject: "aaa", Email: "", Provider: "test"},
				req:      &authztest.GetDocumentRequest{DocumentId: "1"},
				method:   authztest.AuthzTestService_GetDocument_FullMethodName,
			},
			handlerCalled: false,
			expectedErr:   authz.ErrPermissionDenied,
		},
		{
			name: "Author should be able to save own document",
			args: args{
				identity: auth.Identity{Email: "bob@test.com", Provider: "test"},
				req:      &authztest.GetDocumentRequest{DocumentId: "1"},
				method:   authztest.AuthzTestService_SaveDocument_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Other user with email should not be able to save document",
			args: args{
				identity: auth.Identity{Email: "betty@test.com", Provider: "test"},
				req:      &authztest.GetDocumentRequest{DocumentId: "1"},
				method:   authztest.AuthzTestService_SaveDocument_FullMethodName,
			},
			handlerCalled: false,
			expectedErr:   authz.ErrPermissionDenied,
		},
		{
			name: "Method with no Policy should execute",
			args: args{
				identity: auth.Identity{Email: "betty@test.com", Provider: "test"},
				req:      &authztest.Request{},
				method:   authztest.AuthzTestService_NoPolicy_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Authenticated user should be able to call action only method",
			args: args{
				identity: auth.Identity{Email: "betty@test.com", Provider: "test"},
				req:      &authztest.Request{},
				method:   authztest.AuthzTestService_Self_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Anonymous user should not be able to call action only method",
			args: args{
				identity: auth.Identity{},
				req:      &authztest.Request{},
				method:   authztest.AuthzTestService_Self_FullMethodName,
			},
			handlerCalled: false,
			expectedErr:   authz.ErrUnauthenticated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := auth.WithIdentityForTest(t.Context(), tt.args.identity)
			handlerCalled := false
			handlerResponse := &authztest.Response{Success: true}
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				handlerCalled = true
				return handlerResponse, nil
			}
			info := &grpc.UnaryServerInfo{FullMethod: tt.args.method}

			// Test the interceptor.
			gotResp, err := ap.Interceptor(ctx, tt.args.req, info, handler)
			require.ErrorIs(t, err, tt.expectedErr, "AuthzPlugin.Interceptor() error = %v, expectedErr %v", err, tt.expectedErr)
			if handlerCalled != tt.handlerCalled {
				t.Errorf("AuthzPlugin.Interceptor() handlerCalled = %v, want %v", handlerCalled, tt.handlerCalled)
			}
			if err == nil && !reflect.DeepEqual(gotResp, handlerResponse) {
				t.Errorf("AuthzPlugin.Interceptor() = %v, want %v", gotResp, handlerResponse)
			}
		})
	}
}

func TestAuthzPlugin_RoleHierarchy(t *testing.T) {
	owner := authz.Role("owner")
	admin := authz.Role("admin")
	editor := authz.Role("editor")
	suggester := authz.Role("suggester")
	curator := authz.Role("curator")
	viewer := authz.Role("viewer")
	member := authz.Role("member")

	ap := &authz.AuthzPlugin{}
	ap.SetRoleHierarchy(owner, admin, editor, viewer, member)
	ap.SetRoleHierarchy(suggester, viewer)
	ap.SetRoleHierarchy(curator, viewer)

	tests := []struct {
		role     authz.Role
		expected []authz.Role
	}{
		{role: owner, expected: []authz.Role{owner, admin, editor, viewer, member}},
		{role: admin, expected: []authz.Role{admin, editor, viewer, member}},
		{role: editor, expected: []authz.Role{editor, viewer, member}},
		{role: suggester, expected: []authz.Role{suggester, viewer, member}},
		{role: curator, expected: []authz.Role{curator, viewer, member}},
		{role: viewer, expected: []authz.Role{viewer, member}},
	}
	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			actual := ap.RoleHierarchy(tt.role)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestAuthzPlugin_RoleHierarchy_Inverted(t *testing.T) {
	owner := authz.Role("owner")
	admin := authz.Role("admin")
	editor := authz.Role("editor")
	suggester := authz.Role("suggester")
	viewer := authz.Role("viewer")
	member := authz.Role("member")

	ap := &authz.AuthzPlugin{}
	ap.SetRoleHierarchy(suggester, viewer)
	ap.SetRoleHierarchy(owner, admin, editor, viewer, member)

	tests := []struct {
		role     authz.Role
		expected []authz.Role
	}{
		{role: owner, expected: []authz.Role{owner, admin, editor, viewer, member}},
		{role: suggester, expected: []authz.Role{suggester, viewer, member}},
	}
	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			actual := ap.RoleHierarchy(tt.role)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestAuthzPlugin_RoleHierarchy_Error(t *testing.T) {
	owner := authz.Role("owner")
	admin := authz.Role("admin")
	editor := authz.Role("editor")
	suggester := authz.Role("suggester")

	assert.Panics(t, func() {
		ap := &authz.AuthzPlugin{}
		ap.SetRoleHierarchy(owner, admin, editor, suggester)
		ap.SetRoleHierarchy(editor, owner) // editor's parent is already suggester.
	}, "expect panic when reassigning a parent role")
}

func TestAuthzPlugin_RoleHierarchy_Cycle(t *testing.T) {
	owner := authz.Role("owner")
	admin := authz.Role("admin")
	editor := authz.Role("editor")

	assert.Panics(t, func() {
		ap := &authz.AuthzPlugin{}
		ap.SetRoleHierarchy(owner, admin, editor, owner)
	}, "expect panic when creating a cycle")

	assert.Panics(t, func() {
		ap := &authz.AuthzPlugin{}
		ap.SetRoleHierarchy(owner, admin, editor)
		ap.SetRoleHierarchy(editor, owner)
	}, "expect panic when creating a cycle")
}

func TestDetermineEffectWithPolicyEvaluation(t *testing.T) {
	ap := authz.Plugin(
		authz.WithPolicy(authz.Allow, authz.Role("editor"), authz.Action("write")),
		authz.WithPolicy(authz.Deny, authz.Role("suspended"), authz.Action("write")),
		authz.WithPolicy(authz.Allow, authz.Role("admin"), authz.Action("delete")),
	)

	tests := []struct {
		name               string
		action             authz.Action
		roles              []authz.Role
		defaultEffect      authz.Effect
		wantEffect         authz.Effect
		wantEvaluatedCount int
	}{
		{
			name:               "explicit deny wins",
			action:             authz.Action("write"),
			roles:              []authz.Role{"editor", "suspended"},
			defaultEffect:      authz.Deny,
			wantEffect:         authz.Deny,
			wantEvaluatedCount: 2,
		},
		{
			name:               "explicit allow with no deny",
			action:             authz.Action("write"),
			roles:              []authz.Role{"editor"},
			defaultEffect:      authz.Deny,
			wantEffect:         authz.Allow,
			wantEvaluatedCount: 1,
		},
		{
			name:               "simple allow policy",
			action:             authz.Action("delete"),
			roles:              []authz.Role{"admin"},
			defaultEffect:      authz.Deny,
			wantEffect:         authz.Allow,
			wantEvaluatedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect, evaluated := ap.DetermineEffect(tt.action, tt.roles, tt.defaultEffect)
			if effect != tt.wantEffect {
				t.Errorf("effect = %v, want %v", effect, tt.wantEffect)
			}
			if len(evaluated) != tt.wantEvaluatedCount {
				t.Errorf("evaluated policies count = %d, want %d", len(evaluated), tt.wantEvaluatedCount)
			}
		})
	}
}

func TestWithAuditLogger(t *testing.T) {
	var auditedDecisions []authz.AuthzDecision

	ap := authz.Plugin(
		authz.WithPolicy(authz.Allow, authz.Role("viewer"), authz.Action("read")),
		authz.WithAuditLogger(func(ctx context.Context, decision authz.AuthzDecision) {
			auditedDecisions = append(auditedDecisions, decision)
		}),
		authz.WithObjectFetcher("test", authz.AsObjectFetcher(
			authz.MapFetcher(map[string]*testDocument{
				"1": {id: "1", author: "alice", title: "Test", body: "test"},
			}),
		)),
		authz.WithRoleDescriber("test", authz.Compose(
			authz.StaticRole(authz.Role("viewer"), func(_ context.Context, _ auth.Identity, _ *testDocument) bool {
				return true
			}),
		)),
	)

	ctx := auth.WithIdentityForTest(t.Context(), auth.Identity{Subject: "user1", Provider: "test"})

	// Test allowed access
	err := ap.Authorize(ctx, authz.AuthorizeParams{
		ObjectKey:     "test",
		ObjectID:      "1",
		Action:        authz.Action("read"),
		DefaultEffect: authz.Deny,
		Info:          "test",
	})

	if err != nil {
		t.Errorf("expected access to be allowed, got error: %v", err)
	}

	if len(auditedDecisions) != 1 {
		t.Fatalf("expected 1 audit log entry, got %d", len(auditedDecisions))
	}

	decision := auditedDecisions[0]
	if decision.Effect != authz.Allow {
		t.Errorf("decision.Effect = %v, want Allow", decision.Effect)
	}
	if len(decision.Roles) != 1 || decision.Roles[0] != authz.Role("viewer") {
		t.Errorf("decision.Roles = %v, want [viewer]", decision.Roles)
	}
	if len(decision.EvaluatedPolicies) == 0 {
		t.Error("decision.EvaluatedPolicies is empty, expected at least one")
	}
}
