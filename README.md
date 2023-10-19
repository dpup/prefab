
## Opinions

- UUIDs for unique identifiers generated by prefab.
- JWT for authorization tokens and cookies.
- Zap was chosen as default logging library.
- [TK] for templating. (e.g. for default login page)


## Plugins

Plugins are essentially server scoped singletons.

Plugins expose a `Name()` method which returns a string used for querying and
for dependency resolution.

Plugins may expose a `Deps()` method which returns a list of names for plugins
that should be initialized before hand.

Plugins may expose a `ServerOptions()` method, which allows for the registration
of HTTP handlers, GRPC handlers, and interceptors.

Plugins may expose an `Init()` method, which is called when the server starts
up, and should be used for getting references to other plugins.

By convention, plugins should be named `Plugin`. If the plugin is intended to be
used by other plugins, it's name should be exported as `PluginName`. For
example, `gpt.Plugin` and `gpt.PluginName`.
