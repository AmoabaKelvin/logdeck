package main

import (
	"os"

	"github.com/AmoabaKelvin/logdeck/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
