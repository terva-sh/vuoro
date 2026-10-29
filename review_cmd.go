package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/terva-sh/vuoro/app"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/server"
	"github.com/terva-sh/vuoro/web"
)

// Exit statuses of vuoro review (docs/plan.md section 6.7).
const (
	exitClean       = 0
	exitFindings    = 1
	exitUsage       = 2
	exitFailed      = 3
	exitInterrupted = 130
)

// reviewSchemaVersion is the version of the -json envelope's shape.
const reviewSchemaVersion = 1

// reviewCmd is vuoro review. Its fields are what a test replaces.
type reviewCmd struct {
	// newApp builds the app around the drafts MCP server's URL.
	newApp func(mcpURL string) (*app.App, *drafts.Server, error)
	// source is what a review of a forge change fetches.
	source func(a *app.App, ch *forge.Change) (review.Source, error)
	// interrupt delivers an interrupt; nil means the process's own signals.
	interrupt <-chan os.Signal
	poll      time.Duration
}

func runReview(args []string, stdout, stderr io.Writer) int {
	return reviewCmd{newApp: newApp, source: (*app.App).Source}.run(args, stdout, stderr)
}

// reviewResult is the -json envelope.
type reviewResult struct {
	SchemaVersion int             `json:"schemaVersion"`
	Kind          string          `json:"kind"`
	Run           reviewResultRun `json:"run"`
	Findings      []drafts.Draft  `json:"findings"`
	// Earlier are the findings of the run this one follows, with where each
	// stood. A finding that restates one names it in its earlier field.
	Earlier  []drafts.Earlier     `json:"earlier,omitempty"`
	Outcome  string               `json:"outcome"` // clean, findings, failed, timeout, or interrupted
	Error    string               `json:"error,omitempty"`
	Sessions []review.SessionView `json:"sessions"`
}

