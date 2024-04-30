// An example of how to use the ACL plugin.
//
// $ go run examples/acls/aclexample.go
//
// At time of writing there is no web UI to exercise the endpoints, so you'll
// need to use CURL (or equivalent). Use the following commands to try things
// out:
//
// Get an access token for a specific user using email/password:
//
//	curl 'http://localhost:8000/api/auth/login?provider=password&creds%5Bemail%5D=logan@xmen.net&creds%5Bpassword%5D=password&issue_token=true'
//
// Save the token in an environment variable:
//
//	export AT='...'
//
// List documents:
//
//	curl -H "Authorization: bearer $AT" http://localhost:8000/api/xmen/docs
//
// View a document:
//
//	curl -H "Authorization: bearer $AT" http://localhost:8000/api/xmen/docs/3
//
// Save a document:
//
//	curl -X PUT -d '{"title": "new title", "body": "new body"}' -H"Authorization: bearer $AT" -H"X-CSRF-Protection: 1" http://localhost:8000/api/xmen/docs/3
package main

import (
	"context"
	"fmt"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/acl"
	"github.com/dpup/prefab/acl/acltest"
	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/auth/pwdauth"
	"github.com/dpup/prefab/errors"
	"google.golang.org/grpc/codes"
)

const (
	roleStandard = acl.Role("sys.standard")
	roleAdmin    = acl.Role("sys.admin")
	roleDocOwner = acl.Role("doc.owner")
)

type org struct {
	name string
}

type document struct {
	id     string
	author string
	title  string
	body   string
}

func main() {
	s := prefab.New(
		// Use basic email/password auth so that we can demonstrate different users
		// seeing different results.
		prefab.WithPlugin(auth.Plugin()),
		prefab.WithPlugin(pwdauth.Plugin(
			pwdauth.WithAccountFinder(accountStore{}), // Static user data.
			pwdauth.WithHasher(pwdauth.TestHasher),    // Doesn't hash passwords.
		)),

		// Set up the example ACLs.
		// - Any user can list document titles.
		// - The owner or an admin can view a specific document.
		// - Only the owner can write to a document.
		// - For this example, roles will be additive. All authenticated users will
		//   have the "standard" role. Then optionally "admin" and/or "owner".
		prefab.WithPlugin(acl.Plugin(
			acl.WithPolicy(acl.Allow, roleStandard, acl.Action("documents.view_meta")),
			acl.WithPolicy(acl.Allow, roleStandard, acl.Action("documents.list")),
			acl.WithPolicy(acl.Allow, roleDocOwner, acl.Action("documents.view")),
			acl.WithPolicy(acl.Allow, roleDocOwner, acl.Action("documents.write")),
			acl.WithPolicy(acl.Allow, roleAdmin, acl.Action("documents.view")),
			acl.WithObjectFetcher("org", fetchOrg),
			acl.WithObjectFetcher("document", fetchDocument),
			acl.WithRoleDescriber("*", roleDescriber),
		)),

		// TODO: Add basic web UI to make it easier to exercise the endpoints.
	)

	// Register the GRPC service defined in the acltest package.
	acltest.RegisterAclTestServiceHandlerFromEndpoint(s.GatewayArgs())
	acltest.RegisterAclTestServiceServer(s.ServiceRegistrar(), &testServer{})

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

// ObjectFetcher associated with the "org" type which comes from the ACL
// specification in the proto description.
func fetchOrg(ctx context.Context, key any) (any, error) {
	if key.(string) == "xmen" {
		return org{name: "xmen"}, nil
	}
	return nil, errors.NewC("org not found", codes.NotFound)
}

// Object fecher for "document" type.
func fetchDocument(ctx context.Context, key any) (any, error) {
	if doc, ok := staticDocuments[key.(string)]; ok {
		return doc, nil
	}
	return nil, errors.NewC("document not found", codes.NotFound)
}

// RoleDescriber for all objects.
func roleDescriber(ctx context.Context, id auth.Identity, object any, domain acl.Domain) ([]acl.Role, error) {
	// Assume just one domain/org/workspace for this example.
	switch o := object.(type) {
	case document:
		if domain != "xmen" {
			return []acl.Role{}, nil
		}
	case org:
		if o.name != "xmen" {
			return []acl.Role{}, nil
		}
	default:
		return nil, errors.NewC("unknown object type", codes.InvalidArgument)
	}

	if _, ok := object.(document); ok {
		// Assume just one domain/org/workspace for this example.
		if domain != "xmen" {
			return []acl.Role{}, nil
		}
	}

	// All xmen get the standard role.
	roles := []acl.Role{roleStandard}

	// Wolverine gets to be an admin.
	if id.Email == "logan@xmen.net" {
		roles = append(roles, roleAdmin)
	}

	// The author of a document is the owner.
	if doc, ok := object.(document); ok && doc.author == id.Subject {
		roles = append(roles, roleDocOwner)
	}

	return roles, nil
}

// Implements acltest.AclTestServiceServer so we can demonstrate the ACLs in
// action. A very minimal implementation.
type testServer struct {
	acltest.UnimplementedAclTestServiceServer
}

func (t *testServer) ListDocuments(ctx context.Context, in *acltest.ListDocumentsRequest) (*acltest.ListDocumentsResponse, error) {
	return &acltest.ListDocumentsResponse{DocumentIds: []string{"1", "2", "3"}}, nil
}

func (t *testServer) GetDocument(ctx context.Context, in *acltest.GetDocumentRequest) (*acltest.GetDocumentResponse, error) {
	doc := staticDocuments[in.DocumentId]
	return &acltest.GetDocumentResponse{
		Id:    doc.id,
		Title: doc.title,
		Body:  doc.body,
	}, nil
}

func (t *testServer) SaveDocument(ctx context.Context, in *acltest.SaveDocumentRequest) (*acltest.SaveDocumentResponse, error) {
	doc := staticDocuments[in.DocumentId]
	doc.title = in.Title
	doc.body = in.Body
	return &acltest.SaveDocumentResponse{
		Id:    doc.id,
		Title: doc.title,
		Body:  doc.body,
	}, nil
}

// Static user data used by the pwdauth plugin. This allows you to login as
// different users to see different results.
type accountStore struct{}

func (a accountStore) FindAccount(ctx context.Context, email string) (*pwdauth.Account, error) {
	for _, acc := range staticAccounts {
		if acc.Email == email {
			return acc, nil
		}
	}
	return nil, errors.Codef(codes.NotFound, "account not found")
}

// Logan is an admin, so can view all docs. Jean is author of first two docs,
// and Scott as author of the 3rd.
var staticDocuments = map[string]document{
	"1": {id: "1", author: "3", title: "The Phoenix Saga", body: "A long time ago..."},
	"2": {id: "2", author: "3", title: "The Dark Phoenix Saga", body: "A long time ago..."},
	"3": {id: "3", author: "2", title: "Days of Future Past", body: "A long time ago..."},
}

var staticAccounts = []*pwdauth.Account{
	{
		ID:             "1",
		Email:          "logan@xmen.net",
		Name:           "Logan",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
	{
		ID:             "2",
		Email:          "scott@xmen.net",
		Name:           "Scott Summers",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
	{
		ID:             "3",
		Email:          "jean@xmen.net",
		Name:           "Jean Grey",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
	{
		ID:             "4",
		Email:          "ororo@xmen.net",
		Name:           "Ororo Munroe",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
	{
		ID:             "5",
		Email:          "kurt@xmen.net",
		Name:           "Kurt Wagner",
		EmailVerified:  true,
		HashedPassword: []byte("password"),
	},
}
