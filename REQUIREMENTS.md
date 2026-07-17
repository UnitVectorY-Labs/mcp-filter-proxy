# mcp-filter-proxy Technical Requirements

## 1. Overview

`mcp-filter-proxy` is a Go-based MCP server that exposes a local MCP endpoint and transparently proxies requests to a remote MCP server over streamable HTTP.

The primary purpose of the project is to allow local MCP clients to connect to remote MCP servers while adding practical proxy-layer controls that are not always available in clients or upstream MCP servers.

The initial implementation focuses on:

- Local MCP server support using `github.com/mark3labs/mcp-go`
- Local client transport over stdio as the primary mode
- Local client transport over streamable HTTP as a supported mode
- Remote MCP server transport over streamable HTTP only
- Transparent proxying of remote MCP capabilities
- Static outbound header injection
- First-class outbound authorization header support
- OAuth 2.0 client credentials token acquisition
- OAuth token refresh
- Tool filtering by include and exclude wildcard patterns
- Enforcement of tool filters during both tool discovery and tool invocation

The project is not intended to implement a new MCP server with its own native tools. It is intended to act as a transparent local proxy to a remote MCP server.

## 2. Goals

### 2.1 Primary Goals

1. Provide a local MCP server that can connect to a remote MCP server.
2. Use Go as the implementation language.
3. Use `github.com/mark3labs/mcp-go` for MCP server behavior where appropriate.
4. Support stdio as the default local-facing transport.
5. Support streamable HTTP as an additional local-facing transport.
6. Support streamable HTTP as the only remote-facing transport.
7. Transparently proxy MCP requests and responses between the local client and the remote server.
8. Support arbitrary outbound header injection to the remote MCP server.
9. Support a dedicated authorization header configuration model.
10. Support static authorization tokens.
11. Support OAuth 2.0 client credentials as the MVP OAuth flow.
12. Fetch OAuth tokens at startup when OAuth is configured.
13. Proactively refresh OAuth tokens before expiration.
14. Support tool filtering using include and exclude patterns.
15. Hide filtered tools from tool discovery.
16. Reject direct calls to filtered tools.
17. Pass through non-tool MCP capabilities transparently.

### 2.2 Non-Goals for MVP

The following are intentionally out of scope for the initial implementation:

1. SSE transport support.
2. OAuth authorization code flow.
3. OAuth device code flow.
4. OAuth refresh-token flow.
5. OAuth token introspection.
6. Dynamic client registration.
7. User-interactive browser login.
8. Per-user OAuth sessions.
9. Filtering resources, prompts, roots, sampling, logging, or other non-tool capabilities.
10. A configuration file format.
11. Persistent token storage.
12. A web UI.
13. Custom native tools implemented by `mcp-filter-proxy`.
14. Advanced glob syntax beyond basic `*` wildcard matching.
15. Regular expression based filtering.
16. Response transformation beyond tool filtering.
17. Request auditing as a first-class feature.
18. SSE backwards compatibility.

## 3. Architecture

### 3.1 High-Level Architecture

`mcp-filter-proxy` runs as a local MCP server. A local MCP client connects to it using stdio or streamable HTTP. The proxy then connects to a remote MCP server using streamable HTTP.

```text
Local MCP Client
        |
        | stdio or streamable HTTP
        v
mcp-filter-proxy
        |
        | streamable HTTP
        v
Remote MCP Server
```

### 3.2 Proxy Responsibility

The proxy should be transparent by default. For most MCP operations, the proxy should forward requests to the remote server and return the remote response to the local client.

Tool-related requests are the exception. Tool filtering must be applied to:

1. Tool discovery
2. Tool invocation

All other MCP capabilities should be proxied without filtering in the MVP.

### 3.3 Local Transport

The proxy must expose a local MCP server.

Supported local transports:

| Transport | Requirement |
|---|---|
| stdio | Required, default |
| streamable HTTP | Required |
| SSE | Not supported |

The nominal use case is stdio because the proxy is intended to be launched by a local MCP client as a command-line MCP server.

### 3.4 Remote Transport

The proxy must connect to the remote MCP server using streamable HTTP.

Supported remote transports:

| Transport | Requirement |
|---|---|
| streamable HTTP | Required |
| SSE | Not supported |

