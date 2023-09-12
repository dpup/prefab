package simpleservice

import (
	"context"
	"fmt"
)

func New() SimpleServiceServer {
	return &server{}
}

// Implements SimpleServiceServer.
type server struct {
	UnimplementedSimpleServiceServer
}

func (s *server) Health(ctx context.Context, in *HealthRequest) (*HealthResponse, error) {
	fmt.Printf("❤️  Server health reported\n")
	return &HealthResponse{
		Status: "OK",
	}, nil
}
