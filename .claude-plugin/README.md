# Prefab Plugin for Claude Code

This plugin provides skills and resources for developing Go applications using the Prefab server framework.

## Skills

### prefab-dev

A comprehensive skill for building Prefab-based servers, including:

- Server setup and initialization
- gRPC and HTTP handler patterns
- Authentication (OAuth, password, magic link)
- Authorization with declarative access control
- Server-Sent Events (SSE) streaming
- Configuration management
- Error handling best practices
- Security patterns

## Installation

```
/plugin marketplace add dpup/prefab
```

## Usage

Once installed, Claude will automatically use this skill when you're working on Prefab-based projects or ask about Prefab patterns and features.

## Resources

The skill dynamically loads topic-specific resources based on your task:

- `project-setup.md` - Proto files, Makefile, project structure
- `server-setup.md` - Server creation and initialization
- `grpc-http.md` - gRPC services and HTTP handlers
- `auth.md` - Authentication plugins and patterns
- `authz.md` - Authorization and access control
- `sse.md` - Server-Sent Events streaming
- `configuration.md` - Configuration management
- `storage.md` - Storage plugins
- `uploads.md` - File upload/download
- `email.md` - SMTP email sending
- `templates.md` - Go HTML templates
- `eventbus.md` - Publish/subscribe events
- `logging.md` - Structured logging
- `plugins.md` - Custom plugin development
- `errors.md` - Error handling patterns
- `security.md` - Security best practices
