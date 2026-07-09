# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to
follow [Semantic Versioning](https://semver.org/) (note: pre-1.0, the public API
may change between minor versions).

## [0.6.0] - 2026-07-09

### Added

- **ETag plugin (`etag.Plugin()`) for conditional requests.** Handlers advertise
  a validator and short-circuit response generation when the caller already
  holds a current copy:

  ```go
  if err := etag.Guard(ctx, etag.Weak(version)); err != nil {
      return nil, err // 304 / not-modified; the expensive load below is skipped
  }
  ```

  `etag.Matches` is also exposed for handlers that need custom logic before
  bailing. The plugin is transport agnostic: it reads `If-None-Match` from the
  HTTP header (Gateway) or `if-none-match` request metadata (native gRPC), and
  signals not-modified via a `304` status (Gateway) or a `prefab-not-modified`
  response metadata flag with an `OK` status (native gRPC), so it never overloads
  a gRPC error code. Client helpers `etag.IfNoneMatch`, `etag.IsNotModified`, and
  `etag.ETag` support native callers. See `examples/etag` for a curl-driven demo.

### Changed

- **Gateway responses now enforce conditional-response hygiene.** A `304 Not
  Modified` emitted via `serverutil.SendStatusCode` is stripped of its body and
  content headers (per RFC 7232) before being written. Applied only to the
  Gateway mux, so streaming endpoints are unaffected.

## [0.5.0] - 2026-06-17

### Security

- **PKCE is now enforced for public OAuth clients by default.** The
  `oauth.enforcePkce` config now defaults to `true`, so public clients must use
  `S256` PKCE (the `plain` method is rejected). Set `oauth.enforcePkce=false` to
  restore the previous behavior. **Breaking** for public clients that were not
  already sending a `code_challenge`.
- **CSRF signing key now fails safe.** When `server.csrfSigningKey` is unset, a
  random key is generated at startup and a warning is logged, instead of
  silently signing tokens with an empty key.
- **JWT fallback signing key is no longer a hardcoded constant.** Token
  operations that run on a context without the injected signing key now use an
  ephemeral, randomly generated per-process key (with a one-time warning) rather
  than a value baked into the source.
- **`/debug/authz` is now opt-in.** The endpoint, which exposes the full role
  hierarchy and policy set, is disabled by default and must be enabled with
  `authz.WithDebugEndpoint()`.
- **OAuth warns when using the in-memory token store.** The plugin now logs a
  warning at startup when no persistent `TokenStore` is configured, since tokens
  are otherwise lost on restart and not shared across instances.

### Added

- **OAuth fail-closed scope guards:** `oauth.RequireScope`, `oauth.RequireOAuth`,
  and `oauth.RequireAnyScope`. Unlike a bare `IsOAuthRequest`/`HasScope` check
  (which skips for first-party cookie sessions), these reject non-OAuth requests,
  making them the correct guard for OAuth-only endpoints where scope is the
  authorization boundary. The OAuth plugin README now documents both models.
- GitHub Actions CI workflow running build, vet, test, staticcheck, and
  golangci-lint on pushes and pull requests.
- `SECURITY.md`, `CONTRIBUTING.md`, and this `CHANGELOG.md`.

### Changed

- **SQLite store is now pure Go.** Replaced the CGO-based `mattn/go-sqlite3`
  driver with `modernc.org/sqlite`, so the entire module builds and tests with
  `CGO_ENABLED=0` and no C toolchain. The DSN no longer supports the
  mattn-specific `_auth` userauth parameters.
- `staticcheck` is now a managed Go tool dependency; `make test` invokes it via
  `go tool staticcheck` and no longer requires a separate global install.

### Fixed

- README: corrected the default port (`8000`), fixed broken storage plugin
  links, removed a stray parenthesis in the blocklist example, listed the Work
  Queue plugin, and clarified the storage backend options. Corrected the Go
  version in `docs/README.md`.

## [0.4.2]

### Fixed

- **upload:** stored uploads now use a deterministic file extension (e.g.
  `image/jpeg` → `.jpg`) instead of a host-dependent value from the mime
  database.
- **server:** replaced the deprecated `h2c.NewHandler` with
  `http.Server.Protocols` for cleartext HTTP/2.
- **build:** corrected the `GOPATH` path used during protobuf generation.

### Changed

- **deps:** updated dependencies, including `golang.org/x/crypto`,
  `golang.org/x/net`, and `golang.org/x/sys`, resolving outstanding
  vulnerability advisories reported by `govulncheck`.

## Earlier releases

See the [releases page](https://github.com/dpup/prefab/releases) and the git
history for changes prior to 0.4.2.

[0.6.0]: https://github.com/dpup/prefab/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/dpup/prefab/compare/v0.4.2...v0.5.0
[0.4.2]: https://github.com/dpup/prefab/releases/tag/v0.4.2
