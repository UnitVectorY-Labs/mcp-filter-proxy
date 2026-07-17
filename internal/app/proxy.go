package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type proxy struct {
	config  Config
	version string
	filter  toolFilter
	remote  *client.Client
	server  *server.MCPServer
	auth    *tokenManager
	syncMu  sync.Mutex
}

func newProxy(ctx context.Context, config Config, version string) (*proxy, error) {
	filter, err := newToolFilter(config.ToolInclude, config.ToolExclude)
	if err != nil {
		return nil, err
	}
	p := &proxy{config: config, version: version, filter: filter}
	p.auth = &tokenManager{config: config, client: &http.Client{}}
	if err := p.auth.start(ctx); err != nil {
		return nil, err
	}
	remoteHTTP := &http.Client{Transport: headerTransport{base: http.DefaultTransport, headers: config.Headers, auth: p}}
	t, err := transport.NewStreamableHTTP(config.RemoteURL, transport.WithHTTPBasicClient(remoteHTTP), transport.WithContinuousListening())
	if err != nil {
		return nil, err
	}
	p.remote = client.NewClient(t)
	if err := p.remote.Start(ctx); err != nil {
		return nil, fmt.Errorf("remote connection: %w", err)
	}
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "mcp-filter-proxy", Version: version}
	init.Params.Capabilities = mcp.ClientCapabilities{}
	if _, err := p.remote.Initialize(ctx, init); err != nil {
		return nil, fmt.Errorf("remote initialization: %w", err)
	}
	hooks := &server.Hooks{}
	p.server = server.NewMCPServer("mcp-filter-proxy", version, server.WithHooks(hooks), server.WithToolFilter(func(_ context.Context, tools []mcp.Tool) []mcp.Tool {
		out := make([]mcp.Tool, 0, len(tools))
		for _, tool := range tools {
			if p.filter.allowed(tool.Name) {
				out = append(out, tool)
			}
		}
		return out
	}))
	hooks.AddBeforeListTools(func(ctx context.Context, _ any, _ *mcp.ListToolsRequest) { _ = p.refreshTools(ctx) })
	hooks.AddBeforeListResources(func(ctx context.Context, _ any, _ *mcp.ListResourcesRequest) { p.refreshResources(ctx) })
	hooks.AddBeforeListResourceTemplates(func(ctx context.Context, _ any, _ *mcp.ListResourceTemplatesRequest) { p.refreshResourceTemplates(ctx) })
	hooks.AddBeforeListPrompts(func(ctx context.Context, _ any, _ *mcp.ListPromptsRequest) { p.refreshPrompts(ctx) })
	p.remote.OnNotification(func(notification mcp.JSONRPCNotification) {
		refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		switch notification.Method {
		case mcp.MethodNotificationToolsListChanged:
			_ = p.refreshTools(refreshCtx)
		case mcp.MethodNotificationResourcesListChanged:
			p.refreshResources(refreshCtx)
			p.refreshResourceTemplates(refreshCtx)
		case mcp.MethodNotificationPromptsListChanged:
			p.refreshPrompts(refreshCtx)
		}
	})
	if err := p.refreshTools(ctx); err != nil {
		_ = p.remote.Close()
		return nil, err
	}
	p.refreshResources(ctx)
	p.refreshResourceTemplates(ctx)
	p.refreshPrompts(ctx)
	return p, nil
}

func (p *proxy) refreshTools(ctx context.Context) error {
	p.syncMu.Lock()
	defer p.syncMu.Unlock()
	tools, err := p.remote.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("list remote tools: %w", err)
	}
	local := make([]server.ServerTool, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		tool := tool
		local = append(local, server.ServerTool{Tool: tool, Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !p.filter.allowed(req.Params.Name) {
				return mcp.NewToolResultError("tool not found"), nil
			}
			return p.remote.CallTool(ctx, req)
		}})
	}
	p.server.SetTools(local...)
	return nil
}
func (p *proxy) refreshResources(ctx context.Context) {
	resources, err := p.remote.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		return
	}
	p.syncMu.Lock()
	defer p.syncMu.Unlock()
	local := make([]server.ServerResource, 0, len(resources.Resources))
	for _, resource := range resources.Resources {
		resource := resource
		local = append(local, server.ServerResource{Resource: resource, Handler: func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := p.remote.ReadResource(ctx, req)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		}})
	}
	p.server.SetResources(local...)
}
func (p *proxy) refreshResourceTemplates(ctx context.Context) {
	templates, err := p.remote.ListResourceTemplates(ctx, mcp.ListResourceTemplatesRequest{})
	if err != nil {
		return
	}
	p.syncMu.Lock()
	defer p.syncMu.Unlock()
	local := make([]server.ServerResourceTemplate, 0, len(templates.ResourceTemplates))
	for _, template := range templates.ResourceTemplates {
		template := template
		local = append(local, server.ServerResourceTemplate{Template: template, Handler: func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := p.remote.ReadResource(ctx, req)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		}})
	}
	p.server.SetResourceTemplates(local...)
}
func (p *proxy) refreshPrompts(ctx context.Context) {
	prompts, err := p.remote.ListPrompts(ctx, mcp.ListPromptsRequest{})
	if err != nil {
		return
	}
	p.syncMu.Lock()
	defer p.syncMu.Unlock()
	local := make([]server.ServerPrompt, 0, len(prompts.Prompts))
	for _, prompt := range prompts.Prompts {
		prompt := prompt
		local = append(local, server.ServerPrompt{Prompt: prompt, Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return p.remote.GetPrompt(ctx, req)
		}})
	}
	p.server.SetPrompts(local...)
}
func (p *proxy) Serve() error {
	if p.config.Transport == "http" {
		return server.NewStreamableHTTPServer(p.server).Start(p.config.ListenAddr)
	}
	return server.ServeStdio(p.server)
}
func (p *proxy) Close() error {
	if p.remote != nil {
		return p.remote.Close()
	}
	return nil
}
func (p *proxy) authHeader() (string, string) { return p.config.AuthHeaderName, p.auth.authorization() }

type headerTransport struct {
	base    http.RoundTripper
	headers http.Header
	auth    *proxy
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, values := range t.headers {
		req.Header.Del(k)
		for _, value := range values {
			req.Header.Add(k, value)
		}
	}
	name, value := t.auth.authHeader()
	if value != "" {
		req.Header.Set(name, value)
	}
	return t.base.RoundTrip(req)
}
func logSafe(message string) { log.Print(message) }
