// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: server.proto

package prefab

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	anypb "google.golang.org/protobuf/types/known/anypb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Overrides the default error gateway error response to include a code_name
// for convenience.
type CustomErrorResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Code          int32                  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	CodeName      string                 `protobuf:"bytes,2,opt,name=code_name,json=codeName,proto3" json:"code_name,omitempty"`
	Message       string                 `protobuf:"bytes,3,opt,name=message,proto3" json:"message,omitempty"`
	Details       []*anypb.Any           `protobuf:"bytes,4,rep,name=details,proto3" json:"details,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CustomErrorResponse) Reset() {
	*x = CustomErrorResponse{}
	mi := &file_server_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CustomErrorResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CustomErrorResponse) ProtoMessage() {}

func (x *CustomErrorResponse) ProtoReflect() protoreflect.Message {
	mi := &file_server_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CustomErrorResponse.ProtoReflect.Descriptor instead.
func (*CustomErrorResponse) Descriptor() ([]byte, []int) {
	return file_server_proto_rawDescGZIP(), []int{0}
}

func (x *CustomErrorResponse) GetCode() int32 {
	if x != nil {
		return x.Code
	}
	return 0
}

func (x *CustomErrorResponse) GetCodeName() string {
	if x != nil {
		return x.CodeName
	}
	return ""
}

func (x *CustomErrorResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *CustomErrorResponse) GetDetails() []*anypb.Any {
	if x != nil {
		return x.Details
	}
	return nil
}

var file_server_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.MethodOptions)(nil),
		ExtensionType: (*string)(nil),
		Field:         50001,
		Name:          "prefab.csrf_mode",
		Tag:           "bytes,50001,opt,name=csrf_mode",
		Filename:      "server.proto",
	},
}

// Extension fields to descriptorpb.MethodOptions.
var (
	// Whether CSRF verification should be handled by a GRPC Interceptor.
	//
	// Possible values are "on", "off", and "auto".
	//
	// "auto" will enable CSRF checks unless a safe HTTP method is detected (GET,
	// HEAD, and OPTIONS) from the GRPC Gateway. If no HTTP method is detected, it
	// is assumed that the request came via a non-browser client and CSRF checks
	// are disabled.
	//
	// Defaults to "auto".
	//
	// optional string csrf_mode = 50001;
	E_CsrfMode = &file_server_proto_extTypes[0]
)

var File_server_proto protoreflect.FileDescriptor

var file_server_proto_rawDesc = string([]byte{
	0x0a, 0x0c, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06,
	0x70, 0x72, 0x65, 0x66, 0x61, 0x62, 0x1a, 0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x1a, 0x20, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x90, 0x01, 0x0a, 0x13, 0x43, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x45, 0x72,
	0x72, 0x6f, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x63,
	0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12,
	0x1b, 0x0a, 0x09, 0x63, 0x6f, 0x64, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x63, 0x6f, 0x64, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x2e, 0x0a, 0x07, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c,
	0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52, 0x07, 0x64,
	0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x3a, 0x3d, 0x0a, 0x09, 0x63, 0x73, 0x72, 0x66, 0x5f, 0x6d,
	0x6f, 0x64, 0x65, 0x12, 0x1e, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x4f, 0x70, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x18, 0xd1, 0x86, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x63, 0x73, 0x72,
	0x66, 0x4d, 0x6f, 0x64, 0x65, 0x42, 0x18, 0x5a, 0x16, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x64, 0x70, 0x75, 0x70, 0x2f, 0x70, 0x72, 0x65, 0x66, 0x61, 0x62, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_server_proto_rawDescOnce sync.Once
	file_server_proto_rawDescData []byte
)

func file_server_proto_rawDescGZIP() []byte {
	file_server_proto_rawDescOnce.Do(func() {
		file_server_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_server_proto_rawDesc), len(file_server_proto_rawDesc)))
	})
	return file_server_proto_rawDescData
}

var file_server_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_server_proto_goTypes = []any{
	(*CustomErrorResponse)(nil),        // 0: prefab.CustomErrorResponse
	(*anypb.Any)(nil),                  // 1: google.protobuf.Any
	(*descriptorpb.MethodOptions)(nil), // 2: google.protobuf.MethodOptions
}
var file_server_proto_depIdxs = []int32{
	1, // 0: prefab.CustomErrorResponse.details:type_name -> google.protobuf.Any
	2, // 1: prefab.csrf_mode:extendee -> google.protobuf.MethodOptions
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	1, // [1:2] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_server_proto_init() }
func file_server_proto_init() {
	if File_server_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_server_proto_rawDesc), len(file_server_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 1,
			NumServices:   0,
		},
		GoTypes:           file_server_proto_goTypes,
		DependencyIndexes: file_server_proto_depIdxs,
		MessageInfos:      file_server_proto_msgTypes,
		ExtensionInfos:    file_server_proto_extTypes,
	}.Build()
	File_server_proto = out.File
	file_server_proto_goTypes = nil
	file_server_proto_depIdxs = nil
}
