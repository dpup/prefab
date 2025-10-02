# Prefab Project Guidelines

## Build Commands

- `make test` - Run all tests
- `make test-unit` - Run unit tests with coverage
- `make test-vet` - Run go vet
- `make test-staticcheck` - Run staticcheck
- `go test ./path/to/package -run TestName` - Run a specific test
- `make lint` - Run golangci-lint
- `make gen-proto` - Generate protocol buffer code
- `go mod tidy` - Update dependencies

### Coverage Commands

- `make test-coverage` - Generate test coverage for all packages (human-readable summary)
- `make test-coverage TARGET=./plugins/auth` - Generate coverage for specific package
- `make test-coverage PORCELAIN=1` - Machine-readable output (for AI/automation use)
- `make clean-coverage` - Remove coverage report files

The `test-coverage` target automatically excludes generated protobuf files (`.pb.go`, `.pb.gw.go`) from coverage calculations. By default, it shows a human-readable summary. Use `PORCELAIN=1` for detailed function-by-function output suitable for AI analysis or automated processing.

## Code Style

- **Errors**: Use custom errors package with stack traces; utilize `errors.New()`, `errors.NewC()`, `errors.Wrap()`, `errors.WithCode()`
- **Imports**: Standard library first, followed by third-party; use aliasing to resolve conflicts
- **Naming**: Go standard camelCase for variables, PascalCase for exported items; consistent plugin naming (`PluginName`)
- **Plugins**: Follow plugin interface patterns; expose via `Plugin()` function and export `PluginName` constant
- **Documentation**: Document public APIs with clear GoDoc comments; provide examples
- **Testing**: Write tests alongside implementation; provide comprehensive test coverage
- **Error Handling**: Distinguish between user-facing and internal errors; propagate context
- **Configuration**: Follow established config patterns using koanf
- **Logging**: Use structured logging with field tracking and request context

Follow patterns in existing code for consistency. Use provided Go tools and linters.

## Docs

Prefab maintains documentation targeted at AI Coding tools within the `/docs/` folder. `/docs/reference.md` contains
examples and coding guidlines. When making changes ensure the reference material is kept up to date.
