package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

// AskFacts is what an ask session knows of the change beyond its git history:
// the forge read the conversation started from, or the one vuoro made for the
// person's latest follow-up (section 6.11). The context tools answer from it
// and never read the forge themselves.
type AskFacts struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Author      string         `json:"author"`
	State       string         `json:"state"`
	Draft       bool           `json:"draft,omitempty"`
	Source      string         `json:"source_branch"`
	Target      string         `json:"target_branch"`
	Labels      []string       `json:"labels,omitempty"`
	Pipeline    string         `json:"pipeline,omitempty"`
	Threads     []forge.Thread `json:"threads,omitempty"`
	ReadAt      time.Time      `json:"read_at"`
}

// FactsOf is what ch says of itself, read at.
func FactsOf(ch *forge.Change, at time.Time) AskFacts {
	f := AskFacts{Title: ch.Title, Description: ch.Description, Author: ch.Author.Username, State: string(ch.State), Draft: ch.Draft,
		Source: ch.SourceBranch, Target: ch.TargetBranch, Labels: append([]string(nil), ch.Labels...), Pipeline: ch.Pipeline, ReadAt: at.UTC()}
	for _, t := range ch.Threads {
		f.Threads = append(f.Threads, cleanThread(t))
	}
	return f
}

// maxContext caps what one context tool answers. Past it the call is
// refused with the reason, never cut short.
const maxContext = 128 << 10

// askContext is an ask session's read-only tools: git history in vuoro's
// clone, and the change as vuoro last read it. Nothing here writes to the
// clone, a worktree, or the forge.
type askContext struct {
	repos      *Repos
	key        string
	base, head string
	facts      func() AskFacts
}

var contextTools = []map[string]any{
	{
		"name":        "git_log",
		"description": "List commits reachable from a revision, newest first: sha, author, date, subject. Narrow it to one path.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
			"rev":       map[string]any{"type": "string", "description": "head (the default), base, or a commit ID in the change's history, optionally with ~N"},
			"path":      map[string]any{"type": "string", "description": "a repository-relative path"},
			"max_count": map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "description": "at most this many commits; 30 by default"},
		}},
		"annotations": map[string]any{"readOnlyHint": true},
	},
	{
		"name":        "git_show",
		"description": "Show one commit: its message, the files it changed, and its patch, or only its patch to one path.",
		"inputSchema": map[string]any{"type": "object", "required": []string{"rev"}, "properties": map[string]any{
			"rev":  map[string]any{"type": "string"},
			"path": map[string]any{"type": "string"},
		}},
		"annotations": map[string]any{"readOnlyHint": true},
	},
	{
		"name":        "git_blame",
		"description": "Say which commit last changed each of a range of lines in a file, at a revision.",
		"inputSchema": map[string]any{"type": "object", "required": []string{"path", "start_line", "end_line"}, "properties": map[string]any{
			"path":       map[string]any{"type": "string"},
			"start_line": map[string]any{"type": "integer", "minimum": 1},
			"end_line":   map[string]any{"type": "integer", "minimum": 1},
			"rev":        map[string]any{"type": "string", "description": "head by default"},
		}},
		"annotations": map[string]any{"readOnlyHint": true},
	},
	{
		"name":        "git_diff",
		"description": "The diff between two revisions, or of one path between them.",
		"inputSchema": map[string]any{"type": "object", "required": []string{"from"}, "properties": map[string]any{
			"from": map[string]any{"type": "string"},
			"to":   map[string]any{"type": "string", "description": "head by default"},
			"path": map[string]any{"type": "string"},
		}},
		"annotations": map[string]any{"readOnlyHint": true},
	},
	{
		"name":        "change",
		"description": "The change's title, description, author, state, branches, labels, and pipeline status, as vuoro last read them from the forge.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		"annotations": map[string]any{"readOnlyHint": true},
	},
	{
		"name":        "discussion",
		"description": "The change's discussion threads and their comments, with the lines each is on, as vuoro last read them from the forge.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
			"unresolved_only": map[string]any{"type": "boolean"},
		}},
		"annotations": map[string]any{"readOnlyHint": true},
	},
	{
		"name":        "pipeline",
		"description": "The head pipeline's status as vuoro last read it. vuoro knows the status only, not the jobs or their logs.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		"annotations": map[string]any{"readOnlyHint": true},
	},
}

// ContextToolNames are the context tools, for a driver's allowlist.
func ContextToolNames() []string {
	var out []string
	for _, t := range contextTools {
		out = append(out, t["name"].(string))
	}
	return out
}

