// atlas-validate checks game-data against a local Profile.
package main

import (
	"github.com/mardwerk/td-profile/internal/atlasvalidate"
	"io"
	"os"
)

func run(args []string, stdout, stderr io.Writer) int {
	return atlasvalidate.RunCLI(args, stdout, stderr)
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
