# Custom Plugins

Create custom plugins to extend Prefab functionality.

## Plugin Structure

```go
type myPlugin struct {
    dependency anotherPlugin
}

// Plugin name for dependency resolution
func (p *myPlugin) Name() string {
    return "myplugin"
}

// Specify required dependencies
func (p *myPlugin) Deps() []string {
    return []string{"anotherplugin"}
}

// Add server options
func (p *myPlugin) ServerOptions() []prefab.ServerOption {
    return []prefab.ServerOption{
        prefab.WithGRPCInterceptor(p.interceptor),
    }
}

// Initialize plugin
func (p *myPlugin) Init(ctx context.Context, r *prefab.Registry) error {
    // Option 1: Get by name with type assertion
    p.dependency = r.Get("anotherplugin").(anotherPlugin)

    // Option 2: Get by type
    dep, ok := prefab.GetPlugin[anotherPlugin](r)
    if !ok {
        return fmt.Errorf("failed to get anotherplugin")
    }
    p.dependency = dep

    return nil
}
```

## Registering Plugins

```go
s := prefab.New(
    prefab.WithPlugin(&myPlugin{}),
)
```

## Plugin Interface

```go
type Plugin interface {
    Name() string
}

// Optional interfaces
type PluginWithDeps interface {
    Deps() []string
}

type PluginWithInit interface {
    Init(ctx context.Context, r *Registry) error
}

type PluginWithServerOptions interface {
    ServerOptions() []ServerOption
}

type PluginWithShutdown interface {
    Shutdown(ctx context.Context) error
}
```

## Plugin Lifecycle

1. **Registration** - Plugins are registered with `WithPlugin()`
2. **Dependency Resolution** - Dependencies are resolved based on `Deps()`
3. **ServerOptions** - `ServerOptions()` is called to gather options
4. **Initialization** - `Init()` is called in dependency order
5. **Running** - Server runs with all plugins active
6. **Shutdown** - `Shutdown()` is called in reverse order

## Example: Metrics Plugin

```go
type metricsPlugin struct {
    requestCount int64
}

func MetricsPlugin() prefab.Plugin {
    return &metricsPlugin{}
}

func (p *metricsPlugin) Name() string {
    return "metrics"
}

func (p *metricsPlugin) ServerOptions() []prefab.ServerOption {
    return []prefab.ServerOption{
        prefab.WithGRPCInterceptor(p.interceptor),
        prefab.WithHTTPHandler("/metrics", p.metricsHandler),
    }
}

func (p *metricsPlugin) interceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    atomic.AddInt64(&p.requestCount, 1)
    return handler(ctx, req)
}

func (p *metricsPlugin) metricsHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "request_count %d\n", atomic.LoadInt64(&p.requestCount))
}
```

## Plugin Naming Convention

Export a `Plugin()` function and `PluginName` constant:

```go
const PluginName = "myplugin"

func Plugin(opts ...Option) prefab.Plugin {
    p := &myPlugin{}
    for _, opt := range opts {
        opt(p)
    }
    return p
}
```

For complete documentation, see [/docs/plugins.md](/docs/plugins.md).
