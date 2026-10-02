// Command vuoro is one desktop window for every code review you give and get.
//
// This is the scaffold. The walking skeleton replaces it with the loopback
// server shown in a tuohi window (docs/plan.md section 3).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vuoro", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "vuoro %s\n", version)
		return 0
	}
	fmt.Fprintln(stderr, "vuoro: there is no window yet; see docs/plan.md section 10")
	return 1
}
