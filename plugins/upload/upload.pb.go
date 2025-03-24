// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: plugins/upload/upload.proto

package upload

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
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

// Response from calling the upload endpoint.
type UploadResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Files         []*UploadedFile        `protobuf:"bytes,1,rep,name=files,proto3" json:"files,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *UploadResponse) Reset() {
	*x = UploadResponse{}
	mi := &file_plugins_upload_upload_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UploadResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UploadResponse) ProtoMessage() {}

func (x *UploadResponse) ProtoReflect() protoreflect.Message {
	mi := &file_plugins_upload_upload_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UploadResponse.ProtoReflect.Descriptor instead.
func (*UploadResponse) Descriptor() ([]byte, []int) {
	return file_plugins_upload_upload_proto_rawDescGZIP(), []int{0}
}

func (x *UploadResponse) GetFiles() []*UploadedFile {
	if x != nil {
		return x.Files
	}
	return nil
}

type UploadedFile struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The original filename of the uploaded file, so that the client can match
	// the uploaded file with the file that was uploaded.
	OriginalName string `protobuf:"bytes,1,opt,name=original_name,json=originalName,proto3" json:"original_name,omitempty"`
	// The path where the file was uploaded to.
	UploadPath    string `protobuf:"bytes,2,opt,name=upload_path,json=uploadPath,proto3" json:"upload_path,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *UploadedFile) Reset() {
	*x = UploadedFile{}
	mi := &file_plugins_upload_upload_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UploadedFile) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UploadedFile) ProtoMessage() {}

func (x *UploadedFile) ProtoReflect() protoreflect.Message {
	mi := &file_plugins_upload_upload_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UploadedFile.ProtoReflect.Descriptor instead.
func (*UploadedFile) Descriptor() ([]byte, []int) {
	return file_plugins_upload_upload_proto_rawDescGZIP(), []int{1}
}

func (x *UploadedFile) GetOriginalName() string {
	if x != nil {
		return x.OriginalName
	}
	return ""
}

func (x *UploadedFile) GetUploadPath() string {
	if x != nil {
		return x.UploadPath
	}
	return ""
}

var File_plugins_upload_upload_proto protoreflect.FileDescriptor

var file_plugins_upload_upload_proto_rawDesc = string([]byte{
	0x0a, 0x1b, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x2f, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64,
	0x2f, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x70,
	0x72, 0x65, 0x66, 0x61, 0x62, 0x2e, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x22, 0x43, 0x0a, 0x0e,
	0x55, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x31,
	0x0a, 0x05, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1b, 0x2e,
	0x70, 0x72, 0x65, 0x66, 0x61, 0x62, 0x2e, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x2e, 0x55, 0x70,
	0x6c, 0x6f, 0x61, 0x64, 0x65, 0x64, 0x46, 0x69, 0x6c, 0x65, 0x52, 0x05, 0x66, 0x69, 0x6c, 0x65,
	0x73, 0x22, 0x54, 0x0a, 0x0c, 0x55, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x65, 0x64, 0x46, 0x69, 0x6c,
	0x65, 0x12, 0x23, 0x0a, 0x0d, 0x6f, 0x72, 0x69, 0x67, 0x69, 0x6e, 0x61, 0x6c, 0x5f, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x6f, 0x72, 0x69, 0x67, 0x69, 0x6e,
	0x61, 0x6c, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64,
	0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x75, 0x70, 0x6c,
	0x6f, 0x61, 0x64, 0x50, 0x61, 0x74, 0x68, 0x42, 0x27, 0x5a, 0x25, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x64, 0x70, 0x75, 0x70, 0x2f, 0x70, 0x72, 0x65, 0x66, 0x61,
	0x62, 0x2f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x2f, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_plugins_upload_upload_proto_rawDescOnce sync.Once
	file_plugins_upload_upload_proto_rawDescData []byte
)

func file_plugins_upload_upload_proto_rawDescGZIP() []byte {
	file_plugins_upload_upload_proto_rawDescOnce.Do(func() {
		file_plugins_upload_upload_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_plugins_upload_upload_proto_rawDesc), len(file_plugins_upload_upload_proto_rawDesc)))
	})
	return file_plugins_upload_upload_proto_rawDescData
}

var file_plugins_upload_upload_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_plugins_upload_upload_proto_goTypes = []any{
	(*UploadResponse)(nil), // 0: prefab.upload.UploadResponse
	(*UploadedFile)(nil),   // 1: prefab.upload.UploadedFile
}
var file_plugins_upload_upload_proto_depIdxs = []int32{
	1, // 0: prefab.upload.UploadResponse.files:type_name -> prefab.upload.UploadedFile
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_plugins_upload_upload_proto_init() }
func file_plugins_upload_upload_proto_init() {
	if File_plugins_upload_upload_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_plugins_upload_upload_proto_rawDesc), len(file_plugins_upload_upload_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_plugins_upload_upload_proto_goTypes,
		DependencyIndexes: file_plugins_upload_upload_proto_depIdxs,
		MessageInfos:      file_plugins_upload_upload_proto_msgTypes,
	}.Build()
	File_plugins_upload_upload_proto = out.File
	file_plugins_upload_upload_proto_goTypes = nil
	file_plugins_upload_upload_proto_depIdxs = nil
}
