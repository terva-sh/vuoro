// Package gitlab is the GitLab adapter for the forge model (docs/plan.md
// sections 4.2 and 5). It reads a merge request with GraphQL, and its diff and
// the person's draft notes with REST. All of the GitLab API knowledge lives
// here.
package gitlab

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/patch"
)

// Client talks to one GitLab host as one person.
type Client struct {
	base  *url.URL
	token config.Secret
	http  *http.Client
}

// New returns a client for the GitLab at base, such as https://gitlab.com.
// The token is sent only to base's host, in a header, and never written down.
// An empty token reads anonymously, which public projects allow.
func New(base string, token config.Secret) (*Client, error) {
	u, err := url.Parse(strings.TrimSuffix(base, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("gitlab: bad base URL %q", base)
	}
	return &Client{
		base:  u,
		token: token,
		http: &http.Client{
			Timeout: 60 * time.Second,
			// A redirect could carry the token elsewhere. GitLab's API does
			// not redirect, so any redirect is refused.
			CheckRedirect: func(*http.Request, []*http.Request) error { return errRedirect },
		},
	}, nil
}

var errRedirect = errors.New("gitlab: the server answered with a redirect, which vuoro does not follow")

// Host is the GitLab host this client talks to.
func (c *Client) Host() string { return c.base.Host }

// APIError is an unsuccessful API response. It names the request and the
// status, and never the token.
type APIError struct {
	Method, Path string
	Status       int
	Message      string // GitLab's own message, when it sends one
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("gitlab: %s %s: %d %s", e.Method, e.Path, e.Status, http.StatusText(e.Status))
	// GitLab's message is often the status again, which says nothing new.
	if e.Message != "" && e.Message != fmt.Sprintf("%d %s", e.Status, http.StatusText(e.Status)) {
		msg += ": " + e.Message
	}
	return msg
}

// maxBody bounds any one response.
const maxBody = 32 << 20

// do sends a request and returns the body of a 2xx response.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) ([]byte, http.Header, error) {
	// path is already escaped: a project's full path is one %2F segment.
	target := c.base.String() + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rd)
	if err != nil {
		return nil, nil, err
	}
	if !c.token.Empty() {
		req.Header.Set("Authorization", "Bearer "+c.token.Reveal())
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		// A *url.Error names the URL, which carries no token: the token
		// rides in a header.
		return nil, nil, fmt.Errorf("gitlab: %s %s: %w", method, path, unwrapURLError(err))
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return nil, nil, fmt.Errorf("gitlab: %s %s: %w", method, path, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, nil, &APIError{Method: method, Path: path, Status: res.StatusCode, Message: c.scrub(apiMessage(b))}
	}
	return b, res.Header, nil
}

