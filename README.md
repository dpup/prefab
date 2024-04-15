
# Prefabricated gRPC Server and JSON/REST Gateway

**Prefab** is a library designed to streamline the setup of gRPC servers with a
gRPC Gateway. It provides sensible defaults to get your server up and running
with minimal boilerplate, while also offering configuration via environment
variables, config files, or programmatic options.

Furthermore, Prefab includes a suite of plugins, adding capabilities such as
authentication, templating, and ACLs without adding bloat.

## ✅  Features

- **Quick setup:** A production ready gRPC server in 6 lines of code.
- **Serve static files:** Support for hybrid monoliths serving gRPC and HTTP.
- **Auth Plugin:** Authenticate users with Google, Magic Links, or Email/Password.
- **ACLs:** Use proto options to define access rules for RPC endpoints.
- **Security:** CSRF protection built-in and options for configuring CORS.
- **Logging:** Pluggable logging with request scoped field tracking.
- **Configuration:** File, env, or functional options.
- **Templates:** Currently using standard go templates.

## 🚀  Quick Start

Given a GRPC service implementation, the following snippet will start a running
server on `localhost:5678` serving both GRPC requests and JSON rest, with the
only constraint that the GRPC path bindings must be prefixed with `/api/`.

```go
package main

import (
  "fmt"
  "github.com/dpup/prefab"
)

func main() {
	s := prefab.New()
	RegisterFooBarHandlerFromEndpoint(s.GatewayArgs())
	RegisterFooBarServer(s.ServiceRegistrar(), &foobarImpl{})
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}
```


## 💡  Opinions

- UUIDs for unique identifiers generated by prefab.
- JWT for authorization tokens and cookies.
- Functional options pattern for configuring services and plugins, with
  configuration fallback.

## 🚚  Key dependencies

- GRPC and GRPC Gateway ... obviously.
- Koanf for configuration.
- Zap was chosen as default logging library.
- Gomail for sending emails, defaults to SMTP but can be customized.
- Standard Go templates

## 🔌  Plugins

