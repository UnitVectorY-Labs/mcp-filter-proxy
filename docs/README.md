---
layout: default
title: mcp-filter-proxy
nav_order: 1
permalink: /
---
# mcp-filter-proxy

`mcp-filter-proxy` gives a local MCP client a controlled path to a remote, streamable-HTTP MCP server. It is designed for clients that launch MCP servers over stdio, while still allowing an HTTP endpoint for shared local use.

The proxy keeps the remote server's tools, resources (including URI templates), and prompts available locally. Discovery is refreshed from the remote server, and upstream list-change notifications are mirrored locally. Its policy layer can add outbound headers, obtain and rotate OAuth client-credentials tokens, and expose only the tools that match your allow and deny patterns.

## Why use it

- Connect stdio-only MCP clients to remote MCP services.
- Keep tokens and tenant headers in the proxy configuration instead of each client configuration.
- Limit the visible and callable tool surface with simple, auditable wildcards.
- Use OAuth 2.0 client credentials with proactive in-memory refresh.

The default mode is stdio, so it fits directly into a client MCP command configuration. Streamable HTTP is available with `--transport http` when a local endpoint is more convenient.

## Quick start

```bash
mcp-filter-proxy --remote-url https://example.com/mcp
```

To allow only a safe tool family:

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --tool-include "search_*,read_*" \
  --tool-exclude "*delete*"
```

See [USAGE.md](USAGE.md) for every command-line option, environment variable, and authentication example.
