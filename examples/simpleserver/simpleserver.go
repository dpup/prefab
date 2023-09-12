package main

import (
	"fmt"
	"net/http"

	"github.com/dpup/prefab/examples/simpleserver/simpleservice"
	"github.com/dpup/prefab/grpcserver"
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
	fmt.Printf("👋  Ack!  %v\n", req.URL)
	w.Write([]byte("SimpleServer HTTP Ack\n"))
}
