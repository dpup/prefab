
## Open design questions

### Configuration

Currently Viper is used to allow configs to be derived from multiple sources.
Within the code constructors often reach directly into viper to get the values
via a string key, e.g:

```go
func Plugin() *AuthPlugin {
  return &AuthPlugin{
    authService:   &impl{},
    jwtSigningKey: viper.GetString("auth.signingkey"),
    jwtExpiration: viper.GetDuration("auth.expiration"),
  }
}
```

I don't like that this places a hard dependency on external configuration or
that it uses stringly typed values.

## Opinions

- UUIDs for unique identifiers generated by prefab.
- JWT for authorization tokens and cookies.
- Functional options pattern for configuring services and plugins, with
  configuration fallback.

## Key dependencies

- GRPC and GRPC Gateway ... obviously.
- Viper for configuration.
- Zap was chosen as default logging library.
- Gomail for sending emails, defaults to SMTP but can be customized.
- [TK] for templating. (e.g. for default login page)

## Features

- [Plugin Model](#plugin-model)
- [Authentication](#authentication)
- [CSRF Protection](#csrf-protection)

### Plugin Model

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
- `plugin.DependentPlugin` : allows plugins to specify other plugins which they need to use.
- `plugin.InitializablePlugin` : plugins will be initialized in dependency order, allowing for more control of setup.
- `server.OptionProvider` : used to change server behavior, add services, or handlers. See `server.Option` for full functionality.

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
will need to call `server.SendCSRFToken(ctx, signingKey)` at somepoint in the
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

