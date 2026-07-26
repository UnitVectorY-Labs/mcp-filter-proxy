package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type proxy struct {
	config        Config
	version       string
	filter        toolFilter
	remote        *mcp.ClientSession
	server        *mcp.Server
	auth          *tokenManager
	syncMu        sync.Mutex
	httpServer    *http.Server
	toolNames     []string
	resourceURIs  []string
	templateURIs  []string
	promptNames   []string
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
	transport := &mcp.StreamableClientTransport{
		Endpoint:   config.RemoteURL,
		HTTPClient: remoteHTTP,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-filter-proxy", Version: version}, &mcp.ClientOptions{
		ToolListChangedHandler: func(ctx context.Context, _ *mcp.ToolListChangedRequest) {
			go func() {
				refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = p.refreshTools(refreshCtx)
			}()
		},
		ResourceListChangedHandler: func(ctx context.Context, _ *mcp.ResourceListChangedRequest) {
			go func() {
				refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				p.refreshResources(refreshCtx)
				p.refreshResourceTemplates(refreshCtx)
			}()
		},
		PromptListChangedHandler: func(ctx context.Context, _ *mcp.PromptListChangedRequest) {
			go func() {
				refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				p.refreshPrompts(refreshCtx)
			}()
		},
	})
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("remote connection: %w", err)
	}
	p.remote = session
	p.server = mcp.NewServer(&mcp.Implementation{Name: "mcp-filter-proxy", Version: version}, nil)
	if err := p.refreshTools(ctx); err != nil {
		p.remote.Close()
		return nil, err
	}
	p.refreshResources(ctx)
	p.refreshResourceTemplates(ctx)
	p.refreshPrompts(ctx)
	return p, nil
}

func (p *proxy) refreshTools(ctx context.Context) error {
	toolsResult, err := p.remote.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("list remote tools: %w", err)
	}
	p.syncMu.Lock()
	defer p.syncMu.Unlock()
	current := make(map[string]bool, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		if !p.filter.allowed(tool.Name) {
			continue
		}
		current[tool.Name] = true
	}
	for _, name := range p.toolNames {
		if !current[name] {
			p.server.RemoveTools(name)
		}
	}
	p.toolNames = nil
	for _, tool := range toolsResult.Tools {
		if !current[tool.Name] {
			continue
		}
		p.toolNames = append(p.toolNames, tool.Name)
		tool := tool
		p.server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !p.filter.allowed(req.Params.Name) {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "tool not found"}},
					IsError: true,
				}, nil
			}
			var args any
			if len(req.Params.Arguments) > 0 {
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					return &mcp.CallToolResult{
						Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
						IsError: true,
					}, nil
				}
			}
			result, err := p.remote.CallTool(ctx, &mcp.CallToolParams{
				Name:      req.Params.Name,
				Arguments: args,
			})
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
					IsError: true,
				}, nil
			}
			return result, nil
		})
	}
	return nil
}

func (p *proxy) refreshResources(ctx context.Context) {
	resourcesResult, err := p.remote.ListResources(ctx, nil)
	if err != nil {
		return
	}
	p.syncMu.Lock()
	defer p.syncMu.Unlock()
	current := make(map[string]bool, len(resourcesResult.Resources))
	for _, resource := range resourcesResult.Resources {
		current[resource.URI] = true
	}
	for _, uri := range p.resourceURIs {
		if !current[uri] {
			p.server.RemoveResources(uri)
		}
	}
	p.resourceURIs = nil
	for _, resource := range resourcesResult.Resources {
		p.resourceURIs = append(p.resourceURIs, resource.URI)
		resource := resource
		p.server.AddResource(resource, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return p.remote.ReadResource(ctx, &mcp.ReadResourceParams{URI: req.Params.URI})
		})
	}
}

func (p *proxy) refreshResourceTemplates(ctx context.Context) {
	templatesResult, err := p.remote.ListResourceTemplates(ctx, nil)
	if err != nil {
		return
	}
	p.syncMu.Lock()
	defer p.syncMu.Unlock()
	current := make(map[string]bool, len(templatesResult.ResourceTemplates))
	for _, template := range templatesResult.ResourceTemplates {
		current[template.URITemplate] = true
	}
	for _, uri := range p.templateURIs {
		if !current[uri] {
			p.server.RemoveResourceTemplates(uri)
		}
	}
	p.templateURIs = nil
	for _, template := range templatesResult.ResourceTemplates {
		p.templateURIs = append(p.templateURIs, template.URITemplate)
		template := template
		p.server.AddResourceTemplate(template, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return p.remote.ReadResource(ctx, &mcp.ReadResourceParams{URI: req.Params.URI})
		})
	}
}

func (p *proxy) refreshPrompts(ctx context.Context) {
	promptsResult, err := p.remote.ListPrompts(ctx, nil)
	if err != nil {
		return
	}
	p.syncMu.Lock()
	defer p.syncMu.Unlock()
	current := make(map[string]bool, len(promptsResult.Prompts))
	for _, prompt := range promptsResult.Prompts {
		current[prompt.Name] = true
	}
	for _, name := range p.promptNames {
		if !current[name] {
			p.server.RemovePrompts(name)
		}
	}
	p.promptNames = nil
	for _, prompt := range promptsResult.Prompts {
		p.promptNames = append(p.promptNames, prompt.Name)
		prompt := prompt
		p.server.AddPrompt(prompt, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return p.remote.GetPrompt(ctx, &mcp.GetPromptParams{
				Name:      req.Params.Name,
				Arguments: req.Params.Arguments,
			})
		})
	}
}

func (p *proxy) Serve() error {
	if p.config.Transport == "http" {
		handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return p.server
		}, nil)
		p.httpServer = &http.Server{Addr: p.config.ListenAddr, Handler: handler}
		return p.httpServer.ListenAndServe()
	}
	return p.server.Run(context.Background(), &mcp.StdioTransport{})
}

func (p *proxy) Close() error {
	if p.httpServer != nil {
		p.httpServer.Close()
	}
	if p.remote != nil {
		p.remote.Close()
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
