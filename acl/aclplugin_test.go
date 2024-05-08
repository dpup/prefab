package acl_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/dpup/prefab/acl"
	"github.com/dpup/prefab/acl/acltest"
	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func TestAclPlugin_determineEffect(t *testing.T) {
	type args struct {
		action        acl.Action
		roles         []acl.Role
		defaultEffect acl.Effect
	}
	tests := []struct {
		name string
		args args
		want acl.Effect
	}{
		{
			name: "Allow admin write access",
			args: args{
				action:        "write",
				roles:         []acl.Role{"admin"},
				defaultEffect: acl.Deny,
			},
			want: acl.Allow,
		},
		{
			name: "Allow write access when one role matches",
			args: args{
				action:        "write",
				roles:         []acl.Role{"admin", "standard"},
				defaultEffect: acl.Deny,
			},
			want: acl.Allow,
		},
		{
			name: "Deny standard write access",
			args: args{
				action:        "write",
				roles:         []acl.Role{"standard"},
				defaultEffect: acl.Deny,
			},
			want: acl.Deny,
		},
		{
			name: "Deny no roles write access",
			args: args{
				action:        "write",
				roles:         []acl.Role{},
				defaultEffect: acl.Deny,
			},
			want: acl.Deny,
		},
		{
			name: "Deny write access role explicitly overrides",
			args: args{
				action:        "write",
				roles:         []acl.Role{"admin", "nyc-employee"},
				defaultEffect: acl.Deny,
			},
			want: acl.Deny,
		},
		{
			name: "Deny write access role overrides default allow",
			args: args{
				action:        "write",
				roles:         []acl.Role{"nyc-employee"},
				defaultEffect: acl.Allow,
			},
			want: acl.Deny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ap := acl.Plugin(
				acl.WithPolicy(acl.Allow, acl.Role("admin"), acl.Action("write")),
				acl.WithPolicy(acl.Deny, acl.Role("nyc-employee"), acl.Action("write")),
			)
			if got := ap.DetermineEffect(tt.args.action, tt.args.roles, tt.args.defaultEffect); got != tt.want {
				t.Errorf("AclPlugin.determineEffect() = %v, want %v", got, tt.want)
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

func TestInterceptor(t *testing.T) {
	ap := acl.Plugin(
		acl.WithPolicy(acl.Allow, acl.Role("admin"), acl.Action("documents.write")),
		acl.WithPolicy(acl.Allow, acl.Role("admin"), acl.Action("documents.view")),
		acl.WithPolicy(acl.Allow, acl.Role("standard"), acl.Action("documents.view")),
		acl.WithPolicy(acl.Deny, acl.Role("nyc-employee"), acl.Action("documents.write")),
		acl.WithObjectFetcher("document", func(ctx context.Context, key any) (any, error) {
			switch key.(string) {
			case "1":
				return &testDocument{id: "1", author: "bob@test.com", title: "Test Document", body: "This is a test document."}, nil
			case "2":
				return &testDocument{id: "2", author: "betty@test.com", title: "Another Document", body: "This is another test document."}, nil
			default:
				return nil, errors.Codef(codes.NotFound, "document not found")
			}
		}),
		acl.WithRoleDescriber("document", func(ctx context.Context, subject auth.Identity, object any, domain acl.Domain) ([]acl.Role, error) {
			doc := object.(*testDocument)
			if subject.Email == doc.author {
				return []acl.Role{"admin"}, nil
			} else if subject.Email != "" {
				return []acl.Role{"standard"}, nil
			} else {
				return []acl.Role{}, nil
			}
		}),

		acl.WithPolicy(acl.Allow, acl.Role("authenticated"), acl.Action("self.inspect")),
		acl.WithObjectFetcher("*", func(ctx context.Context, key any) (any, error) {
			// For the test we don't care what gets returned, in reality the '*' might
			// return something like the identity for the user, a session object, or
			// a root entity such as a workspace. Key will be an empty string.
			return 1, nil
		}),
		acl.WithRoleDescriber("*", func(ctx context.Context, subject auth.Identity, object any, domain acl.Domain) ([]acl.Role, error) {
			if subject == (auth.Identity{}) {
				return []acl.Role{"anonymous"}, nil
			} else {
				return []acl.Role{"authenticated"}, nil
			}
		}),
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
				identity: auth.Identity{Email: "bob@test.com"},
				req:      &acltest.GetDocumentRequest{DocumentId: "1"},
				method:   acltest.AclTestService_GetDocument_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Other user with email should be able to access another document",
			args: args{
				identity: auth.Identity{Email: "betty@test.com"},
				req:      &acltest.GetDocumentRequest{DocumentId: "1"},
				method:   acltest.AclTestService_GetDocument_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Identity without email should be blocked",
			args: args{
				identity: auth.Identity{},
				req:      &acltest.GetDocumentRequest{DocumentId: "1"},
				method:   acltest.AclTestService_GetDocument_FullMethodName,
			},
			handlerCalled: false,
			expectedErr:   acl.ErrPermissionDenied,
		},
		{
			name: "Author should be able to save own document",
			args: args{
				identity: auth.Identity{Email: "bob@test.com"},
				req:      &acltest.GetDocumentRequest{DocumentId: "1"},
				method:   acltest.AclTestService_SaveDocument_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Other user with email should not be able to save document",
			args: args{
				identity: auth.Identity{Email: "betty@test.com"},
				req:      &acltest.GetDocumentRequest{DocumentId: "1"},
				method:   acltest.AclTestService_SaveDocument_FullMethodName,
			},
			handlerCalled: false,
			expectedErr:   acl.ErrPermissionDenied,
		},
		{
			name: "Method with no ACL should execute",
			args: args{
				identity: auth.Identity{Email: "betty@test.com"},
				req:      &acltest.Request{},
				method:   acltest.AclTestService_NoACL_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Authenticated user should be able to call action only method",
			args: args{
				identity: auth.Identity{Email: "betty@test.com"},
				req:      &acltest.Request{},
				method:   acltest.AclTestService_Self_FullMethodName,
			},
			handlerCalled: true,
		},
		{
			name: "Anonymous user should not be able to call action only method",
			args: args{
				identity: auth.Identity{},
				req:      &acltest.Request{},
				method:   acltest.AclTestService_Self_FullMethodName,
			},
			handlerCalled: false,
			expectedErr:   acl.ErrPermissionDenied,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := auth.ContextWithIdentityForTest(context.Background(), tt.args.identity)
			handlerCalled := false
			handlerResponse := &acltest.Response{Success: true}
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				handlerCalled = true
				return handlerResponse, nil
			}
			info := &grpc.UnaryServerInfo{FullMethod: tt.args.method}

			// Test the interceptor.
			gotResp, err := ap.Interceptor(ctx, tt.args.req, info, handler)
			assert.ErrorIs(t, err, tt.expectedErr, "AclPlugin.Interceptor() error = %v, expectedErr %v", err, tt.expectedErr)
			if handlerCalled != tt.handlerCalled {
				t.Errorf("AclPlugin.Interceptor() handlerCalled = %v, want %v", handlerCalled, tt.handlerCalled)
			}
			if err == nil && !reflect.DeepEqual(gotResp, handlerResponse) {
				t.Errorf("AclPlugin.Interceptor() = %v, want %v", gotResp, handlerResponse)
			}
		})
	}

}