SSE is considered deprecated and must not be implemented for the MVP.

## 4. Implementation Language and Libraries

### 4.1 Language

The implementation must be written in Go.

### 4.2 MCP Library

The implementation should use:

```text
github.com/mark3labs/mcp-go
```

The implementation should use this library for the local MCP server behavior where possible.

### 4.3 Remote Client Behavior

The implementation must include or use a client capable of communicating with a remote MCP server over streamable HTTP.

The remote client layer should be isolated behind an internal abstraction so that request forwarding, header injection, authentication, and filtering logic are not tightly coupled to low-level transport details.

## 5. Configuration

### 5.1 Configuration Sources

The application must support configuration from:

1. CLI flags
2. Environment variables

CLI flags must take precedence over environment variables.

If both an environment variable and a CLI flag are provided for the same setting, the CLI flag value must be used.

### 5.2 Configuration File

Configuration files are not required for MVP.

### 5.3 Flag and Environment Variable Naming

Environment variables should have a clear prefix. Recommended prefix:

```text
MCP_FILTER_PROXY_
```

Example:

```text
--remote-url
MCP_FILTER_PROXY_REMOTE_URL
```

### 5.4 Required Configuration

The remote MCP server URL must be required.

Recommended flag:

```bash
--remote-url https://example.com/mcp
```

Recommended environment variable:

```bash
MCP_FILTER_PROXY_REMOTE_URL=https://example.com/mcp
```

### 5.5 Local Transport Configuration

The application should support selecting the local transport.

Recommended flag:

```bash
--transport stdio
```

Recommended environment variable:

```bash
MCP_FILTER_PROXY_TRANSPORT=stdio
```

Allowed values:

| Value | Meaning |
|---|---|
| `stdio` | Run as a stdio MCP server |
| `http` | Run as a streamable HTTP MCP server |

Default:

```text
stdio
```

### 5.6 Local HTTP Configuration

When local streamable HTTP transport is selected, the application must support configuring the listen address.

Recommended flag:

```bash
--listen-addr :8080
```

Recommended environment variable:

```bash
MCP_FILTER_PROXY_LISTEN_ADDR=:8080
```

Default:

```text
:8080
```

This setting is only relevant when `--transport http` is used.

## 6. Header Injection

### 6.1 Generic Header Injection

The proxy must support arbitrary outbound headers added to requests sent to the remote MCP server.

Recommended CLI flag:

```bash
--header "X-Tenant-ID=abc"
```

The flag must be repeatable.

Example:

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --header "X-Tenant-ID=abc" \
  --header "X-Environment=prod"
