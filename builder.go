package prefab

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"strconv"

	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/serverutil"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ServerOptions customize the configuration and operation of the GRPC server.
type ServerOption func(*builder)

type handler struct {
	prefix      string
	httpHandler http.Handler
	jsonHandler JSONHandler
}

// Options passed to runtime.JSONPb when building the server.
var JSONMarshalOptions = protojson.MarshalOptions{
	Multiline:       true,
	Indent:          "  ",
	EmitUnpopulated: true,
	UseProtoNames:   false,
}

// New returns a new server.
func New(opts ...ServerOption) *Server {
	b := &builder{
		host:            Config.String("server.host"),
		port:            Config.Int("server.port"),
		incomingHeaders: Config.Strings("server.incomingHeaders"),
		certFile:        Config.String("server.tls.certFile"),
		keyFile:         Config.String("server.tls.keyFile"),
		maxMsgSizeBytes: Config.Int("server.maxMsgSizeBytes"),
		csrfSigningKey:  []byte(Config.String("server.csrfSigningKey")),
		securityHeaders: &SecurityHeaders{
			XFramesOptions:        XFramesOptions(Config.String("server.security.xFramesOptions")),
			HSTSExpiration:        Config.Duration("server.security.hstsExpiration"),
			HSTSIncludeSubdomains: Config.Bool("server.security.hstsIncludeSubdomains"),
			HSTSPreload:           Config.Bool("server.security.hstsPreload"),
			CORSOrigins:           Config.Strings("server.security.corsOrigins"),
			CORSAllowMethods:      Config.Strings("server.security.corsAllowMethods"),
			CORSAllowHeaders:      Config.Strings("server.security.corsAllowHeaders"),
			CORSExposeHeaders:     Config.Strings("server.security.corsExposeHeaders"),
			CORSAllowCredentials:  Config.Bool("server.security.corsAllowCredentials"),
			CORSMaxAge:            Config.Duration("server.security.corsMaxAge"),
		},

		plugins: &Registry{},
	}
	for _, opt := range opts {
		opt(b)
	}

	// Add the CSRF header to the safe-lists.
	b.incomingHeaders = append(b.incomingHeaders, csrfHeader)

	// Add headers from CORS allow-list to propagate to the gRPC server. (Dupes don't matter)
	b.incomingHeaders = append(b.incomingHeaders, b.securityHeaders.CORSAllowHeaders...)

	return b.build()
}

type builder struct {
	baseContext     context.Context
	host            string
	port            int
	incomingHeaders []string
	certFile        string
	keyFile         string
	maxMsgSizeBytes int
	csrfSigningKey  []byte
	securityHeaders *SecurityHeaders

	plugins *Registry

	handlers        []handler
	interceptors    []grpc.UnaryServerInterceptor
	serverBuilders  []func(s *Server)
	configInjectors []ConfigInjector
	clientConfigs   map[string]string
}

