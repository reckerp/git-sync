package main

import (
	"os"

	"github.com/reckerp/git-sync/internal/cli"
)

// version is set by goreleaser at build time.
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
