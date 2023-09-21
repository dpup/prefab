
## Opinions

- Zap was chosen as default logging library.
- JWT for authorization tokens and cookies.
- [TK] for templating. (e.g. for default login page)


## Ideas for "plugin" interface

Plugins can add:

- http handlers
- grpc handlers
- interceptors
- data to the context, made accessible via helper methods

Plugins provide:
- static functions which can be called from other plugins or application code

Plugins can access:

- storage
- other plugins data via static function calls

Plugins:

- specify dependencies to ensure initialization order is correct