func (b *builder) build() *Server {
	if b.baseContext == nil {
		b.baseContext = context.Background()
	}

	gatewayOpts := b.buildGatewayOpts()
	gateway := runtime.NewServeMux(
		// Override default JSON marshaler so that 0, false, and "" are emitted as
		// actual values rather than undefined. This allows for better handling of
		// PB wrapper types that allow for true, false, null.
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: JSONMarshalOptions,
		}),

		// Map CSRF query param to metadata.
		runtime.WithMetadata(csrfMetadataAnnotator),

		// Map request fields to metadata.
		runtime.WithMetadata(serverutil.HttpMetadataAnnotator),

		// Forward custom HTTP status codes for GRPC responses.
		runtime.WithForwardResponseOption(statusCodeForwarder),

		// Patch error responses to include a codeName for easier client handling.
		runtime.WithErrorHandler(gatewayErrorHandler),

		// Support form encoded payloads.
		runtime.WithMarshalerOption("application/x-www-form-urlencoded", &formDecoder{}),

		// Support for standard headers plus propriety application headers.
		runtime.WithIncomingHeaderMatcher(serverutil.HeaderMatcher(b.incomingHeaders)),

		// By default standard headers and metadata prefixed with Grpc-Metadata-
		// will be propagated over HTTP.
		runtime.WithOutgoingHeaderMatcher(runtime.DefaultHeaderMatcher),
	)

	// Ensure that a logger is available.
	ctx := logging.EnsureLogger(b.baseContext)

	s := &Server{
		baseContext: ctx,
		host:        b.host,
		port:        b.port,
		certFile:    b.certFile,
		keyFile:     b.keyFile,
		httpMux:     http.NewServeMux(),
		grpcServer:  grpc.NewServer(b.buildGRPCOpts()...),
		gatewayOpts: gatewayOpts,
		grpcGateway: gateway,
		plugins:     b.plugins,
	}

	for _, fn := range b.serverBuilders {
		fn(s)
	}

	s.httpMux.Handle("/api/", securityMiddleware(http.Handler(gateway), b.securityHeaders))
	for _, h := range b.handlers {
		var handler http.Handler
		if h.jsonHandler != nil {
			handler = wrapJSONHandler(h.jsonHandler)
		} else {
			handler = h.httpHandler
		}
		handler = httpContextMiddleware(handler, b.configInjectors, gateway)
		handler = securityMiddleware(handler, b.securityHeaders)
		s.httpMux.Handle(h.prefix, handler)
	}

	// Register the metaservice last so that it can see all the client configs.
	m := &meta{configs: b.clientConfigs, csrfSigningKey: b.csrfSigningKey}
	s.ServiceRegistrar().RegisterService(&MetaService_ServiceDesc, m)
	_ = RegisterMetaServiceHandlerFromEndpoint(s.GatewayArgs())

	return s
}
func (b *builder) buildGRPCOpts() []grpc.ServerOption {
	interceptors := append(
		[]grpc.UnaryServerInterceptor{
			configInterceptor(b.configInjectors),
			logging.Interceptor(),
			csrfInterceptor(b.csrfSigningKey),
		},
		b.interceptors...)
	opts := []grpc.ServerOption{grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(interceptors...))}
	if b.isSecure() {
		opts = append(opts, grpc.Creds(serverTLSFromFile(b.certFile, b.keyFile)))
	}
	if b.maxMsgSizeBytes > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(b.maxMsgSizeBytes))
	}
	return opts
}