- [Plugin Model Overview](#plugin-model-overview)
- [Authentication](#authentication)
- [Storage](#storage)

### Plugin Model Overview

The base server is intended to have everything need to run a standalone service
that multiplexes across a GRPC interface, a JSON/REST interface via the GRPC
Gateway, and arbitrary HTTP handlers, for non-GRPC functionality. Additional
functionality is exposed as plugins.

Plugins are essentially server scoped singletons which add functionality to the
base server, expose functionality for other plugins, or extend other plugins.

As an example, the `Magic Link Plugin` extends the `Auth Plugin` to add
authentication via email. It also depends on an `Email Plugin` for email sending,
and a `Template Plugin` for rendering HTML emails.

It is intended that plugins can be have interchangable implementations to allow
customization at various parts of the stack.

#### Plugin interfaces

Plugins can implement a number of discrete interfaces:

- `plugin.Plugin` : the required base interface which provides a name for each plugin.
- `plugin.DependentPlugin` : allows plugins to specifi other plugins which they need to use.
- `plugin.OptionalDependentPlugin` : allows plugins to specify optional dependencies, which are not required, but must be initialized first.
- `plugin.InitializablePlugin` : allows plugins to be initialized in dependency order, allowing for more control of setup.
- `prefab.OptionProvider` : allows plugins to modify the server behavior, add services, or handlers. See `prefab.Option` for full functionality.

By convention, plugins should be created by a `Plugin` function. If the plugin
is intended to be used by other plugins, it's name should be exported as
`PluginName`. For example, `gpt.Plugin(...)` and `gpt.PluginName`.

Explore the GoDoc and examples to learn how to use each plugin.

### Authentication

Prefab offers a number of authentication plugins that can speed up development
of logged in experiences.

Core functionality is provided by `auth.Plugin()`, however at least one auth
provider should be registered. The following providers are currently included.

- [Google SSO](./examples/googleauth/googleauth.go)
- [Magiclink passwordless login](./examples/magiclinkauth/magiclinkauth.go)

Login is initiated through the `auth.Login()` RPC or the `/api/auth/login`
endpoint.

Clients can access identity information through the `auth.Identity()` RPC or the
`/api/auth/me` endpoint. 

Authentication can be performed through bearer tokens or cookies.

Importantly, the authentication plugins make an authenticated identity available,
however, it does not handle authorization. That must be handled by application
code.

#### Configuration

| Functional Option | Configuration Key | Description                          |
|-------------------|-------------------|--------------------------------------|
| `WithSigningKey`  | `auth.signingKey` | Key used when signing JWT tokens     |
| `WithExpiration`  | `auth.expiration` | Expiry duration for which JWT tokens |
| `WithBlocklist`   | -                 | Customize blocklist implementation   |

#### Invalidation

By default, Prefab's identity tokens are valid until they expire. Logging out
will clear the cookie, but if the token was copied or compromised it can still
be used to identify the user. If the token is used for accessing sensitive
resources, then it is recommended that a short lifetime be configured via the
`auth.expiration` config or `auth.WithExpiration` option.

If you wish to utilize long-lived identity tokens, but need a way to ensure they
are revoked, then you can initialize your server with a Storage Plugin and the
Auth Plugin with persist blocked tokens. Everytime a token is validated, the
blocklist will be checked, which will introduce some latency.

```go
s := prefab.New(
  ...
  prefab.WithPlugin(storage.Plugin(store)),
  prefab.WithPlugin(auth.Plugin()),
  )),
  ...
)
```

If you wish you implement your own Blocklist, then you can do so like so:

```go
s := prefab.New(
  ...
  prefab.WithPlugin(auth.Plugin(
    auth.WithBlocklist(auth.NewBlocklist(store)),
  )),
  ...
)
```

### Storage

Prefab includes a simple Storage interface, primarily for use within plugins,
but also for simple applications. The interface exposes Create, Read, Update,
Upsert, Delete, List, and Exists (CRUUDLE) methods and can be backed by a memory
store, filesystems, RDS, or NoSQL databases.

Entities are modeled as Go structs which expose a `PK()` method. The internal
storage representation is implementation specific, however JSON is a common
default, and as such `List` operations may not be performant for many situations.

Included implementations:

#### [In-memory](./storage/memorystore/)

Stores data in simple Go maps.

#### [SQLite3](./storage/sqlitestore/)

SQLite backed storage. Explicitly initialized models are stored in their own
table, with a `prefab_` prefix. Uninitialized models are stored in
`prefab_default` indexed by `ID` and `EntityType`.

## 🔐  Security

- CORS
- [CSRF Protection](#csrf-protection)

### CSRF Protection

CSRF Protection is handled by middleware and is controlled by an option on the
method descriptor.

Possible values are "on", "off", and "auto", where "auto is
the default.

- "on" means CSRF protection is always needed.
- "off" means it is never needed.
- "auto" indicates CSRF protection is needed unless the HTTP method is `GET`, 
`HEAD`, or `OPTIONS`.

```proto
rpc Get(Request) returns (Response) {
  option (csrf_mode) = "on";
  option (google.api.http) = {
    get: "/get"
  };
}
```

CSRF Protection comes in two forms, an `X-CSRF-Protection` header, which can be
set on XHR requests, or a [signed double-submit cookie](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html#signed-double-submit-cookie-recommended),
which are to be used for form posts and full-page navigations.

For the double-submit method, the CSRF Token can be requested from the
`/api/meta/config` endpoint. It is valid for 6 hours, querying the config
endpoint will extend the expiration. The token should be passed to the server in
a `csrf-token` query param.

The token can also be found in the `pf-ct` cookie. If you are using the cookie
instead of requesting the token from the config endpoint then your server code
will need to call `prefab.SendCSRFToken(ctx, signingKey)` at somepoint in the
user journey to ensure the cookie is set.

Example XHR:

```js
fetch('/api/users/154', {
  method: 'PATCH',
  headers: {
    'Content-Type': 'application/json',
    'X-CSRF-Protection': 1,
  },
  credentials: 'include',
  body: JSON.stringify({
    name: 'Frodo Baggins',
  })
})
```

