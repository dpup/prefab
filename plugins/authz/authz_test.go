package authz_test

import (
	"testing"

	"github.com/dpup/prefab/plugins/authz"
	"github.com/dpup/prefab/plugins/authz/authztest"

	"google.golang.org/grpc"
)

func Test_methodOptions(t *testing.T) {
	// Force proto-dependency for global registration.
	_ = authztest.UnimplementedAuthzTestServiceServer{}

	type args struct {
		info *grpc.UnaryServerInfo
	}
	tests := []struct {
		name              string
		args              args
		wantObjectKey     string
		wantAction        authz.Action
		wantDefaultEffect authz.Effect
	}{
		{
			name: "NoPolicy",
			args: args{
				info: &grpc.UnaryServerInfo{FullMethod: authztest.AuthzTestService_NoPolicy_FullMethodName},
			},
			wantObjectKey:     "*",
			wantAction:        "",
			wantDefaultEffect: authz.Deny,
		},
		{
			name: "ActionOnly",
			args: args{
				info: &grpc.UnaryServerInfo{FullMethod: authztest.AuthzTestService_Self_FullMethodName},
			},
			wantObjectKey:     "*",
			wantAction:        "self.inspect",
			wantDefaultEffect: authz.Deny,
		},
		{
			name: "GetDocument",
			args: args{
				info: &grpc.UnaryServerInfo{FullMethod: authztest.AuthzTestService_GetDocument_FullMethodName},
			},
			wantObjectKey:     "document",
			wantAction:        "documents.view",
			wantDefaultEffect: authz.Deny,
		},
		{
			name: "GetDocumentTitle",
			args: args{
				info: &grpc.UnaryServerInfo{FullMethod: authztest.AuthzTestService_GetDocumentTitle_FullMethodName},
			},
			wantObjectKey:     "document",
			wantAction:        "documents.view_meta",
			wantDefaultEffect: authz.Allow,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotObjectID, gotAction, gotEffect := authz.MethodOptions(tt.args.info)
			if gotObjectID != tt.wantObjectKey {
				t.Errorf("methodOptions() got ObjectKey = %v, want %v", gotObjectID, tt.wantObjectKey)
			}
			if gotAction != tt.wantAction {
				t.Errorf("methodOptions() got Action = %v, want %v", gotAction, tt.wantAction)
			}
			if gotEffect != tt.wantDefaultEffect {
				t.Errorf("methodOptions() got DefaultEffect = %v, want %v", gotEffect, tt.wantDefaultEffect)
			}
		})
	}
}

func TestFieldOptions(t *testing.T) {
	req := &authztest.GetDocumentRequest{
		DocumentId: "123",
		OrgId:      "nyc",
	}
	objectID, domainID, err := authz.FieldOptions(req)
	if err != nil {
		t.Error(err)
	}
	if objectID != "123" {
		t.Errorf("FieldOptions() got ObjectID = %v, want 123", objectID)
	}
	if domainID != "nyc" {
		t.Errorf("FieldOptions() got DomainID = %v, want nyc", domainID)
	}
}
