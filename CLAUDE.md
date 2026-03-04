# Wordmade ID MCP Server

Public source for the Wordmade ID MCP server binary.

## What This Is

A standalone MCP (Model Context Protocol) server that exposes Wordmade ID
agent identity operations over stdio. It proxies all requests through the
ID REST API — no database access, no internal dependencies.

## Structure

```
cmd/id-mcp/              Entry point (main.go)
internal/mcpserver/      MCP server implementation
  client.go              HTTP client for ID REST API
  server.go              MCP server setup + tool/resource registration
  tools.go               Tool handlers (6 tools)
  resources.go           Resource handlers (2 resources)
```

## Building

```bash
make build               # Build binary to bin/id-mcp
make install             # Build and install to ~/.local/bin
make test                # Run tests
make release             # Cross-compile for 5 platforms
```

## Source Sync

This repo's source files are synced from the private `wordmade/id` repo.
The sync workflow is documented in `id`'s `.claude/skills/id-mcp-maintainer/SKILL.md`.

**Sync direction:** private `wordmade/id` -> public `wordmade/id-mcp` (one-way)

Changes to MCP server code are made in the private repo first, then synced here.
Do not make code changes directly in this repo — they will be overwritten.

## CI/CD

- **CI** (`ci.yml`): Go build, test, structure verify, scrub scan (infra + identity leaks)
- **Release** (`release.yml`): Triggered by `v*` tags, cross-compiles 5 platforms, creates GitHub Release
