package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// AcronymEntry represents one row in the CSV
type AcronymEntry struct {
	Full        string `json:"full"`
	Description string `json:"description"`
}

var nonAlpha = regexp.MustCompile("[^A-Za-z]+")

var Version = "dev" // This will be set by the build systems to the release version

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func buildVersionOutput(version string) string {
	normalized := version
	if semverRe.MatchString(normalized) && !strings.HasPrefix(normalized, "v") {
		normalized = "v" + normalized
	}
	return fmt.Sprintf("%s (%s, %s/%s)", normalized, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// sanitizeKey removes non-alphabetic characters and lowercases the string
func sanitizeKey(s string) string {
	s = nonAlpha.ReplaceAllString(s, "")
	return strings.ToLower(s)
}

// loadCSV reads the CSV at path and returns a mapping from sanitized acronym to its entries
func loadCSV(path string) (map[string][]AcronymEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	entries := make(map[string][]AcronymEntry)
	for idx, rec := range recs {
		if len(rec) < 3 {
			continue // skip malformed lines
		}
		// Skip header row if present
		if idx == 0 && strings.EqualFold(rec[0], "acronym") {
			continue
		}

		key := sanitizeKey(rec[0])
		entry := AcronymEntry{
			Full:        strings.TrimSpace(rec[1]),
			Description: strings.TrimSpace(rec[2]),
		}
		entries[key] = append(entries[key], entry)
	}
	return entries, nil
}

func main() {
	// Set the build version from the build info if not set by the build system
	if Version == "dev" || Version == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				Version = bi.Main.Version
			}
		}
	}

	// CLI flag for Streamable HTTP transport
	var httpAddr string
	flag.StringVar(&httpAddr, "http", "", "run in Streamable HTTP transport on the given address, e.g. :8080")
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("mcp-acronym-lookup version %s\n", buildVersionOutput(Version))
		os.Exit(0)
	}

	// Path to CSV file from environment
	csvPath := os.Getenv("ACRONYM_FILE")
	if csvPath == "" {
		fmt.Fprintln(os.Stderr, "Error: ACRONYM_FILE environment variable is required")
		os.Exit(1)
	}

	// Load acronym entries
	entries, err := loadCSV(csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading CSV data: %v\n", err)
		os.Exit(1)
	}

	// Initialize MCP server with fixed name and version
	srv := server.NewMCPServer("mcp-acronym-lookup", Version)

	// Register lookup tool
	tool := mcp.NewTool(
		"lookupAcronym",
		mcp.WithDescription("Resolve an acronym or initialism to its full form(s) and description(s)."),
		mcp.WithTitleAnnotation("Lookup Acronym"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("acronym", mcp.Description("The acronym or initialism to resolve."), mcp.Required()),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		acronym, ok := args["acronym"].(string)
		if !ok {
			return mcp.NewToolResultError("invalid or missing 'acronym' parameter"), nil
		}
		key := sanitizeKey(acronym)
		matches, found := entries[key]
		if !found || len(matches) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("no entry found for '%s'", acronym)), nil
		}
		// Prepare response
		resp := map[string]any{
			"acronym":     key,
			"definitions": matches,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to encode response", err), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Choose transport mode
	if httpAddr != "" {
		fmt.Printf("Starting MCP server using Streamable HTTP transport on %s\n", httpAddr)

		// Create HTTP server
		httpServer := server.NewStreamableHTTPServer(srv)

		fmt.Printf("Streamable HTTP Endpoint: http://localhost:%s/mcp\n", httpAddr)

		// Start the server
		if err := httpServer.Start(":" + httpAddr); err != nil {
			log.Fatalf("Streamable HTTP server failed to start: %v", err)
		}
	} else {
		// stdio mode by default
		if err := server.ServeStdio(srv); err != nil {
			log.Fatalf("Fatal: MCP server terminated: %v\n", err)
		}
	}
}