```

Recommended environment variable:

```bash
MCP_FILTER_PROXY_HEADERS="X-Tenant-ID=abc,X-Environment=prod"
```

### 6.2 Header Parsing

Generic headers must use the format:

```text
Name=Value
```

The first `=` separates the header name from the header value.

Examples:

```text
X-Tenant-ID=abc
X-Custom-Value=foo=bar
```

The second example must produce:

```text
X-Custom-Value: foo=bar
```

### 6.3 Invalid Header Configuration

Startup must fail if a configured header:

1. Has no `=`
2. Has an empty header name
3. Uses an invalid HTTP header name

Empty header values may be allowed.

### 6.4 Header Conflict Behavior

If the same header is specified multiple times, the implementation should preserve all values if the HTTP client supports multi-value headers.

For authorization-specific behavior, see the dedicated auth header section.

## 7. Authorization Header Support

### 7.1 Dedicated Authorization Configuration

The proxy must support first-class authorization header configuration separately from generic `--header` injection.

Required flags:

```bash
--auth-header-name Authorization
--auth-header-prefix "Bearer "
--auth-token "abc123"
```

Required environment variables:

```bash
MCP_FILTER_PROXY_AUTH_HEADER_NAME=Authorization
MCP_FILTER_PROXY_AUTH_HEADER_PREFIX="Bearer "
MCP_FILTER_PROXY_AUTH_TOKEN=abc123
```

### 7.2 Defaults

The following defaults must be used:

| Setting | Default |
|---|---|
| `auth-header-name` | `Authorization` |
| `auth-header-prefix` | `Bearer ` |

This allows the common static token case to be configured with only:

```bash
--auth-token "abc123"
```

which produces the outbound header:

```text
Authorization: Bearer abc123
```

### 7.3 Static Token Rendering

When `--auth-token` is configured, the proxy must send the following header to the remote MCP server:

```text
<auth-header-name>: <auth-header-prefix><auth-token>
```

Example:

```bash
--auth-header-name Authorization \
--auth-header-prefix "Bearer " \
--auth-token abc123
```

Result:

```text
Authorization: Bearer abc123
```

### 7.4 Custom Header Name

The user may override the auth header name.

Example:

```bash
--auth-header-name X-API-Token \
--auth-header-prefix "" \
--auth-token abc123
```

Result:

```text
X-API-Token: abc123
```

### 7.5 Generic Header and Auth Header Interaction

The dedicated auth header configuration should be applied after generic headers.

If a generic header and the dedicated auth header use the same header name, the dedicated auth header should take precedence.

Recommended behavior:

1. Apply generic headers.
2. Apply dedicated auth header.
3. If the dedicated auth header name already exists, replace the previous value for that header.

## 8. OAuth 2.0 Client Credentials Support

### 8.1 OAuth MVP Scope

The MVP must support OAuth 2.0 client credentials.

Other OAuth flows are not required for MVP.

### 8.2 Required OAuth Flags

The proxy must support the following OAuth flags:

```bash
--auth-grant-type client_credentials
--auth-token-url https://auth.example.com/oauth/token
--auth-client-id my-client-id
--auth-client-secret my-client-secret
```

The proxy must support corresponding environment variables:

```bash
MCP_FILTER_PROXY_AUTH_GRANT_TYPE=client_credentials
MCP_FILTER_PROXY_AUTH_TOKEN_URL=https://auth.example.com/oauth/token
MCP_FILTER_PROXY_AUTH_CLIENT_ID=my-client-id
MCP_FILTER_PROXY_AUTH_CLIENT_SECRET=my-client-secret
```

### 8.3 OAuth Defaults

The default grant type must be:

```text
client_credentials
```

If OAuth is configured and no grant type is specified, `client_credentials` must be assumed.

### 8.4 Optional OAuth Scope

The proxy must support an optional OAuth scope value.

Recommended flag:

```bash
--auth-scope "mcp:read mcp:call"
```

Recommended environment variable:

```bash
MCP_FILTER_PROXY_AUTH_SCOPE="mcp:read mcp:call"
```

When provided, the token request body must include:

```text
scope=mcp:read mcp:call
```

No additional OAuth features should be added as part of scope support.

### 8.5 OAuth Token Request Format

For MVP, OAuth client credentials must be sent in the token request body.

The token request must use:

```http
POST <auth-token-url>
Content-Type: application/x-www-form-urlencoded
```

Required form fields:

```text
grant_type=client_credentials
client_id=<auth-client-id>
client_secret=<auth-client-secret>
```

Optional form field:

```text
scope=<auth-scope>
```

HTTP Basic authentication for client credentials is not required for MVP.

This is an explicit MVP assumption and may be revisited later.

### 8.6 OAuth Token Response

The proxy must support a standard OAuth token response containing:

```json
{
  "access_token": "token-value",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

Required fields:

| Field | Requirement |
|---|---|
| `access_token` | Required |
| `expires_in` | Required for proactive refresh |

The proxy may ignore `token_type` for MVP because the outbound header rendering is controlled by `--auth-header-prefix`.

### 8.7 Startup Token Fetch

When OAuth is configured, the proxy must fetch an access token at startup.

If token acquisition fails, startup must fail.

This is required because the token is needed to initialize the proxy’s usable connection to the remote MCP server.

### 8.8 OAuth Token Rendering

When OAuth is configured, the fetched access token must be rendered using the same dedicated auth header model as static tokens:

```text
<auth-header-name>: <auth-header-prefix><access_token>
```

With defaults, this produces:

```text
Authorization: Bearer <access_token>
```

### 8.9 Static Token and OAuth Mutual Exclusivity

Static auth token and OAuth configuration are mutually exclusive.

The application must fail at startup if both are configured.

Invalid example:

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --auth-token static-token \
  --auth-token-url https://auth.example.com/oauth/token \
  --auth-client-id my-client-id \
  --auth-client-secret my-client-secret
```

Expected result:

```text
configuration error: --auth-token cannot be used with OAuth client credentials configuration
```

### 8.10 OAuth Configuration Completeness

If any OAuth client credentials setting is configured, the proxy must validate that all required OAuth settings are present.

Required for OAuth:

1. `auth-token-url`
2. `auth-client-id`
3. `auth-client-secret`
4. `auth-grant-type`, defaulting to `client_credentials`

If `auth-grant-type` is set to anything other than `client_credentials`, startup must fail for MVP.

Example error:

```text
configuration error: unsupported auth grant type "authorization_code"; only "client_credentials" is supported
```

### 8.11 Proactive Token Refresh

The proxy must proactively refresh OAuth tokens before expiration.

The refresh schedule should be based on the `expires_in` value returned by the token endpoint.

Recommended behavior:

1. Record the token acquisition time.
2. Calculate expiration time using `expires_in`.
3. Schedule refresh before expiration.
4. Use a refresh buffer to avoid using tokens too close to expiration.

Recommended refresh buffer:

```text
60 seconds
```

If `expires_in` is shorter than the refresh buffer, refresh should occur earlier using a reasonable minimum delay to avoid a tight loop.

### 8.12 Refresh Failure Behavior

If proactive refresh fails after startup:

1. The proxy must retry refresh with backoff.
2. The proxy must continue using the previous token while it is still valid.
3. Once the previous token expires, upstream requests requiring the token may fail until refresh succeeds.
4. The proxy must not silently switch to unauthenticated requests.

### 8.13 Refresh Backoff

The implementation should use bounded exponential backoff for refresh failures.

Recommended default behavior:

| Attempt | Delay |
|---|---|
| 1 | 1 second |
| 2 | 2 seconds |
| 3 | 4 seconds |
| 4 | 8 seconds |
| 5 and later | 30 seconds max |

Jitter may be added to avoid synchronized retries.

### 8.14 Reactive Refresh

Reactive refresh on upstream `401 Unauthorized` is not required for MVP.

The MVP refresh strategy is proactive only.

## 9. Tool Filtering

### 9.1 Purpose

Tool filtering allows the user to control which remote MCP tools are visible and callable through the local proxy.

Filtering applies only to tools.

Filtering does not apply to:

1. Resources
2. Prompts
3. Roots
4. Logging
5. Sampling
6. Other non-tool MCP capabilities

### 9.2 Include and Exclude Flags

The proxy must support include and exclude patterns.

Recommended flags:

```bash
--tool-include "*"
--tool-exclude ""
```

Recommended environment variables:

```bash
MCP_FILTER_PROXY_TOOL_INCLUDE="*"
MCP_FILTER_PROXY_TOOL_EXCLUDE=""
```

### 9.3 Defaults

Default include pattern:

```text
*
```

Default exclude pattern:

```text
empty string
```

The default behavior must include all tools and exclude no tools.

### 9.4 Multiple Patterns

Both include and exclude settings must support multiple comma-separated patterns.

Examples:

```bash
--tool-include "github_*,slack_*"
```

```bash
--tool-exclude "*delete*,*admin*"
```

Whitespace around comma-separated patterns should be trimmed.

Empty patterns should be ignored.

### 9.5 Wildcard Matching

The MVP must support basic `*` wildcard matching.

Examples:

| Pattern | Matches |
|---|---|
| `*` | Any tool name |
| `github_*` | Tool names starting with `github_` |
| `*_search` | Tool names ending with `_search` |
| `*delete*` | Tool names containing `delete` |
| `exact_tool_name` | Only `exact_tool_name` |

The matching model should not support full regular expressions in MVP.

### 9.6 Case Sensitivity

Tool pattern matching should be case-sensitive.

This avoids surprising behavior and preserves exact MCP tool names.

### 9.7 Filtering Logic

A tool is visible and callable only if:

```text
matches at least one include pattern
AND
does not match any exclude pattern
```

Exclude patterns always win.

### 9.8 Examples

Default behavior:

```bash
--tool-include "*"
--tool-exclude ""
```

Result:

```text
All tools are visible and callable.
```

Exclude dangerous tools:

```bash
--tool-exclude "*delete*,*admin*"
```

Result:

```text
All tools are visible and callable except tools matching *delete* or *admin*.
```

Only include GitHub tools:

```bash
--tool-include "github_*"
```

Result:

```text
Only tools starting with github_ are visible and callable.
```

Include broad pattern with explicit exclusions:

```bash
--tool-include "github_*"
--tool-exclude "github_delete_repo,github_admin_*"
```

Result:

```text
All github_* tools are visible and callable except github_delete_repo and github_admin_*.
```

Multiple includes:

```bash
--tool-include "github_*,slack_*"
```

Result:

```text
Only tools starting with github_ or slack_ are visible and callable.
```

### 9.9 Tool Discovery Filtering

When the local MCP client lists available tools, the proxy must:

1. Retrieve the tool list from the remote MCP server.
2. Apply the include and exclude filters.
3. Return only tools that pass the filter.

Filtered tools must not appear in the list returned to the local client.

### 9.10 Tool Invocation Enforcement

When the local MCP client attempts to call a tool, the proxy must:

1. Inspect the requested tool name.
2. Apply the same include and exclude filter logic.
3. Forward the call to the remote server only if the tool passes the filter.
4. Reject the call locally if the tool does not pass the filter.

This is required even if the tool was not listed to the client.

### 9.11 Blocked Tool Error

If a filtered tool is called directly, the proxy should return a generic not-found style MCP error.

The error should not disclose that the tool exists upstream.

Recommended behavior:

```text
tool not found
```

Avoid messages such as:

```text
tool is blocked by proxy filter
```

because they disclose that the tool may exist.

### 9.12 Filter Evaluation Source

The tool call enforcement should evaluate the requested tool name directly against the configured patterns.

It should not rely only on a cached filtered tool list because remote tool availability may change.

A cache may be used for performance, but authorization must be correct even if the cache is stale.

## 10. Transparent Proxying

### 10.1 General Behavior

For MCP requests not related to tool filtering, the proxy should forward requests to the remote MCP server and return responses to the local client.

The proxy should avoid changing request and response payloads unless necessary for transport, authentication, or tool filtering.

### 10.2 Tools

Tool behavior is special because filtering must be applied.

Required behavior:

| MCP behavior | Proxy behavior |
|---|---|
| List tools | Forward upstream, filter response |
| Call allowed tool | Forward upstream |
| Call filtered tool | Reject locally with generic not-found error |

### 10.3 Resources

Resources must be transparently proxied in MVP.

No include or exclude filtering applies to resources.

### 10.4 Prompts

Prompts must be transparently proxied in MVP.

No include or exclude filtering applies to prompts.

### 10.5 Other Capabilities

Other MCP capabilities should be transparently proxied when supported by the underlying MCP library and remote MCP server.

No filtering applies to these capabilities in MVP.

## 11. CLI Requirements

### 11.1 Required CLI Flags

| Flag | Required | Default | Description |
|---|---:|---|---|
| `--remote-url` | Yes | none | Remote MCP streamable HTTP URL |
| `--transport` | No | `stdio` | Local transport: `stdio` or `http` |
| `--listen-addr` | No | `:8080` | Local HTTP listen address |
| `--header` | No | none | Repeatable outbound header in `Name=Value` format |
| `--auth-header-name` | No | `Authorization` | Header name for static or OAuth token |
| `--auth-header-prefix` | No | `Bearer ` | Prefix prepended to static or OAuth token |
| `--auth-token` | No | none | Static token value |
| `--auth-grant-type` | No | `client_credentials` | OAuth grant type |
| `--auth-token-url` | No | none | OAuth token endpoint URL |
| `--auth-client-id` | No | none | OAuth client ID |
| `--auth-client-secret` | No | none | OAuth client secret |
| `--auth-scope` | No | none | Optional OAuth scope |
| `--tool-include` | No | `*` | Comma-separated include patterns |
| `--tool-exclude` | No | empty | Comma-separated exclude patterns |

### 11.2 Environment Variables

| Environment Variable | Flag Equivalent |
|---|---|
| `MCP_FILTER_PROXY_REMOTE_URL` | `--remote-url` |
| `MCP_FILTER_PROXY_TRANSPORT` | `--transport` |
| `MCP_FILTER_PROXY_LISTEN_ADDR` | `--listen-addr` |
| `MCP_FILTER_PROXY_HEADERS` | `--header` |
| `MCP_FILTER_PROXY_AUTH_HEADER_NAME` | `--auth-header-name` |
| `MCP_FILTER_PROXY_AUTH_HEADER_PREFIX` | `--auth-header-prefix` |
| `MCP_FILTER_PROXY_AUTH_TOKEN` | `--auth-token` |
| `MCP_FILTER_PROXY_AUTH_GRANT_TYPE` | `--auth-grant-type` |
| `MCP_FILTER_PROXY_AUTH_TOKEN_URL` | `--auth-token-url` |
| `MCP_FILTER_PROXY_AUTH_CLIENT_ID` | `--auth-client-id` |
| `MCP_FILTER_PROXY_AUTH_CLIENT_SECRET` | `--auth-client-secret` |
| `MCP_FILTER_PROXY_AUTH_SCOPE` | `--auth-scope` |
| `MCP_FILTER_PROXY_TOOL_INCLUDE` | `--tool-include` |
| `MCP_FILTER_PROXY_TOOL_EXCLUDE` | `--tool-exclude` |

### 11.3 Precedence

CLI flags must take precedence over environment variables.

Example:

```bash
export MCP_FILTER_PROXY_TOOL_INCLUDE="github_*"

mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --tool-include "slack_*"
```

Effective include pattern:

```text
slack_*
```

## 12. Example Usage

### 12.1 Basic Proxy

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp
```

Behavior:

```text
Local transport: stdio
Remote transport: streamable HTTP
Tools: all included
Auth: none
```

### 12.2 Static Bearer Token

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --auth-token "$TOKEN"
```

Outbound header:

```text
Authorization: Bearer <TOKEN>
```

### 12.3 Custom Auth Header

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --auth-header-name X-API-Key \
  --auth-header-prefix "" \
  --auth-token "$API_KEY"
```

Outbound header:

```text
X-API-Key: <API_KEY>
```

### 12.4 OAuth Client Credentials

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --auth-token-url https://auth.example.com/oauth/token \
  --auth-client-id "$CLIENT_ID" \
  --auth-client-secret "$CLIENT_SECRET"
```

Behavior:

```text
grant_type defaults to client_credentials
Authorization header defaults to Authorization
Authorization prefix defaults to Bearer 
Token is fetched at startup
Token is refreshed proactively
```

### 12.5 OAuth Client Credentials With Scope

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --auth-token-url https://auth.example.com/oauth/token \
  --auth-client-id "$CLIENT_ID" \
  --auth-client-secret "$CLIENT_SECRET" \
  --auth-scope "mcp:read mcp:call"
```

### 12.6 Generic Headers

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --header "X-Tenant-ID=tenant-123" \
  --header "X-Environment=prod"
```

### 12.7 Exclude Dangerous Tools

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --tool-exclude "*delete*,*admin*"
```

### 12.8 Include Only Specific Tool Families

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --tool-include "github_*,slack_*"
```

### 12.9 Include and Exclude Together

```bash
mcp-filter-proxy \
  --remote-url https://example.com/mcp \
  --tool-include "github_*" \
  --tool-exclude "github_delete_repo,github_admin_*"
```

### 12.10 Local Streamable HTTP Mode

```bash
mcp-filter-proxy \
  --transport http \
  --listen-addr :8080 \
  --remote-url https://example.com/mcp
```

## 13. Validation Requirements

### 13.1 Startup Validation

The application must validate configuration at startup.

Startup must fail if:

1. `--remote-url` is missing.
2. `--transport` is not `stdio` or `http`.
3. A `--header` value is malformed.
4. `--auth-token` is used together with OAuth configuration.
5. OAuth is partially configured.
6. OAuth grant type is unsupported.
7. OAuth token acquisition fails.
8. `--auth-header-name` is invalid.
9. Tool include or exclude patterns cannot be parsed.

### 13.2 OAuth Validation

OAuth configuration is considered present if any of the following are set:

1. `auth-token-url`
2. `auth-client-id`
3. `auth-client-secret`
4. `auth-scope`

If OAuth configuration is present, all required OAuth values must be present.

Required values:

1. `auth-token-url`
2. `auth-client-id`
3. `auth-client-secret`

`auth-grant-type` defaults to `client_credentials`.

### 13.3 Auth Validation

Static token auth is considered present if `auth-token` is set.

Static token auth and OAuth auth must not both be present.

### 13.4 Pattern Validation

Tool include and exclude values must be parsed into lists of patterns.

The empty exclude value must be valid and mean “exclude nothing.”

The empty include value should be invalid unless explicitly normalized to `*`.

Recommended behavior:

```text
empty include means *
empty exclude means exclude nothing
```

## 14. Error Handling

### 14.1 Configuration Errors

Configuration errors must be clear and actionable.

Example:

```text
configuration error: --remote-url is required
```

Example:

```text
configuration error: --auth-token cannot be used with OAuth client credentials configuration
```

Example:

```text
configuration error: unsupported --auth-grant-type "authorization_code"; only "client_credentials" is supported
```

### 14.2 OAuth Startup Errors

If OAuth token acquisition fails at startup, the application must exit with a non-zero status.

Example:

```text
oauth error: failed to acquire access token: token endpoint returned 401 Unauthorized
```

Secrets must not be printed.

### 14.3 OAuth Refresh Errors

If token refresh fails after startup, the proxy should log the error without terminating immediately.

The proxy should retry with backoff.

The proxy should continue using the existing token while it remains valid.

### 14.4 Remote MCP Errors

Remote MCP errors should generally be passed through to the local client unchanged.

### 14.5 Filtered Tool Errors

Filtered tool calls must return a generic not-found style error.

The error must not disclose that the tool exists upstream.

Recommended message:

```text
tool not found
```

## 15. Logging Requirements

### 15.1 General Logging

The application should log operational events useful for debugging.

Recommended log events:

1. Startup configuration summary without secrets
2. Selected local transport
3. Remote URL host without sensitive query parameters
4. OAuth token acquisition success
5. OAuth token refresh success
6. OAuth token refresh failure
7. Remote connection failure
8. Tool filtering summary
9. Blocked tool call attempt without disclosing unnecessary sensitive arguments

### 15.2 Secret Redaction

The application must not log secrets.

The following values must be redacted:

1. `auth-token`
2. `auth-client-secret`
3. OAuth access token
4. Authorization header values
5. Any header value whose name is `Authorization`
6. Any header value whose name contains `token`, `secret`, `key`, or `password`, case-insensitive

### 15.3 Suggested Redaction Format

```text
Authorization: [REDACTED]
```

## 16. Security Requirements

### 16.1 No Secret Logging

Secrets must never be logged in plaintext.

### 16.2 No Tool Disclosure Through Filtering

Filtered tools must not be disclosed through:

1. Tool listing
2. Tool call error messages

### 16.3 Header Injection Safety

The implementation must validate header names and prevent malformed headers.

### 16.4 OAuth Token Safety

OAuth access tokens must be kept in memory only for MVP.

Persistent token storage is not required.

### 16.5 Transport Security

The proxy should support HTTPS remote URLs.

The implementation should not disable TLS verification by default.

A flag to skip TLS verification is not required for MVP.

## 17. Testing Requirements

### 17.1 Unit Tests

Unit tests should cover:

1. CLI and environment variable precedence
2. Header parsing
3. Auth header rendering
4. Static token and OAuth mutual exclusivity
5. OAuth form body construction
6. OAuth token response parsing
7. OAuth refresh scheduling calculation
8. Include pattern parsing
9. Exclude pattern parsing
10. Wildcard matching
11. Tool filter decision logic
12. Generic not-found error behavior for blocked tool calls

### 17.2 Tool Filter Test Cases

Required test cases:

| Include | Exclude | Tool Name | Expected |
|---|---|---|---|
| `*` | empty | `search` | allowed |
| `github_*` | empty | `github_search` | allowed |
| `github_*` | empty | `slack_search` | blocked |
| `*` | `*delete*` | `github_delete_repo` | blocked |
| `github_*` | `github_delete_repo` | `github_delete_repo` | blocked |
| `github_*,slack_*` | empty | `slack_send` | allowed |
| `github_*,slack_*` | empty | `jira_create` | blocked |
| `*` | `admin_*` | `admin_reset` | blocked |

### 17.3 Integration Tests

Integration tests should cover:

1. Local stdio to remote streamable HTTP proxying.
2. Local streamable HTTP to remote streamable HTTP proxying.
3. Remote `tools/list` response is filtered.
4. Allowed `tools/call` is proxied.
5. Blocked `tools/call` is rejected locally.
6. Static authorization header is sent upstream.
7. Generic headers are sent upstream.
8. OAuth token is fetched at startup.
9. OAuth token is used in upstream requests.
10. OAuth token refresh occurs proactively.

### 17.4 Failure Tests

Failure tests should cover:

1. Missing remote URL.
2. Invalid local transport.
3. Malformed header.
4. OAuth token endpoint failure at startup.
5. OAuth refresh failure after startup.
6. Static token and OAuth configured together.
7. Unsupported OAuth grant type.
8. Blocked tool call does not reach the remote server.

## 18. Documentation Requirements

The project README must document:

1. What `mcp-filter-proxy` does.
2. Primary use case.
3. Local transport options.
4. Remote transport support.
5. Explicit statement that SSE is not supported.
6. Basic usage.
7. Static auth token usage.
8. OAuth client credentials usage.
9. Generic header injection.
10. Tool include and exclude filtering.
11. CLI flags.
12. Environment variables.
13. CLI flag precedence over environment variables.
14. Security considerations.
15. MVP OAuth assumptions.
16. Examples for common MCP client configurations.

## 19. README Positioning Statement

Suggested README summary:

```text
mcp-filter-proxy is a local MCP proxy server written in Go. It lets local MCP clients connect to remote MCP servers over streamable HTTP while adding outbound header injection, OAuth client credentials support, and tool-level include/exclude filtering.
```

Suggested longer description:

```text
mcp-filter-proxy is intended for environments where an MCP client expects to launch a local MCP server, but the actual tools are provided by a remote MCP server. The proxy runs locally, speaks MCP to the client, connects to the remote server over streamable HTTP, and transparently forwards MCP capabilities. It can inject static headers, acquire OAuth client credentials tokens, and restrict which remote tools are visible or callable.
```

## 20. Open Design Assumptions

The following assumptions are accepted for MVP and may be revisited later:

1. Remote transport is streamable HTTP only.
2. SSE is not supported.
3. OAuth support is limited to client credentials.
4. OAuth client credentials are sent in the token request body.
5. OAuth Basic client authentication is not supported in MVP.
6. OAuth tokens are stored only in memory.
7. OAuth refresh is proactive only.
8. Tool filtering is case-sensitive.
9. Tool filtering supports only basic `*` wildcards.
10. Tool filtering applies only to tools.
11. Non-tool MCP capabilities are transparently proxied.
12. Configuration is provided through CLI flags and environment variables only.
13. CLI flags take precedence over environment variables.

## 21. Suggested Internal Package Structure

This section is non-binding but provides a recommended implementation structure.

```text
cmd/mcp-filter-proxy/
  main.go

internal/config/
  config.go
  flags.go
  env.go
  validation.go

internal/auth/
  static.go
  oauth.go
  token_manager.go

internal/headers/
  headers.go
  redaction.go

internal/filter/
  pattern.go
  tool_filter.go

internal/proxy/
  proxy.go
  tools.go
  passthrough.go

internal/remote/
  client.go
  streamable_http.go

internal/server/
  stdio.go
  http.go

internal/logging/
  logging.go
```

## 22. Acceptance Criteria

The MVP is complete when:

1. The binary can run as a local stdio MCP server.
2. The binary can run as a local streamable HTTP MCP server.
3. The proxy can connect to a remote MCP server over streamable HTTP.
4. The proxy can forward non-tool MCP requests transparently.
5. The proxy can list remote tools and filter them before returning them locally.
6. The proxy can call allowed remote tools.
7. The proxy rejects blocked tool calls locally.
8. Blocked tool calls return a generic not-found style error.
9. Generic outbound headers can be configured.
10. Static auth token headers can be configured.
11. OAuth client credentials can be configured.
12. OAuth tokens are fetched at startup.
13. Startup fails if OAuth token acquisition fails.
14. OAuth tokens are proactively refreshed.
15. Refresh failures retry with backoff.
16. CLI flags override environment variables.
17. Configuration errors are clear and actionable.
18. Secrets are not logged.
19. README documents usage and configuration.
20. Unit and integration tests cover the required behavior.
:::