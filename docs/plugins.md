# Prefab Plugins

Prefab uses a plugin architecture to provide modular functionality. Plugins are
server-scoped singletons that add features to the base server, expose
functionality for other plugins, or extend existing plugins.

## Plugin System Overview

Plugins in Prefab implement one or more interfaces:

- `prefab.Plugin`: The required base interface that provides a name for each plugin
- `prefab.DependentPlugin`: Allows plugins to specify other plugins they depend on
- `prefab.OptionalDependentPlugin`: Allows plugins to specify optional dependencies
- `prefab.InitializablePlugin`: Allows plugins to be initialized in dependency order
- `prefab.OptionProvider`: Allows plugins to modify server behavior, add services, or handlers

## Common Plugins

### Authentication (auth)

Provides authentication mechanisms through various providers:

```go
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    // Add at least one authentication provider
    prefab.WithPlugin(google.Plugin()),
    // Optional storage for token persistence
    prefab.WithPlugin(storage.Plugin(sqlite.New("auth.db"))),
)
```

Authentication providers include:

- Google OAuth (`google.Plugin()`)
- Magic Link email-based auth (`magiclink.Plugin()`)
- Password authentication (`pwdauth.Plugin()`)
- API Key authentication (`apikey.Plugin()`)
- Fake authentication for testing (`fakeauth.Plugin()`) - not for production use

### Authorization (authz)

Provides access control for RPC endpoints:

```go
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(pwdauth.Plugin(...)),
    prefab.WithPlugin(authz.Plugin(
        authz.WithPolicy(authz.Allow, roleUser, authz.Action("resources.read")),
        authz.WithObjectFetcher("resource", fetchResource),
        authz.WithRoleDescriber("*", roleDescriber),
    )),
)
```

### Storage

Provides simple CRUD operations:

```go
s := prefab.New(
    prefab.WithPlugin(storage.Plugin(memstore.New())),
    // or
    prefab.WithPlugin(storage.Plugin(sqlite.New("data.db"))),
)
```

### Email

Enables sending emails (required for magic link authentication):

```go
s := prefab.New(
    prefab.WithPlugin(email.Plugin()),
)
```

### Templates

Provides templating for emails and other content:

```go
s := prefab.New(
    prefab.WithPlugin(templates.Plugin()),
)
```

## Creating Custom Plugins

To create a custom plugin:

```go
type myPlugin struct {
    // Plugin state
}

// Required: Plugin name for dependency resolution
func (p *myPlugin) Name() string {
    return "myplugin"
}

// Optional: Specify required dependencies
func (p *myPlugin) Deps() []string {
    return []string{"anotherplugin"}
}

// Optional: Add server options
func (p *myPlugin) ServerOptions() []prefab.ServerOption {
    return []prefab.ServerOption{
        prefab.WithGRPCInterceptor(p.interceptor),
    }
}

// Optional: Initialize plugin
func (p *myPlugin) Init(ctx context.Context, r *prefab.Registry) error {
    // Access other plugins via registry
    otherPlugin := r.Get("anotherplugin").(AnotherPlugin)
    return nil
}

// Add to server
s := prefab.New(
    prefab.WithPlugin(&myPlugin{}),
)
```

## Registering Plugin Configuration

Plugins should register their configuration keys to enable typo detection and validation. Register keys in an `init()` function:

```go
package myplugin

import "github.com/dpup/prefab"

const PluginName = "myplugin"

func init() {
    prefab.RegisterConfigKeys(
        prefab.ConfigKeyInfo{
            Key:         "myplugin.apiKey",
            Description: "API key for external service",
            Type:        "string",
        },
        prefab.ConfigKeyInfo{
            Key:         "myplugin.timeout",
            Description: "Request timeout",
            Type:        "duration",
        },
    )
}

func Plugin() *MyPlugin {
    return &MyPlugin{
        apiKey:  prefab.ConfigString("myplugin.apiKey"),
        timeout: prefab.ConfigDuration("myplugin.timeout"),
    }
}
```

Prefab will automatically validate loaded config keys at startup and warn about potential typos or unknown keys, helping users catch configuration errors early.
