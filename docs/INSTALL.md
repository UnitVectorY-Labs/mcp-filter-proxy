---
layout: default
title: Install
nav_order: 2
permalink: /install
---

# Installation

## Binary Download

Download the latest release from the [releases page](https://github.com/UnitVectorY-Labs/mcp-filter-proxy/releases).

### Linux (amd64)

```bash
curl -L https://github.com/UnitVectorY-Labs/mcp-filter-proxy/releases/latest/download/mcp-filter-proxy_linux_amd64 -o mcp-filter-proxy
chmod +x mcp-filter-proxy
sudo mv mcp-filter-proxy /usr/local/bin/
```

### macOS (amd64)

```bash
curl -L https://github.com/UnitVectorY-Labs/mcp-filter-proxy/releases/latest/download/mcp-filter-proxy_darwin_amd64 -o mcp-filter-proxy
chmod +x mcp-filter-proxy
sudo mv mcp-filter-proxy /usr/local/bin/
```

### macOS (arm64 / Apple Silicon)

```bash
curl -L https://github.com/UnitVectorY-Labs/mcp-filter-proxy/releases/latest/download/mcp-filter-proxy_darwin_arm64 -o mcp-filter-proxy
chmod +x mcp-filter-proxy
sudo mv mcp-filter-proxy /usr/local/bin/
```

### Windows

Download `mcp-filter-proxy_windows_amd64.exe` from the releases page and add it to your PATH.

## Building from Source

### Build

```bash
git clone https://github.com/UnitVectorY-Labs/mcp-filter-proxy.git
cd mcp-filter-proxy
go build -o mcp-filter-proxy .
```

### Install to GOPATH

```bash
go install github.com/UnitVectorY-Labs/mcp-filter-proxy@latest
```

## Verify Installation

```bash
mcp-filter-proxy --help
```