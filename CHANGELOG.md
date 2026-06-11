# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to
follow [Semantic Versioning](https://semver.org/) (note: pre-1.0, the public API
may change between minor versions).

## [Unreleased]

### Security

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

[Unreleased]: https://github.com/dpup/prefab/compare/v0.4.2...HEAD
[0.4.2]: https://github.com/dpup/prefab/releases/tag/v0.4.2
