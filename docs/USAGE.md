---
layout: default
title: Usage
nav_order: 3
permalink: /usage
---
# Usage

`mcp-filter-proxy` connects to one remote MCP server over streamable HTTP. The remote URL is required; all other configuration is optional.

```bash
mcp-filter-proxy --remote-url https://example.com/mcp
```

Configuration can come from flags or environment variables. A supplied flag always overrides its matching environment variable. The `--header` flag is repeatable; when it is supplied, its complete set replaces `MCP_FILTER_PROXY_HEADERS`.

## Transport

| Flag | Environment variable | Default | Meaning |
|---|---|---|---|
| `--remote-url` | `MCP_FILTER_PROXY_REMOTE_URL` | required | Remote streamable HTTP MCP endpoint. |
| `--transport` | `MCP_FILTER_PROXY_TRANSPORT` | `stdio` | Local transport: `stdio` or `http`. |
| `--listen-addr` | `MCP_FILTER_PROXY_LISTEN_ADDR` | `:8080` | Address for local HTTP mode. |

HTTP mode exposes the MCP endpoint at `/mcp`:

```bash
mcp-filter-proxy \
  --transport http \
  --listen-addr :8080 \
  --remote-url https://example.com/mcp
```

## Outbound headers

Use `--header Name=Value` to send static headers to the remote MCP server. It may be repeated. The first `=` separates the name and value, so values may themselves contain `=`.

```bash
mcp-filter-proxy --remote-url https://example.com/mcp \
  --header 'X-Tenant-ID=tenant-123' \
  --header 'X-Custom-Value=one=two'
```

Alternatively, set `MCP_FILTER_PROXY_HEADERS` to a comma-separated list:

```bash
export MCP_FILTER_PROXY_HEADERS='X-Tenant-ID=tenant-123,X-Environment=production'
```

Malformed header names or entries without `=` stop startup. Repeated generic headers retain their values. The dedicated authorization header below is applied last, so it replaces a generic header with the same name.

## Static authentication

| Flag | Environment variable | Default |
|---|---|---|
| `--auth-header-name` | `MCP_FILTER_PROXY_AUTH_HEADER_NAME` | `Authorization` |
| `--auth-header-prefix` | `MCP_FILTER_PROXY_AUTH_HEADER_PREFIX` | `Bearer ` |
| `--auth-token` | `MCP_FILTER_PROXY_AUTH_TOKEN` | none |

For a bearer token, configure only the token:

```bash
mcp-filter-proxy --remote-url https://example.com/mcp --auth-token "$TOKEN"
```

This sends `Authorization: Bearer <token>`. For an API-key header:

```bash
mcp-filter-proxy --remote-url https://example.com/mcp \
  --auth-header-name X-API-Key \
  --auth-header-prefix '' \
  --auth-token "$API_KEY"
```

## OAuth client credentials

| Flag | Environment variable | Default |
|---|---|---|
| `--auth-grant-type` | `MCP_FILTER_PROXY_AUTH_GRANT_TYPE` | `client_credentials` |
| `--auth-token-url` | `MCP_FILTER_PROXY_AUTH_TOKEN_URL` | none |
| `--auth-client-id` | `MCP_FILTER_PROXY_AUTH_CLIENT_ID` | none |
| `--auth-client-secret` | `MCP_FILTER_PROXY_AUTH_CLIENT_SECRET` | none |
| `--auth-scope` | `MCP_FILTER_PROXY_AUTH_SCOPE` | none |

OAuth requires the token URL, client ID, and client secret. The proxy posts those fields in an `application/x-www-form-urlencoded` client-credentials request at startup, keeps the access token in memory, and refreshes it before expiration. It retries refresh failures with bounded exponential backoff and never falls back to unauthenticated requests.

```bash
mcp-filter-proxy --remote-url https://example.com/mcp \
  --auth-token-url https://auth.example.com/oauth/token \
  --auth-client-id "$CLIENT_ID" \
  --auth-client-secret "$CLIENT_SECRET" \
  --auth-scope 'mcp:read mcp:call'
```

Static `--auth-token` and OAuth settings cannot be combined.

## Tool filtering

| Flag | Environment variable | Default |
|---|---|---|
| `--tool-include` | `MCP_FILTER_PROXY_TOOL_INCLUDE` | `*` |
| `--tool-exclude` | `MCP_FILTER_PROXY_TOOL_EXCLUDE` | empty |

Both values accept comma-separated, case-sensitive patterns using only `*` as a wildcard. A tool must match an include pattern and not match an exclude pattern. Excludes always win. Filtered tools are omitted from discovery and a direct call returns a generic `tool not found` error.

Tool discovery is refreshed from the remote MCP server for each local listing and also when the remote server sends a list-change notification. Resources, resource templates, and prompts are passed through without filtering.

```bash
# Expose GitHub tools except destructive or administrative tools.
mcp-filter-proxy --remote-url https://example.com/mcp \
  --tool-include 'github_*' \
  --tool-exclude '*delete*,github_admin_*'
```

## Diagnostics

`--version` prints the build version. Operational logs are written to standard error and deliberately omit tokens, authorization values, client secrets, and sensitive header values.
