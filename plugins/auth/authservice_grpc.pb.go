// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.3
// source: plugins/auth/authservice.proto

package auth

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	AuthService_Login_FullMethodName    = "/prefab.auth.AuthService/Login"
	AuthService_Logout_FullMethodName   = "/prefab.auth.AuthService/Logout"
	AuthService_Config_FullMethodName   = "/prefab.auth.AuthService/Config"
	AuthService_Identity_FullMethodName = "/prefab.auth.AuthService/Identity"
)

// AuthServiceClient is the client API for AuthService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AuthServiceClient interface {
	// Login allows a client to provide credentials which can be used to
	// authenticate the client's identity. POST is preferred, some providers need
	// credentials to be passed as GET.
	Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error)
	// Logout clears the prefab id cookie. It should be noted that by default the
	// identity token will remain valid until its expiry. Token invalidatation is
	// supported via the addition of a blocklist.
	Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*LogoutResponse, error)
	// Config returns auth related configuration information that is safe for
	// unauthenticated users to access.
	Config(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (*ConfigResponse, error)
	// Identity returns information about the authenticated user.
	Identity(ctx context.Context, in *IdentityRequest, opts ...grpc.CallOption) (*IdentityResponse, error)
}

type authServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAuthServiceClient(cc grpc.ClientConnInterface) AuthServiceClient {
	return &authServiceClient{cc}
}

func (c *authServiceClient) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LoginResponse)
	err := c.cc.Invoke(ctx, AuthService_Login_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*LogoutResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LogoutResponse)
	err := c.cc.Invoke(ctx, AuthService_Logout_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) Config(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (*ConfigResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ConfigResponse)
	err := c.cc.Invoke(ctx, AuthService_Config_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) Identity(ctx context.Context, in *IdentityRequest, opts ...grpc.CallOption) (*IdentityResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(IdentityResponse)
	err := c.cc.Invoke(ctx, AuthService_Identity_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AuthServiceServer is the server API for AuthService service.
// All implementations must embed UnimplementedAuthServiceServer
// for forward compatibility.
type AuthServiceServer interface {
	// Login allows a client to provide credentials which can be used to
	// authenticate the client's identity. POST is preferred, some providers need
	// credentials to be passed as GET.
	Login(context.Context, *LoginRequest) (*LoginResponse, error)
	// Logout clears the prefab id cookie. It should be noted that by default the
	// identity token will remain valid until its expiry. Token invalidatation is
	// supported via the addition of a blocklist.
	Logout(context.Context, *LogoutRequest) (*LogoutResponse, error)
	// Config returns auth related configuration information that is safe for
	// unauthenticated users to access.
	Config(context.Context, *ConfigRequest) (*ConfigResponse, error)
	// Identity returns information about the authenticated user.
	Identity(context.Context, *IdentityRequest) (*IdentityResponse, error)
	mustEmbedUnimplementedAuthServiceServer()
}

// UnimplementedAuthServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedAuthServiceServer struct{}

func (UnimplementedAuthServiceServer) Login(context.Context, *LoginRequest) (*LoginResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Login not implemented")
}
func (UnimplementedAuthServiceServer) Logout(context.Context, *LogoutRequest) (*LogoutResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Logout not implemented")
}
func (UnimplementedAuthServiceServer) Config(context.Context, *ConfigRequest) (*ConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Config not implemented")
}
func (UnimplementedAuthServiceServer) Identity(context.Context, *IdentityRequest) (*IdentityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Identity not implemented")
}
func (UnimplementedAuthServiceServer) mustEmbedUnimplementedAuthServiceServer() {}
func (UnimplementedAuthServiceServer) testEmbeddedByValue()                     {}

// UnsafeAuthServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AuthServiceServer will
// result in compilation errors.
type UnsafeAuthServiceServer interface {
	mustEmbedUnimplementedAuthServiceServer()
}

func RegisterAuthServiceServer(s grpc.ServiceRegistrar, srv AuthServiceServer) {
	// If the following call pancis, it indicates UnimplementedAuthServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&AuthService_ServiceDesc, srv)
}

func _AuthService_Login_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_Login_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).Login(ctx, req.(*LoginRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_Logout_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogoutRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Logout(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_Logout_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).Logout(ctx, req.(*LogoutRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_Config_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Config(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_Config_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).Config(ctx, req.(*ConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_Identity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IdentityRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Identity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_Identity_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).Identity(ctx, req.(*IdentityRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// AuthService_ServiceDesc is the grpc.ServiceDesc for AuthService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AuthService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "prefab.auth.AuthService",
	HandlerType: (*AuthServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Login",
			Handler:    _AuthService_Login_Handler,
		},
		{
			MethodName: "Logout",
			Handler:    _AuthService_Logout_Handler,
		},
		{
			MethodName: "Config",
			Handler:    _AuthService_Config_Handler,
		},
		{
			MethodName: "Identity",
			Handler:    _AuthService_Identity_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "plugins/auth/authservice.proto",
}
