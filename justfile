
# Commands for mcp-filter-proxy
default:
  @just --list
# Build mcp-filter-proxy with Go
build:
  go build ./...

# Run tests for mcp-filter-proxy with Go
test:
  go clean -testcache
  go test ./...