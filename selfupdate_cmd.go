package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/terva-sh/vuoro/update"
)

// selfUpdateCmd is vuoro self-update (D74). Its fields are what a test
// replaces.
type selfUpdateCmd struct {
	client     update.Client
	version    string
	executable func() (string, error)
}

func runSelfUpdate(args []string, stdout, stderr io.Writer) int {
	return selfUpdateCmd{
		client:     update.Client{HTTP: &http.Client{Timeout: 2 * time.Minute}},
		version:    version,
		executable: update.Executable,
	}.run(args, stdout, stderr)
}

// selfUpdateResult is the -json envelope.
type selfUpdateResult struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	Current       string `json:"current"`
	Latest        string `json:"latest"`
	Newer         bool   `json:"newer"`
	Applied       bool   `json:"applied"`
	Target        string `json:"target,omitempty"`
}

func (c selfUpdateCmd) run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vuoro self-update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: vuoro self-update [-check] [-json]")
		fmt.Fprintln(stderr, "\nReplaces this vuoro with the latest release on GitHub, after checking the archive against the release's checksums.txt.")
		fmt.Fprintln(stderr, "Exit status: 0 up to date or updated, 1 a newer release exists (with -check), 2 usage, 3 the update failed.")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}
	check := fs.Bool("check", false, "say whether a newer release exists, and change nothing")
	asJSON := fs.Bool("json", false, "print one JSON envelope instead of a sentence")
	if err := fs.Parse(args); errors.Is(err, flag.ErrHelp) {
		return exitClean
	} else if err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "vuoro self-update: it takes no arguments")
		return exitUsage
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "vuoro self-update: %v\n", err)
		return exitFailed
	}

	ctx := context.Background()
	rel, err := c.client.Latest(ctx, c.version)
	if err != nil {
		return fail(err)
	}
	res := selfUpdateResult{SchemaVersion: 1, Kind: "self-update", Current: c.version, Latest: rel.Tag}
	if res.Newer, err = update.Newer(c.version, rel.Tag); err != nil {
		return fail(err)
	}
	if res.Newer && !*check {
		if res.Target, err = c.executable(); err != nil {
			return fail(err)
		}
		if err := c.client.Apply(ctx, rel, c.version, res.Target); err != nil {
			return fail(err)
		}
		res.Applied = true
	}

	switch {
	case *asJSON:
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(res)
	case res.Applied:
		fmt.Fprintf(stdout, "vuoro %s replaced %s at %s. Quit and reopen vuoro to use it.\n", rel.Tag, c.version, res.Target)
	case res.Newer:
		fmt.Fprintf(stdout, "vuoro %s is available; this is %s. Run vuoro self-update to move to it.\n", rel.Tag, c.version)
	default:
		fmt.Fprintf(stdout, "vuoro %s is up to date (the latest release is %s).\n", c.version, rel.Tag)
	}
	if res.Newer && *check {
		return exitFindings
	}
	return exitClean
}
