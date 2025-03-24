// An example of how to use the Authz plugin with the common builder pattern.
//
// $ go run examples/authz/main.go -example=common-builder
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
package commonbuilder

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

// Document implements the GetOwnerID method required by the common role describer
type document struct {
	id     string
	author string
	title  string
	body   string
}

// GetOwnerID returns the author ID which is considered the owner
func (d document) GetOwnerID() string {
	return d.author
}

// Run starts the common builder example server
func Run() {
	// Define custom actions that map to standard CRUD operations
	docList := authz.Action("documents.list")     // similar to authz.ActionList 
	docView := authz.Action("documents.view")     // similar to authz.ActionRead
	docViewMeta := authz.Action("documents.view_meta") // similar to authz.ActionRead
	docWrite := authz.Action("documents.write")    // similar to authz.ActionUpdate
	
	// Create the authz plugin using the CRUD builder pattern with custom actions
	builder := authz.NewCRUDBuilder()
	
	// Map our domain-specific actions to standard CRUD operations
	builder.WithPolicy(authz.Allow, authz.RoleViewer, docList)
	builder.WithPolicy(authz.Allow, authz.RoleViewer, docView)
	builder.WithPolicy(authz.Allow, authz.RoleViewer, docViewMeta)
	builder.WithPolicy(authz.Allow, authz.RoleEditor, docWrite)
	
	// Define our object fetchers
	builder.WithObjectFetcher("document", fetchDocument)
	
	// Define a custom role describer
	builder.WithRoleDescriber("*", func(ctx context.Context, identity auth.Identity, object any, domain authz.Domain) ([]authz.Role, error) {
		// Add domain-specific logic if needed
		if domain != "xmen" {
			return []authz.Role{}, nil
		}
		
		// Base role for any authenticated user
		roles := []authz.Role{authz.RoleUser}
		
		// Admin role for specific users
		if identity.Email == "logan@xmen.net" {
			roles = append(roles, authz.RoleAdmin)
			// Admin already has all permissions, no need to check other roles
			return roles, nil
		}
		
		// Editor role for specific users
		if identity.Email == "jean@xmen.net" {
			roles = append(roles, authz.RoleEditor)
		}
		
		// Viewer role for specific users
		if identity.Email == "scott@xmen.net" || 
		   identity.Email == "ororo@xmen.net" || 
		   identity.Email == "kurt@xmen.net" {
			roles = append(roles, authz.RoleViewer)
		}
		
		// Check if user is the owner of the document
		if owner, ok := object.(interface{ GetOwnerID() string }); ok {
			if owner.GetOwnerID() == identity.Subject {
				roles = append(roles, authz.RoleOwner)
			}
		}
		
		return roles, nil
	})
	
	// Build the plugin
	authzPlugin := builder.Build()

	s := prefab.New(
		// Configure authentication
		prefab.WithPlugin(auth.Plugin()),
		prefab.WithPlugin(pwdauth.Plugin(
			pwdauth.WithAccountFinder(accountStore{}),
			pwdauth.WithHasher(pwdauth.TestHasher),
		)),

		// Add our authz plugin
		prefab.WithPlugin(authzPlugin),
	)

	// Register the GRPC service
	authztest.RegisterAuthzTestServiceHandlerFromEndpoint(s.GatewayArgs())
	authztest.RegisterAuthzTestServiceServer(s.ServiceRegistrar(), &testServer{})

	// Start the server
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

// Object fetcher for document resource
func fetchDocument(ctx context.Context, key any) (any, error) {
	if doc, ok := staticDocuments[key.(string)]; ok {
		return doc, nil
	}
	return nil, errors.NewC("document not found", codes.NotFound)
}

// Basic implementation of the test server
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

// Account store implementation
type accountStore struct{}

func (a accountStore) FindAccount(ctx context.Context, email string) (*pwdauth.Account, error) {
	for _, acc := range staticAccounts {
		if acc.Email == email {
			return acc, nil
		}
	}
	return nil, errors.Codef(codes.NotFound, "account not found")
}

// Static data
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