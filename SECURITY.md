# Security Policy

Prefab provides authentication, authorization, OAuth2, and CSRF functionality,
so security reports are taken seriously.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, use GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/dpup/prefab/security) of the
   repository.
2. Click **Report a vulnerability** to open a private advisory.

Please include:

- A description of the vulnerability and its impact.
- Steps to reproduce, or a proof of concept.
- Affected version(s) and configuration.

You can expect an initial acknowledgement within a few business days. Once the
issue is confirmed, a fix and coordinated disclosure will be arranged.

## Supported Versions

Prefab is pre-1.0; only the latest tagged release receives security fixes.
Pin a specific version and review the [CHANGELOG](./CHANGELOG.md) before
upgrading.

## Security Notes for Operators

A few defaults are safe for development but **must** be configured for
production:

- **JWT signing key** (`auth.signingKey` / `PF__AUTH__SIGNING_KEY`): if unset, a
  random key is generated at startup and a warning is logged. Tokens are then
  invalidated on restart and not shared across instances. Set an explicit key in
  production.
- **CSRF signing key** (`server.csrfSigningKey` / `PF__SERVER__CSRF_SIGNING_KEY`):
  same behavior — a random key is generated with a warning if unset. Set an
  explicit key in production.
- **OAuth2 token/client stores**: in-memory by default (development only).
  Configure persistent `TokenStore` / `ClientStore` implementations for
  production.
- **`oauth.enforcePkce`**: enable it if you register any public clients.
- **`/debug/authz`**: disabled by default; only enable it via
  `authz.WithDebugEndpoint()` in trusted environments — it exposes the full
  authorization model.
