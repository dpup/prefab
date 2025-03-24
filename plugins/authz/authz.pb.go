// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: plugins/authz/authz.proto

package authz

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	reflect "reflect"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

var file_plugins_authz_authz_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.MethodOptions)(nil),
		ExtensionType: (*string)(nil),
		Field:         50011,
		Name:          "prefab.authz.action",
		Tag:           "bytes,50011,opt,name=action",
		Filename:      "plugins/authz/authz.proto",
	},
	{
		ExtendedType:  (*descriptorpb.MethodOptions)(nil),
		ExtensionType: (*string)(nil),
		Field:         50012,
		Name:          "prefab.authz.resource",
		Tag:           "bytes,50012,opt,name=resource",
		Filename:      "plugins/authz/authz.proto",
	},
	{
		ExtendedType:  (*descriptorpb.MethodOptions)(nil),
		ExtensionType: (*string)(nil),
		Field:         50013,
		Name:          "prefab.authz.default_effect",
		Tag:           "bytes,50013,opt,name=default_effect",
		Filename:      "plugins/authz/authz.proto",
	},
	{
		ExtendedType:  (*descriptorpb.FieldOptions)(nil),
		ExtensionType: (*bool)(nil),
		Field:         50021,
		Name:          "prefab.authz.id",
		Tag:           "varint,50021,opt,name=id",
		Filename:      "plugins/authz/authz.proto",
	},
	{
		ExtendedType:  (*descriptorpb.FieldOptions)(nil),
		ExtensionType: (*bool)(nil),
		Field:         50022,
		Name:          "prefab.authz.domain",
		Tag:           "varint,50022,opt,name=domain",
		Filename:      "plugins/authz/authz.proto",
	},
}

// Extension fields to descriptorpb.MethodOptions.
var (
	// optional string action = 50011;
	E_Action = &file_plugins_authz_authz_proto_extTypes[0]
	// optional string resource = 50012;
	E_Resource = &file_plugins_authz_authz_proto_extTypes[1]
	// optional string default_effect = 50013;
	E_DefaultEffect = &file_plugins_authz_authz_proto_extTypes[2]
)

// Extension fields to descriptorpb.FieldOptions.
var (
	// optional bool id = 50021;
	E_Id = &file_plugins_authz_authz_proto_extTypes[3]
	// optional bool domain = 50022;
	E_Domain = &file_plugins_authz_authz_proto_extTypes[4]
)

var File_plugins_authz_authz_proto protoreflect.FileDescriptor

var file_plugins_authz_authz_proto_rawDesc = string([]byte{
	0x0a, 0x19, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x2f, 0x61, 0x75, 0x74, 0x68, 0x7a, 0x2f,
	0x61, 0x75, 0x74, 0x68, 0x7a, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0c, 0x70, 0x72, 0x65,
	0x66, 0x61, 0x62, 0x2e, 0x61, 0x75, 0x74, 0x68, 0x7a, 0x1a, 0x20, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x65, 0x73, 0x63, 0x72,
	0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x3a, 0x38, 0x0a, 0x06, 0x61,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1e, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x4f, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xdb, 0x86, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x61,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x3a, 0x3c, 0x0a, 0x08, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63,
	0x65, 0x12, 0x1e, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x18, 0xdc, 0x86, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x3a, 0x47, 0x0a, 0x0e, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x5f, 0x65,
	0x66, 0x66, 0x65, 0x63, 0x74, 0x12, 0x1e, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x4f, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xdd, 0x86, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x64,
	0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x45, 0x66, 0x66, 0x65, 0x63, 0x74, 0x3a, 0x2f, 0x0a, 0x02,
	0x69, 0x64, 0x12, 0x1d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x18, 0xe5, 0x86, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x02, 0x69, 0x64, 0x3a, 0x37, 0x0a,
	0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x12, 0x1d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x4f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xe6, 0x86, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06,
	0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x42, 0x26, 0x5a, 0x24, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x64, 0x70, 0x75, 0x70, 0x2f, 0x70, 0x72, 0x65, 0x66, 0x61, 0x62,
	0x2f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x2f, 0x61, 0x75, 0x74, 0x68, 0x7a, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var file_plugins_authz_authz_proto_goTypes = []any{
	(*descriptorpb.MethodOptions)(nil), // 0: google.protobuf.MethodOptions
	(*descriptorpb.FieldOptions)(nil),  // 1: google.protobuf.FieldOptions
}
var file_plugins_authz_authz_proto_depIdxs = []int32{
	0, // 0: prefab.authz.action:extendee -> google.protobuf.MethodOptions
	0, // 1: prefab.authz.resource:extendee -> google.protobuf.MethodOptions
	0, // 2: prefab.authz.default_effect:extendee -> google.protobuf.MethodOptions
	1, // 3: prefab.authz.id:extendee -> google.protobuf.FieldOptions
	1, // 4: prefab.authz.domain:extendee -> google.protobuf.FieldOptions
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	0, // [0:5] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_plugins_authz_authz_proto_init() }
func file_plugins_authz_authz_proto_init() {
	if File_plugins_authz_authz_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_plugins_authz_authz_proto_rawDesc), len(file_plugins_authz_authz_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   0,
			NumExtensions: 5,
			NumServices:   0,
		},
		GoTypes:           file_plugins_authz_authz_proto_goTypes,
		DependencyIndexes: file_plugins_authz_authz_proto_depIdxs,
		ExtensionInfos:    file_plugins_authz_authz_proto_extTypes,
	}.Build()
	File_plugins_authz_authz_proto = out.File
	file_plugins_authz_authz_proto_goTypes = nil
	file_plugins_authz_authz_proto_depIdxs = nil
}
