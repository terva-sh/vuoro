// Package local reads the person's own checkouts for the window's Local tab
// (docs/plan.md section 6.8): the projects they added or a scan found, each
// project's branches beyond its base, and the worktrees those branches are
// checked out in. It only reads. git runs with the checkout's hooks,
// fsmonitor, external diff programs, and textconv disabled, because a
// checkout's configuration is not the person's choice to run code.
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// gitTimeout bounds one git command.
const gitTimeout = 20 * time.Second

// maxOutput bounds what one git command may print.
const maxOutput = 1 << 20

var errTooLarge = errors.New("local: git printed more than vuoro reads")

// Git runs read-only git commands in one directory.
type Git struct {
	Bin string // empty means git on the PATH
	Dir string
}

// Run runs git with args in g.Dir and returns what it printed.
func (g Git) Run(ctx context.Context, args ...string) (string, error) {
	bin := g.Bin
	if bin == "" {
		bin = "git"
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	full := []string{"-C", g.Dir, "--literal-pathspecs",
		"-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false", "-c", "core.pager=cat",
		"-c", "diff.external=", "-c", "core.quotePath=false"}
	cmd := exec.CommandContext(ctx, bin, append(full, args...)...)
	// No index refresh is written, and git never prompts.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &capped{buf: &out, cancel: cancel}, &errb
	err := cmd.Run()
	if out.Len() > maxOutput {
		return "", errTooLarge
	}
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = msg[i+1:]
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("local: git %s: %s", args[0], msg)
	}
	return out.String(), nil
}

// lines splits output into its non-empty lines.
func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimRight(l, "\r"); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// capped stops the command once it has printed more than maxOutput.
type capped struct {
	buf    *bytes.Buffer
	cancel context.CancelFunc
}

func (c *capped) Write(p []byte) (int, error) {
	if c.buf.Len() > maxOutput {
		c.cancel()
		return len(p), nil
	}
	return c.buf.Write(p)
}