func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// apiMessage pulls GitLab's message out of an error body, briefly.
func apiMessage(b []byte) string {
	var m struct {
		Message any    `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	s := m.Error
	if m.Message != nil {
		s = fmt.Sprint(m.Message)
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// scrub removes the token from a message the server sent back, in case a
// proxy or a misconfigured server echoes the request.
func (c *Client) scrub(s string) string {
	if t := c.token.Reveal(); t != "" {
		s = strings.ReplaceAll(s, t, "[redacted]")
	}
	return s
}

// projectPath is the API path of a project, with its full path escaped as one
// segment.
func projectPath(project string) string {
	return "/api/v4/projects/" + url.PathEscape(project)
}

//go:embed query.graphql
var mergeRequestQuery string

type gqlUser struct {
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

type gqlPosition struct {
	FilePath string `json:"filePath"`
	OldPath  string `json:"oldPath"`
	NewLine  *int   `json:"newLine"`
	OldLine  *int   `json:"oldLine"`
	DiffRefs struct {
		BaseSha  string `json:"baseSha"`
		HeadSha  string `json:"headSha"`
		StartSha string `json:"startSha"`
	} `json:"diffRefs"`
}

type gqlNote struct {
	ID         string       `json:"id"`
	Author     gqlUser      `json:"author"`
	CreatedAt  time.Time    `json:"createdAt"`
	System     bool         `json:"system"`
	Body       string       `json:"body"`
	Resolvable bool         `json:"resolvable"`
	Resolved   bool         `json:"resolved"`
	Position   *gqlPosition `json:"position"`
}

type gqlMR struct {
	IID                 string    `json:"iid"`
	Title               string    `json:"title"`
	Description         string    `json:"description"`
	State               string    `json:"state"`
	Draft               bool      `json:"draft"`
	DetailedMergeStatus string    `json:"detailedMergeStatus"`
	SourceBranch        string    `json:"sourceBranch"`
	TargetBranch        string    `json:"targetBranch"`
	WebURL              string    `json:"webUrl"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
	Author              gqlUser   `json:"author"`
	DiffRefs            *struct {
		BaseSha  string `json:"baseSha"`
		HeadSha  string `json:"headSha"`
		StartSha string `json:"startSha"`
	} `json:"diffRefs"`
	Reviewers struct {
		Nodes []struct {
			gqlUser
			Interaction struct {
				ReviewState string `json:"reviewState"`
				CanMerge    bool   `json:"canMerge"`
			} `json:"mergeRequestInteraction"`
		} `json:"nodes"`
	} `json:"reviewers"`
	Assignees struct {
		Nodes []gqlUser `json:"nodes"`
	} `json:"assignees"`
	Participants struct {
		Nodes []gqlUser `json:"nodes"`
	} `json:"participants"`
	Diffs struct {
		Nodes []struct {
			CreatedAt time.Time `json:"createdAt"`
		} `json:"nodes"`
	} `json:"mergeRequestDiffs"`
	Discussions struct {
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
		Nodes []struct {
			ID         string `json:"id"`
			Resolvable bool   `json:"resolvable"`
			Resolved   bool   `json:"resolved"`
			Notes      struct {
				Nodes []gqlNote `json:"nodes"`
			} `json:"notes"`
		} `json:"nodes"`
	} `json:"discussions"`
}

type gqlResponse struct {
	Data struct {
		Project *struct {
			MergeRequest *gqlMR `json:"mergeRequest"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ErrNotFound means the project or the merge request does not exist, or the
// person cannot see it.
var ErrNotFound = errors.New("gitlab: merge request not found, or not visible to this account")

// Open reads a merge request with its diff, threads, review state, and the
// person's own draft notes.
func (c *Client) Open(ctx context.Context, ref forge.Ref) (*forge.Change, error) {
	if ref.Host != c.base.Host {
		return nil, fmt.Errorf("gitlab: %s is on %s, and this client talks to %s", ref, ref.Host, c.base.Host)
	}
	mr, err := c.readMR(ctx, ref)
	if err != nil {
		return nil, err
	}
	ch := convert(ref, mr)
	if ch.Files, err = c.diffs(ctx, ref, ch.Diff); err != nil {
		return nil, err
	}
	if c.token.Empty() { // an anonymous reader has no drafts
		return ch, nil
	}
	if ch.Drafts, err = c.drafts(ctx, ref); err != nil {
		return nil, err
	}
	return ch, nil
}

// readMR runs the merge request query, following the discussions' pages.
func (c *Client) readMR(ctx context.Context, ref forge.Ref) (*gqlMR, error) {
	var mr *gqlMR
	after := ""
	for page := 0; ; page++ {
		vars := map[string]any{"fullPath": ref.Project, "iid": strconv.Itoa(ref.Number)}
		if after != "" {
			vars["after"] = after
		}
		b, _, err := c.do(ctx, "POST", "/api/graphql", nil, map[string]any{"query": mergeRequestQuery, "variables": vars})
		if err != nil {
			return nil, err
		}
		var r gqlResponse
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("gitlab: graphql: %w", err)
		}
		if len(r.Errors) > 0 {
			return nil, fmt.Errorf("gitlab: graphql: %s", c.scrub(r.Errors[0].Message))
		}
		if r.Data.Project == nil || r.Data.Project.MergeRequest == nil {
			return nil, fmt.Errorf("%s: %w", ref, ErrNotFound)
		}
		got := r.Data.Project.MergeRequest
		if mr == nil {
			mr = got
		} else {
			mr.Discussions.Nodes = append(mr.Discussions.Nodes, got.Discussions.Nodes...)
		}
		pi := got.Discussions.PageInfo
		if !pi.HasNextPage || pi.EndCursor == "" || page > 50 {
			return mr, nil
		}
		after = pi.EndCursor
	}
}

var reviewStates = map[string]forge.ReviewState{
	"UNREVIEWED":        forge.Unreviewed,
	"REVIEW_STARTED":    forge.Unreviewed,
	"UNAPPROVED":        forge.Unreviewed,
	"REVIEWED":          forge.Reviewed,
	"REQUESTED_CHANGES": forge.RequestedChanges,
	"APPROVED":          forge.Approved,
}

var states = map[string]forge.State{"opened": forge.Open, "merged": forge.Merged, "closed": forge.Closed, "locked": forge.Closed}

func user(u gqlUser) forge.User { return forge.User{Username: u.Username, Bot: u.Bot} }

func users(us []gqlUser) []forge.User {
	out := make([]forge.User, 0, len(us))
	for _, u := range us {
		out = append(out, user(u))
	}
	return out
}

func convert(ref forge.Ref, mr *gqlMR) *forge.Change {
	ch := &forge.Change{
		Ref: ref, Title: mr.Title, Description: mr.Description, Author: user(mr.Author),
		WebURL: mr.WebURL, SourceBranch: mr.SourceBranch, TargetBranch: mr.TargetBranch,
		Draft: mr.Draft, State: states[mr.State], Mergeability: strings.ToLower(mr.DetailedMergeStatus),
		Created: mr.CreatedAt, Updated: mr.UpdatedAt,
		Assignees: users(mr.Assignees.Nodes), Participants: users(mr.Participants.Nodes),
	}
	if mr.DiffRefs != nil {
		ch.Diff = forge.DiffRefs{Base: mr.DiffRefs.BaseSha, Start: mr.DiffRefs.StartSha, Head: mr.DiffRefs.HeadSha}
	}
	for _, r := range mr.Reviewers.Nodes {
		ch.Reviewers = append(ch.Reviewers, forge.Reviewer{
			User: user(r.gqlUser), State: reviewStates[r.Interaction.ReviewState], CanMerge: r.Interaction.CanMerge,
		})
	}
	for _, d := range mr.Diffs.Nodes {
		ch.Pushes = append(ch.Pushes, forge.Push{At: d.CreatedAt})
	}
	slices.SortFunc(ch.Pushes, func(a, b forge.Push) int { return a.At.Compare(b.At) })
	if n := len(ch.Pushes); n > 0 {
		ch.Pushes[n-1].Head = ch.Diff.Head // the newest version is the head
	}

	for _, d := range mr.Discussions.Nodes {
		notes := d.Notes.Nodes
		if len(notes) == 0 {
			continue
		}
		// A discussion of system notes only is an event, not a thread.
		if !slices.ContainsFunc(notes, func(n gqlNote) bool { return !n.System }) {
			for _, n := range notes {
				events(ch, n)
			}
			continue
		}
		// REST and draft notes name a discussion by its bare id, so the model
		// does too.
		t := forge.Thread{ID: strings.TrimPrefix(d.ID, "gid://gitlab/Discussion/"), Resolvable: d.Resolvable, Resolved: d.Resolved}
		if p := notes[0].Position; p != nil && p.FilePath != "" {
			t.Anchor = &forge.Anchor{
				Path: p.FilePath, OldPath: p.OldPath, NewLine: deref(p.NewLine), OldLine: deref(p.OldLine),
				Diff: forge.DiffRefs{Base: p.DiffRefs.BaseSha, Start: p.DiffRefs.StartSha, Head: p.DiffRefs.HeadSha},
			}
		}
		for _, n := range notes {
			t.Notes = append(t.Notes, forge.Note{ID: strings.TrimPrefix(n.ID, "gid://gitlab/Note/"), Author: user(n.Author), Body: n.Body, At: n.CreatedAt, System: n.System})
		}
		ch.Threads = append(ch.Threads, t)
	}
	return ch
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

var requestedReviewRE = regexp.MustCompile(`^requested review from (.+)$`)
var mentionRE = regexp.MustCompile(`@([A-Za-z0-9_.\-]+)`)

// events reads the review facts GitLab records only as system notes: review
// requests and verdicts, with their times (docs/plan.md section 4).
func events(ch *forge.Change, n gqlNote) {
	body := strings.TrimSpace(n.Body)
	by := n.Author.Username
	switch {
	case requestedReviewRE.MatchString(body):
		for _, m := range mentionRE.FindAllStringSubmatch(body, -1) {
			ch.Requests = append(ch.Requests, forge.ReviewRequest{Reviewer: m[1], By: by, At: n.CreatedAt})
		}
	case body == "approved this merge request":
		ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: by, Verdict: forge.VerdictApproved, At: n.CreatedAt})
	case body == "requested changes":
		ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: by, Verdict: forge.VerdictChangesRequested, At: n.CreatedAt})
	case body == "unapproved this merge request":
		for i := len(ch.Reviews) - 1; i >= 0; i-- {
			if ch.Reviews[i].Reviewer == by && ch.Reviews[i].Verdict == forge.VerdictApproved {
				ch.Reviews[i].Dismissed = true
				break
			}
		}
	}
}

type restDiff struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	Diff        string `json:"diff"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
	Collapsed   bool   `json:"collapsed"`
	TooLarge    bool   `json:"too_large"`
}

// maxRebuild bounds a file the adapter fetches to rebuild a diff GitLab
// would not send.
const maxRebuild = 1 << 20

// diffs reads every file's diff, rebuilding from both versions any file that
// GitLab collapsed or sent too large to show.
func (c *Client) diffs(ctx context.Context, ref forge.Ref, refs forge.DiffRefs) ([]forge.FileDiff, error) {
	var out []forge.FileDiff
	for page := 1; page > 0; {
		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"100"}}
		b, h, err := c.do(ctx, "GET", fmt.Sprintf("%s/merge_requests/%d/diffs", projectPath(ref.Project), ref.Number), q, nil)
		if err != nil {
			return nil, err
		}
		var files []restDiff
		if err := json.Unmarshal(b, &files); err != nil {
			return nil, fmt.Errorf("gitlab: diffs: %w", err)
		}
		for _, f := range files {
			fd := forge.FileDiff{OldPath: f.OldPath, NewPath: f.NewPath, New: f.NewFile, Deleted: f.DeletedFile, Renamed: f.RenamedFile, Patch: f.Diff}
			if truncated(f) {
				if err := c.rebuild(ctx, ref, refs, &fd); err != nil {
					return nil, err
				}
			}
			out = append(out, fd)
		}
		page, _ = strconv.Atoi(h.Get("X-Next-Page"))
	}
	return out, nil
}

// truncated reports whether GitLab left a file's diff out. An empty diff is
// legitimate for a rename with no change in content.
func truncated(f restDiff) bool {
	if f.Collapsed || f.TooLarge {
		return true
	}
	return f.Diff == "" && !(f.RenamedFile && f.OldPath != f.NewPath)
}

func (c *Client) rebuild(ctx context.Context, ref forge.Ref, refs forge.DiffRefs, fd *forge.FileDiff) error {
	var old, new []byte
	var err error
	if !fd.New {
		if old, err = c.raw(ctx, ref, fd.OldPath, refs.Base); err != nil {
			return err
		}
	}
	if !fd.Deleted {
		if new, err = c.raw(ctx, ref, fd.NewPath, refs.Head); err != nil {
			return err
		}
	}
	if old == nil && !fd.New || new == nil && !fd.Deleted || binary(old) || binary(new) {
		fd.Omitted = true
		fd.Patch = ""
		return nil
	}
	fd.Patch = patch.Unified(string(old), string(new))
	fd.Rebuilt = true
	return nil
}

// raw reads one file at a SHA. A file larger than maxRebuild comes back nil.
func (c *Client) raw(ctx context.Context, ref forge.Ref, path, sha string) ([]byte, error) {
	b, _, err := c.do(ctx, "GET", fmt.Sprintf("%s/repository/files/%s/raw", projectPath(ref.Project), url.PathEscape(path)), url.Values{"ref": {sha}}, nil)
	if err != nil {
		return nil, err
	}
	if len(b) > maxRebuild {
		return nil, nil
	}
	return b, nil
}

func binary(b []byte) bool { return bytes.IndexByte(b, 0) >= 0 }

type restDraft struct {
	ID           int    `json:"id"`
	Note         string `json:"note"`
	DiscussionID string `json:"discussion_id"`
	Position     *struct {
		BaseSHA  string `json:"base_sha"`
		StartSHA string `json:"start_sha"`
		HeadSHA  string `json:"head_sha"`
		OldPath  string `json:"old_path"`
		NewPath  string `json:"new_path"`
		OldLine  *int   `json:"old_line"`
		NewLine  *int   `json:"new_line"`
	} `json:"position"`
}

// drafts reads the person's own draft notes on the merge request.
func (c *Client) drafts(ctx context.Context, ref forge.Ref) ([]forge.Draft, error) {
	b, _, err := c.do(ctx, "GET", fmt.Sprintf("%s/merge_requests/%d/draft_notes", projectPath(ref.Project), ref.Number), nil, nil)
	if err != nil {
		return nil, err
	}
	var ds []restDraft
	if err := json.Unmarshal(b, &ds); err != nil {
		return nil, fmt.Errorf("gitlab: draft notes: %w", err)
	}
	out := make([]forge.Draft, 0, len(ds))
	for _, d := range ds {
		fd := forge.Draft{ID: strconv.Itoa(d.ID), Body: d.Note, ThreadID: d.DiscussionID}
		if p := d.Position; p != nil && p.NewPath != "" {
			fd.Anchor = &forge.Anchor{Path: p.NewPath, OldPath: p.OldPath, NewLine: deref(p.NewLine), OldLine: deref(p.OldLine),
				Diff: forge.DiffRefs{Base: p.BaseSHA, Start: p.StartSHA, Head: p.HeadSHA}}
		}
		out = append(out, fd)
	}
	return out, nil
}
