package main

import (
	"os"

	"github.com/lanternfold/prd-pr/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args, os.Stdout, os.Stderr, cli.DefaultRuntime()))
}
