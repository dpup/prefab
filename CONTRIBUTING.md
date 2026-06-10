# Contributing to Prefab

Thanks for your interest in contributing! This guide covers the development
setup and conventions for the project.

## Prerequisites

- **Go 1.25 or later** (see the `go` directive in [go.mod](./go.mod)).
- **Protocol Buffers compiler** (`protoc`) — only needed when regenerating code
  from `.proto` files. Generated `*.pb.go` files are committed, so most changes
  do not require it.

The Go-based code generators (`protoc-gen-go`, `protoc-gen-go-grpc`,
`protoc-gen-grpc-gateway`, `protoc-gen-openapiv2`) and `staticcheck` are managed
as Go tool dependencies and built by `make tools`.

## Common Commands

```sh
make test        # staticcheck + go vet + unit tests
make lint        # golangci-lint
make fix         # golangci-lint --fix
make gen-proto   # regenerate protobuf code (requires protoc)
make tools       # build the pinned code-generation tools
```

To run a single test:

```sh
go test ./path/to/package -run TestName
```

For coverage:

```sh
make test-coverage                      # human-readable summary
make test-coverage TARGET=./plugins/auth
make test-coverage PORCELAIN=1          # machine-readable
```

## Code Style

- **Errors**: use the project's `errors` package (`errors.New`, `errors.NewC`,
  `errors.Wrap`, `errors.WithCode`) for stack traces and coded errors.
- **Imports**: standard library first, then third-party; alias to resolve
  conflicts.
- **Naming**: Go-standard camelCase / PascalCase. Plugins expose a `Plugin()`
  constructor and, when used by other plugins, an exported `PluginName`.
- **Documentation**: document exported APIs with GoDoc comments.
- **Testing**: add tests alongside implementation; keep coverage meaningful.

CI runs `go build`, `go vet`, `go test`, `staticcheck`, and `golangci-lint` on
every pull request — please make sure these pass locally before opening a PR.

## Commit Messages

This repository follows [Conventional Commits](https://www.conventionalcommits.org/)
(e.g. `fix(oauth): ...`, `chore(deps): ...`, `docs: ...`).

## Pull Requests

1. Fork and create a feature branch.
2. Make your change with tests.
3. Ensure `make test` and `make lint` pass.
4. Open a PR describing the change and its motivation.
