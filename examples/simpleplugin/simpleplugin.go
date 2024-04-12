package main

import (
	"context"
	"fmt"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/examples/simpleserver/simpleservice"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugin"
	"github.com/dpup/prefab/storage"
	"github.com/dpup/prefab/storage/memorystore"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func main() {
	// Initialize the server with the sample plugin and a memory store.
	s := prefab.New(
		prefab.WithPlugin(&samplePlugin{}),
		prefab.WithPlugin(storage.Plugin(memorystore.New())),
	)

	// Register the GRPC service handlers from the other example.
	simpleservice.RegisterSimpleServiceHandlerFromEndpoint(s.GatewayArgs())
	simpleservice.RegisterSimpleServiceServer(s.ServiceRegistrar(), simpleservice.New())

	// Guidance for people who don't read the example code.
	fmt.Println("")
	fmt.Println("Try making a request to the echo endpoint:")
	fmt.Println("curl 'http://0.0.0.0:8000/api/echo?ping=hello+world'")
	fmt.Println("")
	fmt.Println("Then note that the server logs contain a `req.ping` entry.")
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

type samplePlugin struct {
	store storage.Store
}

// From plugin.Plugin, provides the plugin name for querying and dependency
// resolution.
func (s *samplePlugin) Name() string {
	return "sample"
}

// From plugin.DependentPlugin, ensures dependencies are registered in order.
func (s *samplePlugin) Deps() []string {
	return []string{storage.PluginName}
}

// From prefab.OptionProvider, registers an additional interceptor.
func (s *samplePlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithGRPCInterceptor(s.interceptor),
	}
}

// From plugin.InitializablePlugin, stores a reference to the storage plugin for
// use by the interceptor.
func (s *samplePlugin) Init(ctx context.Context, r *plugin.Registry) error {
	s.store = r.Get(storage.PluginName).(storage.Store)
	logging.Info(ctx, "Sample Plugin initialized!")
	return nil
}

// Simple interceptor that:
// 1. adds fields from the request object to the logs.
// 2. tracks per-method request counts.
//
// FWIW Neither implementation is safe for prod.
func (s *samplePlugin) interceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {

	// Add request fields in the form `req.field`
	if msg, ok := req.(proto.Message); ok {
		m := msg.ProtoReflect()
		fields := m.Descriptor().Fields()
		fieldsLen := fields.Len()
		for i := 0; i < fieldsLen; i++ {
			fd := fields.Get(i)
			v := m.Get(fd)
			fieldName := fmt.Sprintf("req.%s", fd.Name())
			fieldValue := v.Interface()
			logging.Track(ctx, fieldName, fieldValue)
		}
	}

	// Get the current request count for this method.
	var stats Stats
	if err := s.store.Read(info.FullMethod, &stats); err == storage.ErrNotFound {
		stats = Stats{Method: info.FullMethod, Count: 0}
	} else if err != nil {
		logging.Errorw(ctx, "error getting stats", "err", err)
	}

	// Increment the count, and store. Clearly not thread safe!
	stats.Count++
	if err := s.store.Upsert(&stats); err != nil {
		logging.Errorw(ctx, "error saving stats", "err", err)
	}

	// Add the request count to the logging context.
	logging.Track(ctx, "stats.counter", stats.Count)

	return handler(ctx, req)
}

// Model for storing request counts.
type Stats struct {
	Method string
	Count  int
}

func (s *Stats) PK() string {
	return s.Method
}
