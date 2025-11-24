# Project Setup

Recommendations for setting up a Prefab-based Go project with protobuf code generation.

## Project Structure

```
myproject/
├── cmd/
│   └── server/
│       └── main.go           # Server entrypoint
├── proto/
│   └── myservice/
│       └── myservice.proto   # Service definitions
├── gen/
│   └── myservice/
│       ├── myservice.pb.go        # Generated protobuf
│       ├── myservice_grpc.pb.go   # Generated gRPC
│       └── myservice.pb.gw.go     # Generated gateway
├── internal/
│   └── myservice/
│       └── service.go        # Service implementation
├── config.yaml               # Configuration
├── Makefile
└── go.mod
```

## Proto File Template

```protobuf
syntax = "proto3";

package myservice;

option go_package = "github.com/yourorg/myproject/gen/myservice";

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";

service MyService {
  rpc GetItem(GetItemRequest) returns (Item) {
    option (google.api.http) = {
      get: "/api/v1/items/{id}"
    };
  }

  rpc CreateItem(CreateItemRequest) returns (Item) {
    option (google.api.http) = {
      post: "/api/v1/items"
      body: "*"
    };
  }

  rpc ListItems(ListItemsRequest) returns (ListItemsResponse) {
    option (google.api.http) = {
      get: "/api/v1/items"
    };
  }
}

message Item {
  string id = 1;
  string name = 2;
  string description = 3;
}

message GetItemRequest {
  string id = 1;
}

message CreateItemRequest {
  string name = 1;
  string description = 2;
}

message ListItemsRequest {
  int32 page_size = 1;
  string page_token = 2;
}

message ListItemsResponse {
  repeated Item items = 1;
  string next_page_token = 2;
}
```

## Makefile

```makefile
.PHONY: all proto build run test lint clean

# Proto generation settings
PROTO_DIR := proto
GEN_DIR := gen
PROTO_FILES := $(shell find $(PROTO_DIR) -name '*.proto')

# Go settings
BINARY := server
CMD_DIR := ./cmd/server

all: proto build

# Generate protobuf, gRPC, and gateway code
proto:
	@mkdir -p $(GEN_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--proto_path=proto/third_party/googleapis \
		--go_out=$(GEN_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) \
		--go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(GEN_DIR) \
		--grpc-gateway_opt=paths=source_relative \
		--grpc-gateway_opt=logtostderr=true \
		$(PROTO_FILES)

build: proto
	go build -o $(BINARY) $(CMD_DIR)

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf $(GEN_DIR)
	rm -f $(BINARY)

# Install required tools
tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
```

## Required Tools

Install protobuf compiler and Go plugins:

```bash
# macOS
brew install protobuf

# Ubuntu/Debian
apt-get install protobuf-compiler

# Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
```

## Google API Protos

You need the Google API protos for HTTP annotations. Add them as a submodule or copy them:

```bash
mkdir -p proto/third_party
git clone --depth 1 https://github.com/googleapis/googleapis.git /tmp/googleapis
cp -r /tmp/googleapis/google proto/third_party/
```

Or use buf.build for proto management.

## go.mod Dependencies

```bash
go get github.com/dpup/prefab
go get google.golang.org/grpc
go get google.golang.org/protobuf
go get github.com/grpc-ecosystem/grpc-gateway/v2
```

## Server Implementation

```go
package main

import (
    "log"

    "github.com/dpup/prefab"
    "github.com/dpup/prefab/logging"
    "github.com/dpup/prefab/plugins/auth"

    pb "github.com/yourorg/myproject/gen/myservice"
    "github.com/yourorg/myproject/internal/myservice"
)

func main() {
    s := prefab.New(
        prefab.WithPort(8080),
        prefab.WithLogger(logging.NewProdLogger()),
        prefab.WithPlugin(auth.Plugin()),
    )

    s.RegisterService(
        &pb.MyService_ServiceDesc,
        pb.RegisterMyServiceHandler,
        myservice.New(),
    )

    if err := s.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## Development Workflow

1. Define/update proto files in `proto/`
2. Run `make proto` to generate code
3. Implement service in `internal/`
4. Run `make run` to start server
5. Test with `make test`

## Common Patterns

### Multiple Services

```makefile
# Generate all protos
proto:
	@for dir in $(PROTO_DIR)/*/; do \
		protoc ... $$dir/*.proto; \
	done
```

### Watch Mode (with entr)

```bash
find proto -name '*.proto' | entr -r make run
```

### Docker Build

```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN make build

FROM gcr.io/distroless/base
COPY --from=builder /app/server /server
COPY --from=builder /app/config.yaml /config.yaml
CMD ["/server"]
```
