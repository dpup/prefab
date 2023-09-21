package main

import (
	"fmt"
	"net/http"

	"github.com/dpup/prefab/examples/simpleserver/simpleservice"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/server"
)

func main() {
	s := server.New(
		server.WithHTTPHandler("/", http.HandlerFunc(ack)),
	)

	simpleservice.RegisterSimpleServiceHandlerFromEndpoint(s.GatewayArgs())
	simpleservice.RegisterSimpleServiceServer(s.ServiceRegistrar(), simpleservice.New())

	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

func ack(w http.ResponseWriter, req *http.Request) {
	logging.Infow(req.Context(), "👋  Ack!", "url", req.URL)
	w.Write([]byte("SimpleServer HTTP Ack\n"))
}
