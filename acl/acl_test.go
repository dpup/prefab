package acl_test

import (
	"testing"

	"github.com/dpup/prefab/acl"
	"github.com/dpup/prefab/acl/acltest"

	"google.golang.org/grpc"
)

func Test_methodOptions(t *testing.T) {
	// Force proto-dependency for global registration.
	_ = acltest.UnimplementedAclTestServiceServer{}

	type args struct {
		info *grpc.UnaryServerInfo
	}
	tests := []struct {
		name              string
		args              args
		wantObjectKey     string
		wantAction        acl.Action
		wantDefaultEffect acl.Effect
	}{
		{
			name: "NoACL",
			args: args{
				info: &grpc.UnaryServerInfo{FullMethod: acltest.AclTestService_NoACL_FullMethodName},
			},
			wantObjectKey:     "*",
			wantAction:        "",
			wantDefaultEffect: acl.Deny,
		},
		{
			name: "ActionOnly",
			args: args{
				info: &grpc.UnaryServerInfo{FullMethod: acltest.AclTestService_ActionOnly_FullMethodName},
			},
			wantObjectKey:     "*",
			wantAction:        "action_only",
			wantDefaultEffect: acl.Deny,
		},
		{
			name: "GetDocument",
			args: args{
				info: &grpc.UnaryServerInfo{FullMethod: acltest.AclTestService_GetDocument_FullMethodName},
			},
			wantObjectKey:     "document",
			wantAction:        "documents.view",
			wantDefaultEffect: acl.Deny,
		},
		{
			name: "GetDocumentTitle",
			args: args{
				info: &grpc.UnaryServerInfo{FullMethod: acltest.AclTestService_GetDocumentTitle_FullMethodName},
			},
			wantObjectKey:     "document",
			wantAction:        "documents.view_meta",
			wantDefaultEffect: acl.Allow,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotObjectID, gotAction, gotEffect := acl.MethodOptions(tt.args.info)
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
	req := &acltest.GetDocumentRequest{
		DocumentId: "123",
		OrgId:      "nyc",
	}
	objectID, domainID, err := acl.FieldOptions(req)
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
