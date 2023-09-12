package grpcserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/textproto"
	"os"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	defaultHost          = "0.0.0.0"
	defaultPort          = 8000
	defaultGatewayPrefix = "/v1/"

	// GRPC Metadata prefix that is added to allowed headers specified with
	// WithIncomingHeaders.
	MetadataPrefix = "prefab-"
)

// ServerOptions customize the configuration and operation of the GRPC server.
type ServerOption func(*builder)

type handler struct {
	prefix  string
	handler http.Handler
}

// New returns a new server.
func New(opts ...ServerOption) *Server {
	b := &builder{
		host:          defaultHost,
		port:          defaultPort,
		gatewayPrefix: defaultGatewayPrefix,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b.build()
}

type builder struct {
	host            string
	port            int
	corsOrigins     []string
	incomingHeaders []string
	gatewayPrefix   string
	certFile        string
	keyFile         string
	maxMsgSizeBytes int
	httpHandlers    []handler
	interceptors    []grpc.UnaryServerInterceptor
}

func (b *builder) build() *Server {
	gatewayOpts := b.buildGatewayOpts()
	gateway := runtime.NewServeMux(
		// Override default JSON marshaler so that 0, false, and "" are emitted as
		// actual values rather than undefined. This allows for better handling of
		// PB wrapper types that allow for true, false, null.
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				Multiline:       true,
				Indent:          "  ",
				EmitUnpopulated: true,
			},
		}),

		// Patch error responses to include a codeName for easier client handling.
		runtime.WithErrorHandler(gatewayErrorHandler),

		// TODO: Add support for form encoded data out of the box.
		// runtime.WithMarshalerOption("application/x-www-form-urlencoded", &FormMarshaler{}),

		// Support for standard headers plus propriety Productable headers.
		runtime.WithIncomingHeaderMatcher(b.buildGatewayHeaderMatcher()),

		// By default standard headers and metadata prefixed with Grpc-Metadata-
		// will be propagated over HTTP.
		runtime.WithOutgoingHeaderMatcher(runtime.DefaultHeaderMatcher),
	)

	s := &Server{
		baseContext: context.Background(),
		host:        b.host,
		port:        b.port,
		certFile:    b.certFile,
		keyFile:     b.keyFile,
		httpMux:     http.NewServeMux(),
		grpcServer:  grpc.NewServer(b.buildGRPCOpts()...),
		gatewayOpts: gatewayOpts,
		grpcGateway: gateway,
	}

	s.httpMux.Handle(b.gatewayPrefix, b.wrapHandler(http.Handler(gateway)))
	for _, h := range b.httpHandlers {
		s.httpMux.Handle(h.prefix, b.wrapHandler(h.handler))
	}

	return s
}

func (b *builder) wrapHandler(h http.Handler) http.Handler {
	if len(b.corsOrigins) == 0 {
		// If there are no allowed origins configured, disable CORS headers completely.
		return h
	}
	allowed := map[string]bool{}
	for _, origin := range b.corsOrigins {
		allowed[origin] = true
	}
	allowedHeaders := strings.Join(b.incomingHeaders, ", ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowed[r.Header.Get("Origin")] {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		}
		if r.Method == "OPTIONS" {
			return // Just the headers.
		}
		h.ServeHTTP(w, r) // Handle the request.
	})
}

func (b *builder) buildGRPCOpts() []grpc.ServerOption {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(b.interceptors...)),
	}
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

func (b *builder) buildGatewayHeaderMatcher() func(string) (string, bool) {
	headerMap := map[string]bool{}
	for _, h := range b.incomingHeaders {
		headerMap[h] = true
	}
	return func(key string) (string, bool) {
		key = textproto.CanonicalMIMEHeaderKey(key)
		if headerMap[key] {
			return MetadataPrefix + key, true
		}
		return runtime.DefaultHeaderMatcher(key)
	}
}

func (b *builder) isSecure() bool {
	return b.certFile != "" && b.keyFile != ""
}

// WithHost configures the hostname or IP the server will listen on.
func WithHost(host string) ServerOption {
	return func(b *builder) {
		b.host = host
	}
}

// WithPort configures the port the server will listen on.
func WithPort(port int) ServerOption {
	return func(b *builder) {
		b.port = port
	}
}

// WithCORSAllowedOrigins specifies origins that are allowed to make requests.
// See https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
func WithCORSAllowedOrigins(origins ...string) ServerOption {
	return func(b *builder) {
		b.corsOrigins = append(b.corsOrigins, origins...)
	}
}

// WithIncomingHeaders specifies a safe-list of headers that can be forwarded
// via CORS and made available in as GRPC metadata with the `prefab` prefix.
func WithIncomingHeaders(headers ...string) ServerOption {
	return func(b *builder) {
		b.incomingHeaders = append(b.incomingHeaders, headers...)
	}
}

// WithTLS configures the server to allow traffic via TLS using the provided
// cert. If not called server will use HTTP/H2C.
func WithTLS(certFile, keyFile string) ServerOption {
	return func(b *builder) {
		b.certFile = certFile
		b.keyFile = keyFile
	}
}

// WithMaxRecvMsgSize sets the maximum GRPC message size. Default is 4Mb.
func WithMaxRecvMsgSize(maxMsgSizeBytes int) ServerOption {
	return func(b *builder) {
		b.maxMsgSizeBytes = maxMsgSizeBytes
	}
}

// WithGatewayPrefix sets the path prefix that the GRPC Gateway will be bound
// to. Default is `/v1/`.
func WithGatewayPrefix(prefix string) ServerOption {
	return func(b *builder) {
		b.gatewayPrefix = prefix
	}
}

// WithStaticFileServer configures the server to serve static files from disk
// for HTTP requests that match the given prefix.
func WithStaticFiles(prefix, dir string) ServerOption {
	return func(b *builder) {
		b.httpHandlers = append(b.httpHandlers, handler{
			prefix:  prefix,
			handler: http.FileServer(http.Dir(dir)),
		})
	}
}

// WithHTTPHandler adds an HTTP handler.
func WithHTTPHandler(prefix string, h http.Handler) ServerOption {
	return func(b *builder) {
		b.httpHandlers = append(b.httpHandlers, handler{
			prefix:  prefix,
			handler: h,
		})
	}
}

// WithHTTPHandlerFunc adds an HTTP handler function.
func WithHTTPHandlerFunc(prefix string, h func(http.ResponseWriter, *http.Request)) ServerOption {
	return func(b *builder) {
		b.httpHandlers = append(b.httpHandlers, handler{
			prefix:  prefix,
			handler: http.HandlerFunc(h),
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

// Creates credentials from a cert and key file.
// Based on credentials.NewServerTLSFromFile
func serverTLSFromFile(cert, key string) credentials.TransportCredentials {
	c, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		panic(err)
	}
	tlsConfig := safeTLSConfig()
	tlsConfig.Certificates = []tls.Certificate{c}
	return credentials.NewTLS(tlsConfig)
}

// Based on credentials.NewClientTLSFromFile
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
