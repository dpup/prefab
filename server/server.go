// Package server provides common helpers to streamline the initialization
// and configuration of a typical hybrid web server, which can handle GRPC
// JSON RPC, and regular HTTP handlers.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/dpup/prefab/logging"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

// Server wraps a HTTP server, a GRPC server, and a GRPC Gateway.
//
// Usage:
//
//	server := grpcserver.New(opts...)
//	debugservice.RegisterDebugServiceHandlerFromEndpoint(server.GatewayArgs())
//	debugservice.RegisterDebugServiceServer(server.ServiceRegistrar(), &impl{})
//	server.Start()
//
// See examples/simpleserver.
type Server struct {
	// Hostname or IP to bind to.
	host string

	// Port to listen on.
	port int

	// Location of certificate file, if TLS to be used.
	certFile string

	// Location of key file, if TLS to be used.
	keyFile string

	// Context that is propagated to gateway handlers.
	baseContext context.Context

	// Handles original request and multiplexes to grpcServer or httpMux.
	httpServer *http.Server

	// Handles regular HTTP requests.
	httpMux *http.ServeMux

	// Handles GRPC requests of content-type application/grpc.
	grpcServer *grpc.Server

	// Bound to httpMux and exposes GRPC services as JSON/REST.
	grpcGateway *runtime.ServeMux

	// DialOptions passed when registering GRPC Gateway handlers.
	gatewayOpts []grpc.DialOption
}

// GRPCServer returns the GRPC Service Registrar for use with service
// implementations.
//
// For example, if you have DebugService:
//
//	debugservice.RegisterDebugServiceServer(server.ServiceRegistrar(), &debugServiceImpl{})
func (s *Server) ServiceRegistrar() grpc.ServiceRegistrar {
	return s.grpcServer
}

// GatewayArgs is used when registering a gateway handler.
//
// For example, if you have DebugService:
//
//	debugservice.RegisterDebugServiceHandlerFromEndpoint(server.GatewayArgs())
func (s *Server) GatewayArgs() (ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) {
	ctx = s.baseContext
	mux = s.grpcGateway
	opts = s.gatewayOpts
	if s.host == "0.0.0.0" {
		// Special case of 0.0.0.0 is a listen-only IP, and must be changed into
		// localhost in a containerized environment.
		endpoint = fmt.Sprintf("localhost:%d", s.port)
	} else {
		endpoint = fmt.Sprintf("%s:%d", s.host, s.port)
	}
	return
}

// Start serving requests. Blocks until Shutdown is called.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.httpServer = &http.Server{
		Addr: addr,
		BaseContext: func(listener net.Listener) context.Context {
			return s.baseContext
		},
	}

	var done = make(chan struct{})
	var err error

	go func() {
		var gracefulStop = make(chan os.Signal, 1)
		signal.Notify(gracefulStop, syscall.SIGTERM)
		signal.Notify(gracefulStop, syscall.SIGINT)
		sig := <-gracefulStop
		logging.Infof(s.baseContext, "👋 Graceful shutdown triggered... (sig %+v)\n", sig)
		s.Shutdown()
		close(done)
	}()

	// TODO: Allow bufconn to be injected to allow tests to avoid the network.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	defer ln.Close()

	grpcHandler := s.grpcServer
	httpHandler := gziphandler.GzipHandler(s.httpMux)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcHandler.ServeHTTP(w, r)
		} else {
			httpHandler.ServeHTTP(w, r)
		}
	})

	if s.certFile != "" {
		s.httpServer.Handler = handler
		s.httpServer.TLSConfig = safeTLSConfig()
		logging.Infof(s.baseContext, "🚀  Listening for traffic on https://%s\n", addr)
		err = s.httpServer.ServeTLS(ln, s.certFile, s.keyFile)
	} else {
		s.httpServer.Handler = h2c.NewHandler(handler, &http2.Server{})
		logging.Infof(s.baseContext, "🚀  Listening for traffic on http://%s\n", addr)
		err = s.httpServer.Serve(ln)
	}

	if !errors.Is(err, http.ErrServerClosed) {
		return err // The server wasn't shutdown gracefully.
	}

	<-done
	return nil
}

// Shutdown gracefully shuts down the server with a 2s timeout.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(s.baseContext, time.Second*2)
	defer cancel()

	// TODO: Add support for shutdown hooks.

	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		fmt.Printf("❌ Shutdown error: %v\n", err)
	} else {
		fmt.Printf("👍 Connections drained\n")
	}
	s.httpServer = nil
	return err
}