func (c *askContext) List() []map[string]any { return contextTools }

// Call answers one tool. A refusal names its rule, so the agent can correct
// the call, and never runs git.
func (c *askContext) Call(name string, raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	out, err := c.call(name, raw)
	if err != nil {
		return err.Error(), true
	}
	return out, false
}

func (c *askContext) call(name string, raw json.RawMessage) (string, error) {
	switch name {
	case "change":
		if err := strict(raw, &struct{}{}); err != nil {
			return "", err
		}
		f := c.facts()
		f.Threads = nil
		return marshal(f)
	case "discussion":
		var p struct {
			UnresolvedOnly bool `json:"unresolved_only"`
		}
		if err := strict(raw, &p); err != nil {
			return "", err
		}
		return marshal(discussion(c.facts(), p.UnresolvedOnly))
	case "pipeline":
		if err := strict(raw, &struct{}{}); err != nil {
			return "", err
		}
		f := c.facts()
		status := f.Pipeline
		if status == "" {
			status = "unknown"
		}
		return marshal(map[string]any{"status": status, "read_at": f.ReadAt, "note": "vuoro knows the head pipeline's status only, not its jobs or their logs"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	gitDir := filepath.Join(c.repos.Dir, filepath.FromSlash(c.key)+".git")
	if !keyRE.MatchString(c.key) || strings.Contains(c.key, "..") {
		return "", errors.New("the repository is unavailable")
	}
	unlock, err := c.repos.lock(ctx, gitDir)
	if err != nil {
		return "", errors.New("the repository is busy; try again")
	}
	defer unlock()
	switch name {
	case "git_log":
		var p struct {
			Rev      string `json:"rev"`
			Path     string `json:"path"`
			MaxCount int    `json:"max_count"`
		}
		if err := strict(raw, &p); err != nil {
			return "", err
		}
		if p.MaxCount == 0 {
			p.MaxCount = 30
		}
		if p.MaxCount < 1 || p.MaxCount > 200 {
			return "", errors.New("rule max_count: between 1 and 200")
		}
		rev, err := c.rev(ctx, gitDir, p.Rev)
		if err != nil {
			return "", err
		}
		args := []string{"log", "--no-show-signature", "--no-color", "--date=short", "--format=%H%x09%an%x09%ad%x09%s", fmt.Sprintf("--max-count=%d", p.MaxCount), "--end-of-options", rev}
		return c.git(ctx, gitDir, args, p.Path)
	case "git_show":
		var p struct {
			Rev  string `json:"rev"`
			Path string `json:"path"`
		}
		if err := strict(raw, &p); err != nil {
			return "", err
		}
		if p.Rev == "" {
			return "", errors.New("rule rev: required")
		}
		rev, err := c.rev(ctx, gitDir, p.Rev)
		if err != nil {
			return "", err
		}
		args := []string{"show", "--no-show-signature", "--no-ext-diff", "--no-textconv", "--no-color", "--format=fuller", "--stat", "--patch", "--end-of-options", rev}
		return c.git(ctx, gitDir, args, p.Path)
	case "git_blame":
		var p struct {
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
			Rev       string `json:"rev"`
		}
		if err := strict(raw, &p); err != nil {
			return "", err
		}
		if p.Path == "" {
			return "", errors.New("rule path: required")
		}
		if p.StartLine < 1 || p.EndLine < p.StartLine || p.EndLine-p.StartLine >= 1000 {
			return "", errors.New("rule start_line, end_line: 1 <= start_line <= end_line, at most 1000 lines")
		}
		rev, err := c.rev(ctx, gitDir, p.Rev)
		if err != nil {
			return "", err
		}
		// blame reads --end-of-options as a revision, and takes a file, not
		// a pathspec. rev is a resolved commit ID, and the path is checked.
		if err := checkPath(p.Path); err != nil {
			return "", err
		}
		out, err := c.repos.summaryGit(ctx, gitDir, maxContext, "blame", "--no-textconv", "-L", fmt.Sprintf("%d,%d", p.StartLine, p.EndLine), rev, "--", p.Path)
		return answer(out, err)
	case "git_diff":
		var p struct {
			From string `json:"from"`
			To   string `json:"to"`
			Path string `json:"path"`
		}
		if err := strict(raw, &p); err != nil {
			return "", err
		}
		if p.From == "" {
			return "", errors.New("rule from: required")
		}
		from, err := c.rev(ctx, gitDir, p.From)
		if err != nil {
			return "", err
		}
		to, err := c.rev(ctx, gitDir, p.To)
		if err != nil {
			return "", err
		}
		args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--end-of-options", from, to}
		return c.git(ctx, gitDir, args, p.Path)
	}
	return "", fmt.Errorf("rule tool.unknown: one of %s", strings.Join(ContextToolNames(), ", "))
}

// revRE is a commit ID, whole or abbreviated, optionally counted back.
var revRE = regexp.MustCompile(`^[0-9a-fA-F]{7,64}(~[0-9]{1,4})?$`)

// rev resolves head, base, or a commit ID in the change's history to a full
// commit ID. Branch and tag names, ranges, and anything git reads as an
// option are refused, so a revision names a commit the change holds.
func (c *askContext) rev(ctx context.Context, gitDir, rev string) (string, error) {
	switch rev {
	case "", "head":
		return c.head, nil
	case "base":
		return c.base, nil
	}
	if !revRE.MatchString(rev) {
		return "", errors.New("rule rev: head, base, or a commit ID in the change's history, optionally with ~N")
	}
	out, err := c.repos.git(ctx, gitDir, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	sha := strings.TrimSpace(out)
	if err != nil || !commitRE.MatchString(sha) {
		return "", fmt.Errorf("rule rev: %s is not a commit in this repository", rev)
	}
	for _, tip := range []string{c.head, c.base} {
		if _, err := c.repos.git(ctx, gitDir, "merge-base", "--is-ancestor", sha, tip); err == nil {
			return sha, nil
		}
	}
	return "", fmt.Errorf("rule rev: %s is not in the change's history", rev)
}

// git runs one read of the clone, with path, when given, as a literal
// repository-relative path, and refuses an answer past the cap.
func (c *askContext) git(ctx context.Context, gitDir string, args []string, path string) (string, error) {
	args = append(args, "--")
	if path != "" {
		if err := checkPath(path); err != nil {
			return "", err
		}
		args = append(args, ":(literal)"+path)
	}
	return answer(c.repos.summaryGit(ctx, gitDir, maxContext, args...))
}

// checkPath refuses a path that is not relative and inside the repository.
func checkPath(path string) error {
	if strings.ContainsRune(path, 0) || strings.ContainsRune(path, '\\') || !filepath.IsLocal(filepath.FromSlash(path)) {
		return errors.New("rule path: a relative path inside the repository")
	}
	return nil
}

// answer is a git read as a tool answers it: refused past the cap, with
// nothing of it sent.
func answer(out string, err error) (string, error) {
	if errors.Is(err, errSummaryLarge) {
		return "", fmt.Errorf("the answer is larger than %d KiB, and nothing of it was sent; narrow it with a path, a smaller range, or max_count", maxContext>>10)
	}
	if err != nil {
		return "", errors.New("git could not answer that; check the revision and the path")
	}
	return out, nil
}

// discussion is the threads an agent reads: each note's author, time, and
// text, with the forge's own system notes left out.
func discussion(f AskFacts, unresolvedOnly bool) map[string]any {
	type note struct {
		Author string    `json:"author"`
		At     time.Time `json:"at"`
		Body   string    `json:"body"`
	}
	type thread struct {
		ID       string `json:"id"`
		Path     string `json:"path,omitempty"`
		Line     int    `json:"line,omitempty"`
		Resolved bool   `json:"resolved"`
		Notes    []note `json:"notes"`
	}
	out := []thread{}
	for _, t := range f.Threads {
		if unresolvedOnly && (t.Resolved || !t.Resolvable) {
			continue
		}
		th := thread{ID: t.ID, Resolved: t.Resolved, Notes: []note{}}
		if t.Anchor != nil {
			th.Path, th.Line = t.Anchor.Path, t.Anchor.NewLine
			if th.Line == 0 {
				th.Line = t.Anchor.OldLine
			}
		}
		for _, n := range t.Notes {
			if !n.System {
				th.Notes = append(th.Notes, note{Author: n.Author.Username, At: n.At, Body: n.Body})
			}
		}
		if len(th.Notes) > 0 {
			out = append(out, th)
		}
	}
	return map[string]any{"threads": out, "read_at": f.ReadAt}
}

func strict(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.New("rule arguments: an object matching the tool's input schema")
	}
	return nil
}

// marshal answers v as JSON, refused past the cap like a git answer.
func marshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if len(b) > maxContext {
		return "", fmt.Errorf("the answer is larger than %d KiB, and nothing of it was sent", maxContext>>10)
	}
	return string(b), nil
}