func (b *builder) buildGatewayOpts() []grpc.DialOption {
	opts := []grpc.DialOption{}
	if b.isSecure() {
		opts = append(opts, grpc.WithTransportCredentials(clientTLSFromFile(b.certFile)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	// TODO: Allow a local connection for testing.
	return opts
}

func (b *builder) isSecure() bool {
	return b.certFile != "" && b.keyFile != ""
}

// WithContext sets the base context for the server. This context will be used
// for all requests and can be used to inject values into the context.
func WithContext(ctx context.Context) ServerOption {
	return func(b *builder) {
		b.baseContext = ctx
	}
}

// WithHost configures the hostname or IP the server will listen on.
//
// Config key: `server.host`.
func WithHost(host string) ServerOption {
	return func(b *builder) {
		b.host = host
	}
}

// WithPort configures the port the server will listen on.
//
// Config key: `server.port`.
func WithPort(port int) ServerOption {
	return func(b *builder) {
		b.port = port
	}
}

// WithIncomingHeaders specifies a safe-list of headers that can be forwarded
// via gRPC metadata with the `prefab` prefix. Headers that are allowed by
// the CORS security config are automatically added to this list,
// see WithSecurityHeaders.
//
// Config key: `server.incomingHeaders`.
func WithIncomingHeaders(headers ...string) ServerOption {
	return func(b *builder) {
		b.incomingHeaders = append(b.incomingHeaders, headers...)
	}
}

// WithTLS configures the server to allow traffic via TLS using the provided
// cert. If not called server will use HTTP/H2C.
//
// Config keys: `server.tls.certFile`, `server.tls.keyFile`.
func WithTLS(certFile, keyFile string) ServerOption {
	return func(b *builder) {
		b.certFile = certFile
		b.keyFile = keyFile
	}
}

// WithMaxRecvMsgSize sets the maximum GRPC message size. Default is 4Mb.
//
// Config key: `server.maxMsgSizeBytes`.
func WithMaxRecvMsgSize(maxMsgSizeBytes int) ServerOption {
	return func(b *builder) {
		b.maxMsgSizeBytes = maxMsgSizeBytes
	}
}

// WithCRSFSigningKey sets the key used to sign CSRF tokens.
//
// Config key: `server.csrfSigningKey`.
func WithCRSFSigningKey(signingKey string) ServerOption {
	return func(b *builder) {
		b.csrfSigningKey = []byte(signingKey)
	}
}

// WithSecurityHeaders sets the security headers that should be set on HTTP
// responses.
//
// Config keys:
// - `server.security.xFramesOptions`
// - `server.security.hstsExpiration`
// - `server.security.hstsIncludeSubdomains`
// - `server.security.hstsPreload`
// - `server.security.corsOrigins`
// - `server.security.corsAllowMethods`
// - `server.security.corsAllowHeaders`
// - `server.security.corsExposeHeaders`
// - `server.security.corsAllowCredentials`
// - `server.security.corsMaxAge`.
func WithSecurityHeaders(headers *SecurityHeaders) ServerOption {
	return func(b *builder) {
		b.securityHeaders = headers
	}
}

// WithStaticFileServer configures the server to serve static files from disk
// for HTTP requests that match the given prefix.
func WithStaticFiles(prefix, dir string) ServerOption {
	return func(b *builder) {
		b.handlers = append(b.handlers, handler{
			prefix:      prefix,
			httpHandler: http.FileServer(http.Dir(dir)),
		})
	}
}

// WithHTTPHandler adds an HTTP handler.
func WithHTTPHandler(prefix string, h http.Handler) ServerOption {
	return func(b *builder) {
		b.handlers = append(b.handlers, handler{
			prefix:      prefix,
			httpHandler: h,
		})
	}
}

// WithHTTPHandlerFunc adds an HTTP handler function.
func WithHTTPHandlerFunc(prefix string, h func(http.ResponseWriter, *http.Request)) ServerOption {
	return func(b *builder) {
		b.handlers = append(b.handlers, handler{
			prefix:      prefix,
			httpHandler: http.HandlerFunc(h),
		})
	}
}

// WithJSONHandler adds a HTTP handler which returns JSON, serialized in a
// consistent way to gRPC gateway responses.
func WithJSONHandler(prefix string, h JSONHandler) ServerOption {
	return func(b *builder) {
		b.handlers = append(b.handlers, handler{
			prefix:      prefix,
			jsonHandler: h,
		})
	}
}

// WithGRPCInterceptor configures GRPC Unary Interceptors. They will be executed
// in the order they were added.
func WithGRPCInterceptor(interceptor grpc.UnaryServerInterceptor) ServerOption {
	return func(b *builder) {
		b.interceptors = append(b.interceptors, interceptor)
	}
}

// WithGRPCService registers a GRPC service handler.
func WithGRPCService(desc *grpc.ServiceDesc, impl any) ServerOption {
	return func(b *builder) {
		b.serverBuilders = append(b.serverBuilders, func(s *Server) {
			s.ServiceRegistrar().RegisterService(desc, impl)
		})
	}
}

// WithGRPCReflection registers the GRPC reflection service.
func WithGRPCReflection() ServerOption {
	return func(b *builder) {
		b.serverBuilders = append(b.serverBuilders, func(s *Server) {
			reflection.Register(s.GRPCServerForReflection())
		})
	}
}

// WithGRPCGateway registers a GRPC gateway handler.
//
// Example:
//
//	WithGRPCGateway(debugservice.RegisterDebugServiceHandlerFromEndpoint)
func WithGRPCGateway(fn func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error) ServerOption {
	return func(b *builder) {
		b.serverBuilders = append(b.serverBuilders, func(s *Server) {
			err := fn(s.GatewayArgs())
			if err != nil {
				panic(err)
			}
		})
	}
}

// WithPlugin registers a plugin with the server's registry. Plugins will be
// initialized at server start. If the Plugin implements `OptionProvider` then
// additional server options can be configured for the server.
func WithPlugin(p Plugin) ServerOption {
	return func(b *builder) {
		if so, ok := p.(OptionProvider); ok {
			for _, opt := range so.ServerOptions() {
				opt(b)
			}
		}
		b.plugins.Register(p)
	}
}

// WithClientConfig adds a key value pair which will be made available to the
// client via the metaservice.
func WithClientConfig(key, value string) ServerOption {
	return func(b *builder) {
		if b.clientConfigs == nil {
			b.clientConfigs = map[string]string{}
		}
		b.clientConfigs[key] = value
	}
}

// WithRequestConfig adds a ConfigInjector to the server. The injector will be
// called for every request and can be used to inject request scoped
// configuration into the context.
func WithRequestConfig(injector ConfigInjector) ServerOption {
	return func(b *builder) {
		b.configInjectors = append(b.configInjectors, injector)
	}
}

// Creates credentials from a cert and key file.
// Based on credentials.NewServerTLSFromFile.
func serverTLSFromFile(cert, key string) credentials.TransportCredentials {
	c, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		panic(err)
	}
	tlsConfig := safeTLSConfig()
	tlsConfig.Certificates = []tls.Certificate{c}
	return credentials.NewTLS(tlsConfig)
}

// Based on credentials.NewClientTLSFromFile.
func clientTLSFromFile(cert string) credentials.TransportCredentials {
	b, err := os.ReadFile(cert)
	if err != nil {
		panic(err)
	}
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(b) {
		panic("Failed to append credentials")
	}
	tlsConfig := safeTLSConfig()
	tlsConfig.RootCAs = cp
	return credentials.NewTLS(tlsConfig)
}

// TLS1.2 min and support for HTTP2.
func safeTLSConfig() *tls.Config {
	return &tls.Config{
		NextProtos: []string{"h2"},
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
	}
}

// Taken from example code here:
// https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/customizing_your_gateway/#controlling-http-response-status-codes
func statusCodeForwarder(ctx context.Context, w http.ResponseWriter, p proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return nil
	}

	if values := md.HeaderMD.Get("x-http-code"); len(values) > 0 {
		code, err := strconv.Atoi(values[0])
		if err != nil {
			return err
		}
		// Delete the headers to not expose any grpc-metadata in http response
		delete(md.HeaderMD, "x-http-code")
		delete(w.Header(), "Grpc-Metadata-X-Http-Code")
		w.WriteHeader(code)
	}
	return nil
}

// OptionProvider can be implemented by plugins to augment the server at build
// time.
type OptionProvider interface {
	ServerOptions() []ServerOption
}

// For HTTP only requests, annotates the context with configs and with gRPC
// metadata so that HTTP handlers can call downstream gRPC services directly.
func httpContextMiddleware(h http.Handler, cf []ConfigInjector, gateway *runtime.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = injectConfigs(ctx, cf)

		// TODO: Is this worth specifying? It is read via runtime.RPCMethod()
		name := "HttpHandler"

		// Extract information from the request and map it to GRPC metadata,
		// mirroring the approach of the gRPC Gateway so that HTTP handlers can call
		// downstream gRPC services directly and have HTTP headers forwarded.
		ctx, err := runtime.AnnotateContext(ctx, gateway, r, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logging.Errorw(r.Context(), "Failed to annotate context", "error", err)
			return
		}

		// The incoming context is also annotated, so that prefab utility functions
		// can be use from within the HTTP handlers as well as gRPC handlers.
		ctx, err = runtime.AnnotateIncomingContext(ctx, gateway, r, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logging.Errorw(r.Context(), "Failed to annotate context", "error", err)
			return
		}

		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
