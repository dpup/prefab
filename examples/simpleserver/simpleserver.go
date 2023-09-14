package main

import (
	"fmt"
	"net/http"

	"github.com/dpup/prefab/examples/simpleserver/simpleservice"
	"github.com/dpup/prefab/grpcserver"
	"github.com/dpup/prefab/logging"
)

func main() {
	server := grpcserver.New(
		grpcserver.WithHTTPHandler("/", http.HandlerFunc(ack)),
	)

	simpleservice.RegisterSimpleServiceHandlerFromEndpoint(server.GatewayArgs())
	simpleservice.RegisterSimpleServiceServer(server.ServiceRegistrar(), simpleservice.New())

	if err := server.Start(); err != nil {
		fmt.Println(err)
	}
}

func ack(w http.ResponseWriter, req *http.Request) {
	logging.Infow(req.Context(), "👋  Ack!", "url", req.URL)
	w.Write([]byte("SimpleServer HTTP Ack\n"))
}
