package main

import (
	"context"
	"fmt"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/examples/simpleserver/simpleservice"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/storage"
	"github.com/dpup/prefab/plugins/storage/memstore"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func main() {
	// Initialize the server with the sample plugin and a memory store.
	s := prefab.New(
		prefab.WithPlugin(&samplePlugin{}),
		prefab.WithPlugin(storage.Plugin(memstore.New())),
	)

	// Register the GRPC service handlers from the other example.
	s.RegisterService(
		&simpleservice.SimpleService_ServiceDesc,
		simpleservice.RegisterSimpleServiceHandler,
		simpleservice.New(),
	)

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

// From prefab.Plugin, provides the plugin name for querying and dependency
// resolution.
func (s *samplePlugin) Name() string {
	return "sample"
}

// From prefab.DependentPlugin, ensures dependencies are registered in order.
func (s *samplePlugin) Deps() []string {
	return []string{storage.PluginName}
}

// From prefab.OptionProvider, registers an additional interceptor.
func (s *samplePlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithGRPCInterceptor(s.interceptor),
	}
}

// From prefab.InitializablePlugin, stores a reference to the storage plugin for
// use by the interceptor.
//
//nolint:unparam // return value is always nil, but we need it to satisfy the interface.
func (s *samplePlugin) Init(ctx context.Context, r *prefab.Registry) error {
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
		for i := range fields.Len() {
			fd := fields.Get(i)
			v := m.Get(fd)
			fieldName := fmt.Sprintf("req.%s", fd.Name())
			fieldValue := v.Interface()
			logging.Track(ctx, fieldName, fieldValue)
		}
	}

	// Get the current request count for this method.
	var stats Stats
	if err := s.store.Read(ctx, info.FullMethod, &stats); errors.Is(err, storage.ErrNotFound) {
		stats = Stats{Method: info.FullMethod, Count: 0}
	} else if err != nil {
		logging.Errorw(ctx, "error getting stats", "err", err)
	}

	// Increment the count, and store. Clearly not thread safe!
	stats.Count++
	if err := s.store.Upsert(ctx, &stats); err != nil {
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
