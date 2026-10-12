// Command wctl runs declared Watts workflows.
//
// Workflows and runtime choices come from manifests. The CLI adapts terminal commands
// to application services; project version control is not required or modified.
package main

import (
	"os"

	"github.com/jasondeutsch/watts/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
