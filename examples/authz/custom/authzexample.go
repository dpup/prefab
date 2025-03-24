// An example of how to use the Authz plugin with fully custom configuration.
//
// $ go run examples/authz/main.go -example=custom
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
package custom

import (
	"context"
	"fmt"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/auth/pwdauth"
	"github.com/dpup/prefab/plugins/authz"
	"github.com/dpup/prefab/plugins/authz/authztest"
	"google.golang.org/grpc/codes"
)

const (
	roleStandard = authz.Role("sys.standard")
	roleAdmin    = authz.Role("sys.admin")
	roleDocOwner = authz.Role("doc.owner")
)

// Org implements the AuthzObject interface
type org struct {
	name string
}

// AuthzType returns the object type
func (o org) AuthzType() string {
	return "org"
}

// ScopeID implements the ScopedObject interface
func (o org) ScopeID() string {
	return o.name
}

// Document implements the OwnedObject and ScopedObject interfaces
type document struct {
	id     string
	author string
	title  string
	body   string
	org    string // the organization this document belongs to
}

// AuthzType implements the AuthzObject interface
func (d document) AuthzType() string {
	return "document"
}

// OwnerID implements the OwnedObject interface
func (d document) OwnerID() string {
	return d.author
}

// ScopeID implements the ScopedObject interface
func (d document) ScopeID() string {
	return d.org
}

// Run starts the custom authz example server
func Run() {
	// Create our custom RoleDescriber implementation
	customRoleDescriber := &CustomRoleDescriber{}

	s := prefab.New(
		// Use basic email/password auth so that we can demonstrate different users
		// seeing different results.
		prefab.WithPlugin(auth.Plugin()),
		prefab.WithPlugin(pwdauth.Plugin(
			pwdauth.WithAccountFinder(accountStore{}), // Static user data.
			pwdauth.WithHasher(pwdauth.TestHasher),    // Doesn't hash passwords.
		)),

		// Set up the example policies and rules with fully custom configuration
		// - Any user can list document titles.
		// - The owner or an admin can view a specific document.
		// - Only the owner can write to a document.
		// - For this example, roles will be additive. All authenticated users will
		//   have the "standard" role. Then optionally "admin" and/or "owner".
		prefab.WithPlugin(authz.Plugin(
			authz.WithPolicy(authz.Allow, roleStandard, authz.Action("documents.view_meta")),
			authz.WithPolicy(authz.Allow, roleStandard, authz.Action("documents.list")),
			authz.WithPolicy(authz.Allow, roleDocOwner, authz.Action("documents.view")),
			authz.WithPolicy(authz.Allow, roleDocOwner, authz.Action("documents.write")),
			authz.WithPolicy(authz.Allow, roleAdmin, authz.Action("documents.view")),
			authz.WithFunctionObjectFetcher("org", fetchOrg),
			authz.WithFunctionObjectFetcher("document", fetchDocument),
			authz.WithRoleDescriber("*", customRoleDescriber),
		)),

		// TODO: Add basic web UI to make it easier to exercise the endpoints.
	)

	// Register the GRPC service defined in the authztest package.
	authztest.RegisterAuthzTestServiceHandlerFromEndpoint(s.GatewayArgs())
	authztest.RegisterAuthzTestServiceServer(s.ServiceRegistrar(), &testServer{})

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

// ObjectFetcher associated with the "org" type which comes from the Authz
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

// CustomRoleDescriber implements the RoleDescriber interface
type CustomRoleDescriber struct{}

// DescribeRoles implements the RoleDescriber interface
func (d *CustomRoleDescriber) DescribeRoles(ctx context.Context, id auth.Identity, object any, scope authz.Scope) ([]authz.Role, error) {
	// Check the scope for this object
	if scoped, ok := object.(authz.ScopedObject); ok {
		if scoped.ScopeID() != string(scope) {
			return []authz.Role{}, nil
		}
	}

	// All authenticated users get the standard role
	roles := []authz.Role{roleStandard}

	// Wolverine gets to be an admin
	if id.Email == "logan@xmen.net" {
		roles = append(roles, roleAdmin)
	}

	// The author of a document is the owner
	if owned, ok := object.(authz.OwnedObject); ok {
		if owned.OwnerID() == id.Subject {
			roles = append(roles, roleDocOwner)
		}
	}

	return roles, nil
}

// Implements authztest.AuthzTestServiceServer so we can demonstrate the
// policies in action. A very minimal implementation.
type testServer struct {
	authztest.UnimplementedAuthzTestServiceServer
}

func (t *testServer) ListDocuments(ctx context.Context, in *authztest.ListDocumentsRequest) (*authztest.ListDocumentsResponse, error) {
	return &authztest.ListDocumentsResponse{DocumentIds: []string{"1", "2", "3"}}, nil
}

func (t *testServer) GetDocument(ctx context.Context, in *authztest.GetDocumentRequest) (*authztest.GetDocumentResponse, error) {
	doc := staticDocuments[in.DocumentId]
	return &authztest.GetDocumentResponse{
		Id:    doc.id,
		Title: doc.title,
		Body:  doc.body,
	}, nil
}

func (t *testServer) SaveDocument(ctx context.Context, in *authztest.SaveDocumentRequest) (*authztest.SaveDocumentResponse, error) {
	doc := staticDocuments[in.DocumentId]
	doc.title = in.Title
	doc.body = in.Body
	return &authztest.SaveDocumentResponse{
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
	"1": {id: "1", author: "3", title: "The Phoenix Saga", body: "A long time ago...", org: "xmen"},
	"2": {id: "2", author: "3", title: "The Dark Phoenix Saga", body: "A long time ago...", org: "xmen"},
	"3": {id: "3", author: "2", title: "Days of Future Past", body: "A long time ago...", org: "xmen"},
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