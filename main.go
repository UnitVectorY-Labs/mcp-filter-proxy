package main

import (
	"os"
	"runtime/debug"

	"github.com/UnitVectorY-Labs/mcp-filter-proxy/internal/app"
)

var Version = "dev"

func main() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
	os.Exit(app.Run(os.Args[1:], os.Getenv, Version, os.Stdout, os.Stderr))
}