type reviewResultRun struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`
	Ref     forge.Ref `json:"ref"`
	Title   string    `json:"title"`
	WebURL  string    `json:"web_url,omitempty"`
	Head    string    `json:"head"`
	Base    string    `json:"base"`
	Status  string    `json:"status"`
	CostUSD float64   `json:"cost_usd"`
	// CostUnknown says a session's tool reported no cost for its tokens, or
	// a turn was stopped or failed before it reported usage, so CostUSD is
	// a floor.
	CostUnknown bool   `json:"cost_unknown,omitempty"`
	Follows     string `json:"follows,omitempty"`
}

func (c reviewCmd) run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vuoro review", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: vuoro review [-C dir] [-preset name | -profile name [-model m]] [-json] [-timeout d] [-follows run [-decided id=fixed:commit | -decided id=dismissed:reason]...] [range | change-url]")
		fmt.Fprintln(stderr, "\nReviews a range of local commits, or a change on a forge, and prints the findings. It never posts.")
		fmt.Fprintln(stderr, "A range is A..B or A...B (B against its merge base with A), or A (A...HEAD). No range means the upstream against HEAD.")
		fmt.Fprintln(stderr, "Exit status: 0 no findings, 1 findings, 2 usage, 3 a run or session failed or timed out, 130 interrupted.")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}
	dir := fs.String("C", ".", "the checkout a range is read from")
	preset := fs.String("preset", "", "the preset to run; vuoro ships quick, standard, and thorough; default quick when the config defines it, else the first preset")
	profile := fs.String("profile", "", "run one reviewer with this profile instead of a preset")
	model := fs.String("model", "", "the model for -profile")
	asJSON := fs.Bool("json", false, "print one JSON envelope instead of Markdown")
	timeout := fs.Duration("timeout", 30*time.Minute, "give up on the run after this long")
	follows := fs.String("follows", "", "an earlier run of the same change or checkout, by id, whose findings the reviewers are given")
	var decided decidedFlag
	fs.Var(&decided, "decided", "record a decision on a finding of the -follows run first, as id=fixed:commit or id=dismissed:reason; repeat it for each")
	// Flags may follow the range, as in vuoro review main..HEAD -json.
	var positional []string
	for {
		if err := fs.Parse(args); errors.Is(err, flag.ErrHelp) {
			return exitClean
		} else if err != nil {
			return exitUsage
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
	usage := func(msg string) int {
		fmt.Fprintf(stderr, "vuoro review: %s\n", msg)
		return exitUsage
	}
	switch {
	case len(positional) > 1:
		return usage("name one range or change, not several")
	case *preset != "" && *profile != "":
		return usage("-preset and -profile do not go together")
	case *model != "" && *profile == "":
		return usage("-model goes with -profile")
	case *timeout <= 0:
		return usage("-timeout must be positive")
	case len(decided) > 0 && *follows == "":
		return usage("-decided goes with -follows")
	}
	target := ""
	if len(positional) == 1 {
		target = positional[0]
	}
	logf := func(format string, a ...any) { fmt.Fprintf(stderr, "vuoro review: "+format+"\n", a...) }

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	interrupt := c.interrupt
	if interrupt == nil {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)
		interrupt = ch
	}

	// The drafts MCP server is the only route, on a loopback server of the
	// command's own, behind the same guard as the window's.
	ln, err := server.Listen("127.0.0.1:0")
	if err != nil {
		logf("%v", err)
		return exitFailed
	}
	a, mcp, err := c.newApp("http://" + ln.Addr().String() + "/mcp/")
	if err != nil {
		ln.Close()
		logf("%v", err)
		return exitFailed
	}
	if a.ConfigErr != nil {
		ln.Close()
		logf("the config file is broken: %v", a.ConfigErr)
		return exitFailed
	}
	srv := server.HTTPServer(server.New(server.Config{Addr: ln.Addr().String(), Token: server.NewToken(), Dist: web.Dist(), MCP: mcp}))
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logf("serve: %v", err)
		}
	}()
	defer srv.Close()
	defer a.Runs.Shutdown()

	var ch *forge.Change
	var src review.Source
	if strings.Contains(target, "://") {
		if ch, err = a.Open(ctx, target); err == nil {
			src, err = c.source(a, ch)
		}
	} else {
		var lt *review.LocalTarget
		if lt, err = review.LocalChange(ctx, "", *dir, target); err == nil {
			ch, src = lt.Change, lt.Source
			if lt.Dirty {
				logf("the checkout has uncommitted changes; only commits are reviewed")
			}
		}
	}
	if errors.Is(err, review.ErrNothingToReview) {
		logf("%v", err)
		return exitUsage
	}
	// Reading the change counts against -timeout too, and running out of it
	// there is the same timeout as running out of it during the review.
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		logf("the run did not finish within %s", *timeout)
		return exitFailed
	}
	if err != nil {
		logf("%v", err)
		return exitFailed
	}

	var reviewers []review.Reviewer
	if *profile != "" {
		reviewers = []review.Reviewer{{Profile: *profile, Model: *model}}
	} else if *preset == "" && a.Config != nil {
		// The person's own presets only, never the merged list that Plan
		// reads. vuoro ships a preset named quick (D61), so matching against
		// the merged list would start preferring it for someone whose config
		// names other presets, which is a change to what their command line
		// does today. With no config at all, Plan's first preset is quick
		// anyway, which is what the window picks too.
		for _, p := range a.Config.Presets {
			if p.Name == "quick" {
				*preset = "quick"
			}
		}
	}
	reviewers, synth, verifier, err := a.Plan(*preset, reviewers, nil, nil)
	if err != nil {
		return usage(err.Error())
	}
	if *follows != "" {
		// The earlier run was written by another process, so this one reads
		// it, and only it, first. Refusing it here, before any session
		// starts, keeps a wrong id a usage error rather than a failed run.
		if err := a.Runs.LoadRun(*follows); err != nil {
			logf("%v", err)
			return exitFailed
		}
		// Let go of the run on every way out, a refusal included, so the
		// deferred Shutdown, which runs after this, writes no stale copy.
		// Follow's own write adds the forward link as the run starts.
		defer a.Runs.Forget(*follows)
		switch prev := a.Runs.Get(*follows); {
		case prev == nil:
			return usage(fmt.Sprintf("no finished run %s to follow", *follows))
		case prev.Mode != "":
			return usage(fmt.Sprintf("run %s is not a review", *follows))
		case prev.Kind != ch.Kind || prev.Ref != ch.Ref:
			return usage(fmt.Sprintf("run %s reviewed another change or checkout", *follows))
		}
		// Recorded in the earlier run, as the window's Drop and fix link
		// are, so the reviewers are told and a later round still knows.
		if err := a.Runs.RecordDecided(ctx, *follows, &src, decided); err != nil {
			return usage(err.Error())
		}
	}
	a.ProbeTools()
	run, err := a.Runs.Follow(ch, src, reviewers, synth, verifier, *follows)
	if err != nil {
		logf("%v", err)
		return exitFailed
	}
	logf("run %s: %s at %s, %d reviewer%s", run.ID, ch.Title, shortSHA(ch.Diff.Head), len(reviewers), plural(len(reviewers)))

	outcome := c.wait(ctx, run, interrupt, logf)
	v := run.View()
	closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Minute)
	if err := a.Runs.Close(closeCtx, run.ID); err != nil {
		logf("closing the run: %v", err)
	}
	closeCancel()

	shown := review.Shown(v)
	status := exitClean
	switch {
	case outcome != "":
	case v.Status == review.Failed || anyFailed(v):
		outcome, status = "failed", exitFailed
	case len(shown) > 0:
		outcome, status = "findings", exitFindings
	default:
		outcome = "clean"
	}
	switch outcome {
	case "interrupted":
		status = exitInterrupted
	case "timeout":
		status = exitFailed
	}

	if *asJSON {
		res := reviewResult{
			SchemaVersion: reviewSchemaVersion, Kind: "review-result", Outcome: outcome, Error: v.Error,
			Run: reviewResultRun{ID: v.ID, Kind: v.Kind, Ref: v.Ref, Title: v.Title, WebURL: v.WebURL,
				Head: v.Head, Base: v.Base, Status: string(v.Status), CostUSD: v.CostUSD, CostUnknown: v.CostUnknown, Follows: v.Follows},
			Findings: shown, Earlier: v.Earlier, Sessions: v.Sessions,
		}
		if res.Findings == nil {
			res.Findings = []drafts.Draft{}
		}
		for i := range res.Sessions {
			res.Sessions[i].Log = nil // the transcript stays in the run record
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			logf("%v", err)
			return exitFailed
		}
	} else {
		io.WriteString(stdout, review.Findings(v))
	}
	if outcome == "timeout" {
		logf("the run did not finish within %s", *timeout)
	}
	return status
}

// wait polls the run until it settles, and reports each session's moves on
// stderr. It returns "timeout" or "interrupted" when it stops early, and
// otherwise "".
func (c reviewCmd) wait(ctx context.Context, run *review.Run, interrupt <-chan os.Signal, logf func(string, ...any)) string {
	poll := c.poll
	if poll == 0 {
		poll = 500 * time.Millisecond
	}
	tick := time.NewTicker(poll)
	defer tick.Stop()
	seen := map[string]review.Status{}
	for {
		v := run.View()
		for _, s := range v.Sessions {
			if seen[s.ID] != s.Status {
				seen[s.ID] = s.Status
				line := fmt.Sprintf("%s %s %s (%s", s.ID, s.Role, s.Profile, s.Tool)
				if s.Model != "" {
					line += " " + s.Model
				}
				line += "): " + string(s.Status)
				if s.Error != "" {
					line += ": " + s.Error
				}
				logf("%s", line)
			}
		}
		if run.Settled() {
			return ""
		}
		select {
		case <-ctx.Done():
			return "timeout"
		case <-interrupt:
			return "interrupted"
		case <-tick.C:
		}
	}
}

func anyFailed(v review.RunView) bool {
	for _, s := range v.Sessions {
		if s.Status == review.Failed {
			return true
		}
	}
	return false
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// decidedFlag collects -decided: id=fixed:commit or id=dismissed:reason.
type decidedFlag []review.Decided

func (f *decidedFlag) String() string { return "" }

func (f *decidedFlag) Set(v string) error {
	id, rest, ok := strings.Cut(v, "=")
	standing, arg, _ := strings.Cut(rest, ":")
	arg = strings.TrimSpace(arg)
	d := review.Decided{ID: strings.TrimSpace(id), Standing: standing}
	switch {
	case !ok || d.ID == "":
		return errors.New("want id=fixed:commit or id=dismissed:reason")
	case standing == drafts.StandingFixed && arg != "":
		d.Commit = arg
	case standing == drafts.StandingDismissed && arg != "":
		d.Reason = arg
	case standing == drafts.StandingFixed:
		return fmt.Errorf("%s: a fix names its commit, as %s=fixed:commit", d.ID, d.ID)
	case standing == drafts.StandingDismissed:
		return fmt.Errorf("%s: a dismissal gives its reason, as %s=dismissed:reason", d.ID, d.ID)
	default:
		return fmt.Errorf("%s: a finding is decided fixed or dismissed, not %q", d.ID, standing)
	}
	*f = append(*f, d)
	return nil
}
