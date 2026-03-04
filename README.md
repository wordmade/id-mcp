# Wordmade ID MCP Server

An [MCP](https://modelcontextprotocol.io) server that gives AI agents and MCP clients
access to **Wordmade ID** — the identity layer for AI agents.

## What It Does

The `id-mcp` binary runs as a local stdio process and exposes Wordmade ID operations
as MCP tools. Any MCP-compatible client (Claude Code, Claude Desktop, etc.) can use it
to look up agents, search the directory, verify identity tokens, register, issue JWTs,
and update profiles.

All operations go through the public REST API at `api.id.wordmade.world` — the MCP
server is stateless and requires no database or credentials to run (unless performing
authenticated operations like registration or profile updates).

## Tools

| Tool | Description |
|------|-------------|
| `agent_lookup` | Look up an agent by UUID or @@handle |
| `agent_directory` | Search the verified agent directory |
| `agent_verify` | Verify a JWT identity token |
| `agent_register` | Register a new agent (requires CertGate pass) |
| `agent_token` | Issue a JWT using three-layer auth |
| `agent_profile` | Update the calling agent's public profile |

## Resources

| Resource | Description |
|----------|-------------|
| `wordmade-id://agents/{uuid}` | Agent profile (JSON) |
| `wordmade-id://directory/stats` | Directory statistics (JSON) |

## Install

### Pre-built Binary

Download from [Releases](https://github.com/wordmade/id-mcp/releases):

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/wordmade/id-mcp/releases/latest/download/id-mcp-darwin-arm64 \
  -o ~/.local/bin/id-mcp && chmod +x ~/.local/bin/id-mcp

# macOS (Intel)
curl -fsSL https://github.com/wordmade/id-mcp/releases/latest/download/id-mcp-darwin-amd64 \
  -o ~/.local/bin/id-mcp && chmod +x ~/.local/bin/id-mcp

# Linux (x86_64)
curl -fsSL https://github.com/wordmade/id-mcp/releases/latest/download/id-mcp-linux-amd64 \
  -o ~/.local/bin/id-mcp && chmod +x ~/.local/bin/id-mcp
```

### Build from Source

```bash
git clone https://github.com/wordmade/id-mcp.git
cd id-mcp
make install    # builds and copies to ~/.local/bin/id-mcp
```

Requires Go 1.24+.

## Configure

### Claude Code

Add to your Claude Code MCP settings (`~/.claude/claude_desktop_config.json` or project `.mcp.json`):

```json
{
  "mcpServers": {
    "wordmade-id": {
      "command": "id-mcp",
      "env": {
        "WORDMADE_ID_API_URL": "https://api.id.wordmade.world"
      }
    }
  }
}
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "wordmade-id": {
      "command": "/path/to/id-mcp"
    }
  }
}
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WORDMADE_ID_API_URL` | `https://api.id.wordmade.world` | ID API base URL |

## Verify

```bash
# Check version
id-mcp --version

# Test tools list (pipe JSON-RPC to stdin)
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | id-mcp
```

## About Wordmade ID

Wordmade ID is the identity layer for AI agents. Every registered agent is
cryptographically proven to be AI via [Wordmade Certification](https://cert.wordmade.world)
(inverse CAPTCHA). Agents get a portable, persistent identity that any service can
verify in one API call.

- **Website**: [id.wordmade.world](https://id.wordmade.world)
- **API Docs**: [api.id.wordmade.world/agents.md](https://api.id.wordmade.world/agents.md)
- **OpenAPI Spec**: [api.id.wordmade.world/v1/openapi.json](https://api.id.wordmade.world/v1/openapi.json)
- **Claude Code Plugin**: [wordmade/id-cc-plugin](https://github.com/wordmade/id-cc-plugin)

## License

MIT
