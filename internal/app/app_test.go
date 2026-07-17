package app

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--version"}, env(nil), "1.2.3", &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "mcp-filter-proxy 1.2.3 ") {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}
func TestCLIOverridesEnvironment(t *testing.T) {
	c, _, err := parseConfig([]string{"--remote-url", "https://flag.example/mcp", "--tool-include", "slack_*", "--header", "X-CLI=yes"}, env(map[string]string{"MCP_FILTER_PROXY_REMOTE_URL": "https://env.example/mcp", "MCP_FILTER_PROXY_TOOL_INCLUDE": "github_*", "MCP_FILTER_PROXY_HEADERS": "X-Env=yes"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.RemoteURL != "https://flag.example/mcp" || c.ToolInclude != "slack_*" {
		t.Fatalf("CLI did not override environment: %#v", c)
	}
	if c.Headers.Get("X-Env") != "" || c.Headers.Get("X-CLI") != "yes" {
		t.Fatalf("headers = %#v", c.Headers)
	}
}
func TestParseHeadersAndAuthPrecedence(t *testing.T) {
	h, err := parseHeaders([]string{"X-Value=one=two", "X-Multi=first", "X-Multi=second"})
	if err != nil {
		t.Fatal(err)
	}
	if h.Get("X-Value") != "one=two" || len(h.Values("X-Multi")) != 2 {
		t.Fatalf("headers = %#v", h)
	}
	if _, err := parseHeaders([]string{"bad header=value"}); err == nil {
		t.Fatal("invalid header accepted")
	}
	p := &proxy{config: Config{AuthHeaderName: "Authorization", AuthHeaderPrefix: "Bearer "}, auth: &tokenManager{config: Config{AuthToken: "secret", AuthHeaderPrefix: "Bearer "}}}
	rt := headerTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
	}), headers: http.Header{"Authorization": {"other"}}, auth: p}
	req, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
}
func TestConfigurationFailures(t *testing.T) {
	if _, _, err := parseConfig(nil, env(nil)); err == nil {
		t.Fatal("missing remote URL accepted")
	}
	if _, _, err := parseConfig([]string{"--remote-url", "https://example.test", "--auth-token", "a", "--auth-token-url", "https://token.test", "--auth-client-id", "id", "--auth-client-secret", "secret"}, env(nil)); err == nil {
		t.Fatal("static and OAuth accepted")
	}
}
func TestToolFilters(t *testing.T) {
	cases := []struct {
		include, exclude, name string
		want                   bool
	}{{"*", "", "search", true}, {"github_*", "", "github_search", true}, {"github_*", "", "slack_search", false}, {"*", "*delete*", "github_delete_repo", false}, {"github_*", "github_delete_repo", "github_delete_repo", false}, {"github_*,slack_*", "", "slack_send", true}, {"github_*,slack_*", "", "jira_create", false}, {"*", "admin_*", "admin_reset", false}}
	for _, tc := range cases {
		f, err := newToolFilter(tc.include, tc.exclude)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.allowed(tc.name); got != tc.want {
			t.Errorf("%q/%q %q = %v", tc.include, tc.exclude, tc.name, got)
		}
	}
}
func TestOAuthRequestAndRefreshSchedule(t *testing.T) {
	var got url.Values
	m := &tokenManager{config: Config{AuthTokenURL: "https://token.example", AuthClientID: "id", AuthClientSecret: "secret", AuthScope: "mcp:read", AuthHeaderPrefix: "Bearer "}, client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		got, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"access_token":"token","expires_in":61}`)), Header: make(http.Header)}, nil
	})}}
	if err := m.refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got.Get("grant_type") != "client_credentials" || got.Get("scope") != "mcp:read" || m.authorization() != "Bearer token" {
		t.Fatalf("unexpected token request: %#v", got)
	}
	if until := time.Until(m.expires); until < 59*time.Second || until > 62*time.Second {
		t.Fatalf("expiry schedule = %s", until)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
