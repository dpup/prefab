package simpleservice

import (
	"context"

	"github.com/dpup/prefab/logging"
)

func New() SimpleServiceServer {
	return &server{}
}

// Implements SimpleServiceServer.
type server struct {
	UnimplementedSimpleServiceServer
}

func (s *server) Health(ctx context.Context, in *HealthRequest) (*HealthResponse, error) {
	logging.Info(ctx, "❤️  Server health reported")
	return &HealthResponse{
		Status: "OK",
	}, nil
}
