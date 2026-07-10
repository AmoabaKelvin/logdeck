package main

import (
	"os"

	"github.com/AmoabaKelvin/logdeck/internal/cli"
)

// version is injected at build time via -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
