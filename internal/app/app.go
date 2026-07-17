// Package app contains the mcp-filter-proxy application implementation.
package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"runtime"
)

// Run executes the proxy and returns a process exit code.
func Run(args []string, getenv func(string) string, version string, stdout, stderr io.Writer) int {
	config, showVersion, err := parseConfig(args, getenv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if showVersion {
		fmt.Fprintf(stdout, "mcp-filter-proxy %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}
	proxy, err := newProxy(context.Background(), config, version)
	if err != nil {
		fmt.Fprintf(stderr, "startup failed: %v\n", err)
		return 1
	}
	defer proxy.Close()
	log.Printf("starting %s proxy to %s; tool include=%q exclude=%q", config.Transport, safeURL(config.RemoteURL), config.ToolInclude, config.ToolExclude)
	if err := proxy.Serve(); err != nil {
		fmt.Fprintf(stderr, "server failed: %v\n", err)
		return 1
	}
	return 0
}
