package app

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Config struct {
	RemoteURL, Transport, ListenAddr                                       string
	Headers                                                                http.Header
	AuthHeaderName, AuthHeaderPrefix, AuthToken                            string
	AuthGrantType, AuthTokenURL, AuthClientID, AuthClientSecret, AuthScope string
	ToolInclude, ToolExclude                                               string
}

type headerValues struct {
	values []string
	cli    bool
}

func (h *headerValues) String() string { return strings.Join(h.values, ",") }
func (h *headerValues) Set(v string) error {
	if !h.cli {
		h.values = nil
		h.cli = true
	}
	h.values = append(h.values, v)
	return nil
}

func parseConfig(args []string, getenv func(string) string) (Config, bool, error) {
	c := Config{
		RemoteURL: getenv("MCP_FILTER_PROXY_REMOTE_URL"), Transport: valueOr(getenv("MCP_FILTER_PROXY_TRANSPORT"), "stdio"), ListenAddr: valueOr(getenv("MCP_FILTER_PROXY_LISTEN_ADDR"), ":8080"),
		AuthHeaderName: valueOr(getenv("MCP_FILTER_PROXY_AUTH_HEADER_NAME"), "Authorization"), AuthHeaderPrefix: valueOr(getenv("MCP_FILTER_PROXY_AUTH_HEADER_PREFIX"), "Bearer "), AuthToken: getenv("MCP_FILTER_PROXY_AUTH_TOKEN"),
		AuthGrantType: valueOr(getenv("MCP_FILTER_PROXY_AUTH_GRANT_TYPE"), "client_credentials"), AuthTokenURL: getenv("MCP_FILTER_PROXY_AUTH_TOKEN_URL"), AuthClientID: getenv("MCP_FILTER_PROXY_AUTH_CLIENT_ID"), AuthClientSecret: getenv("MCP_FILTER_PROXY_AUTH_CLIENT_SECRET"), AuthScope: getenv("MCP_FILTER_PROXY_AUTH_SCOPE"),
		ToolInclude: valueOr(getenv("MCP_FILTER_PROXY_TOOL_INCLUDE"), "*"), ToolExclude: getenv("MCP_FILTER_PROXY_TOOL_EXCLUDE"),
	}
	headers := headerValues{}
	if raw := getenv("MCP_FILTER_PROXY_HEADERS"); raw != "" {
		headers.values = strings.Split(raw, ",")
	}
	fs := flag.NewFlagSet("mcp-filter-proxy", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	fs.StringVar(&c.RemoteURL, "remote-url", c.RemoteURL, "Remote MCP streamable HTTP URL")
	fs.StringVar(&c.Transport, "transport", c.Transport, "Local transport: stdio or http")
	fs.StringVar(&c.ListenAddr, "listen-addr", c.ListenAddr, "Local HTTP listen address")
	fs.Var(&headers, "header", "Repeatable outbound header in Name=Value format")
	fs.StringVar(&c.AuthHeaderName, "auth-header-name", c.AuthHeaderName, "Authorization header name")
	fs.StringVar(&c.AuthHeaderPrefix, "auth-header-prefix", c.AuthHeaderPrefix, "Authorization value prefix")
	fs.StringVar(&c.AuthToken, "auth-token", c.AuthToken, "Static authorization token")
	fs.StringVar(&c.AuthGrantType, "auth-grant-type", c.AuthGrantType, "OAuth grant type")
	fs.StringVar(&c.AuthTokenURL, "auth-token-url", c.AuthTokenURL, "OAuth token endpoint")
	fs.StringVar(&c.AuthClientID, "auth-client-id", c.AuthClientID, "OAuth client ID")
	fs.StringVar(&c.AuthClientSecret, "auth-client-secret", c.AuthClientSecret, "OAuth client secret")
	fs.StringVar(&c.AuthScope, "auth-scope", c.AuthScope, "OAuth scope")
	fs.StringVar(&c.ToolInclude, "tool-include", c.ToolInclude, "Comma-separated tool include patterns")
	fs.StringVar(&c.ToolExclude, "tool-exclude", c.ToolExclude, "Comma-separated tool exclude patterns")
	showVersion := fs.Bool("version", false, "Print version and exit")
	if err := fs.Parse(args); err != nil {
		return Config{}, false, fmt.Errorf("configuration error: %w", err)
	}
	if *showVersion {
		return c, true, nil
	}
	var err error
	c.Headers, err = parseHeaders(headers.values)
	if err != nil {
		return Config{}, false, err
	}
	if err := c.validate(); err != nil {
		return Config{}, false, err
	}
	return c, false, nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
func parseHeaders(values []string) (http.Header, error) {
	h := make(http.Header)
	for _, value := range values {
		name, val, ok := strings.Cut(value, "=")
		if !ok || name == "" || !validHeaderName(name) {
			return nil, fmt.Errorf("configuration error: invalid --header %q; expected Name=Value", value)
		}
		h.Add(name, val)
	}
	return h, nil
}
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || strings.ContainsRune("!#$%&'*+-.^_`|~", c)) {
			return false
		}
	}
	return true
}
func (c Config) validate() error {
	if c.RemoteURL == "" {
		return fmt.Errorf("configuration error: --remote-url is required")
	}
	u, err := url.ParseRequestURI(c.RemoteURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("configuration error: invalid --remote-url")
	}
	if c.Transport != "stdio" && c.Transport != "http" {
		return fmt.Errorf("configuration error: unsupported --transport %q; expected stdio or http", c.Transport)
	}
	if c.AuthHeaderName == "" || !validHeaderName(c.AuthHeaderName) {
		return fmt.Errorf("configuration error: invalid --auth-header-name %q", c.AuthHeaderName)
	}
	oauth := c.AuthTokenURL != "" || c.AuthClientID != "" || c.AuthClientSecret != "" || c.AuthScope != ""
	if c.AuthToken != "" && oauth {
		return fmt.Errorf("configuration error: --auth-token cannot be used with OAuth client credentials configuration")
	}
	if oauth {
		if c.AuthTokenURL == "" || c.AuthClientID == "" || c.AuthClientSecret == "" {
			return fmt.Errorf("configuration error: OAuth requires --auth-token-url, --auth-client-id, and --auth-client-secret")
		}
		if c.AuthGrantType != "client_credentials" {
			return fmt.Errorf("configuration error: unsupported --auth-grant-type %q; only %q is supported", c.AuthGrantType, "client_credentials")
		}
	}
	if _, err := parsePatterns(c.ToolInclude, true); err != nil {
		return err
	}
	if _, err := parsePatterns(c.ToolExclude, false); err != nil {
		return err
	}
	return nil
}
func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[invalid URL]"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
