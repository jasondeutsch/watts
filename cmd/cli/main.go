// Command watts governs AI coding work on Pi 1.0 with the pi-ralph-loop extension.
//
// Install it once, then run `watts init` in each repository. Everything Watts needs is written to
// that project's .watts directory. The CLI adapts terminal commands to internal application
// services; project version control is not required or modified.
package main

import (
	"os"

	"github.com/jasondeutsch/watts/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
