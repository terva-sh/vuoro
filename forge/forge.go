// Package forge is vuoro's forge-neutral model of a code review (docs/plan.md
// section 4). It holds no HTTP: each forge's API knowledge lives in its own
// adapter, under forge/gitlab and later forge/github and forge/forgejo.
//
// It imports neither agent nor tickets, and its tests do no network I/O.
package forge

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Ref names one change on one forge.
type Ref struct {
	Host    string `json:"host"`    // such as gitlab.com
	Project string `json:"project"` // the project's full path, such as group/sub/project
	Number  int    `json:"number"`  // the merge request's iid, or the pull request's number
}

func (r Ref) String() string { return fmt.Sprintf("%s/%s!%d", r.Host, r.Project, r.Number) }

// ErrNotAChange means a URL does not name a merge request.
var ErrNotAChange = errors.New("not a merge request URL")

// ParseGitLabURL reads a GitLab merge request URL, such as
// https://gitlab.com/group/project/-/merge_requests/12, with any trailing
// path, query, or fragment.
func ParseGitLabURL(raw string) (Ref, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return Ref{}, fmt.Errorf("%q: %w", raw, ErrNotAChange)
	}
	project, rest, ok := strings.Cut(strings.Trim(u.Path, "/"), "/-/merge_requests/")
	if !ok || project == "" {
		return Ref{}, fmt.Errorf("%q: %w", raw, ErrNotAChange)
	}
	num, _, _ := strings.Cut(rest, "/")
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 {
		return Ref{}, fmt.Errorf("%q: %w", raw, ErrNotAChange)
	}
	return Ref{Host: u.Host, Project: project, Number: n}, nil
}

// State is where a change stands.
type State string

const (
	Open   State = "open"
	Merged State = "merged"
	Closed State = "closed"
)

// Change is one merge request or pull request, as the forge reports it.
type Change struct {
	Ref          Ref      `json:"ref"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Author       User     `json:"author"`
	WebURL       string   `json:"web_url"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Diff         DiffRefs `json:"diff"`
	Draft        bool     `json:"draft"`
	State        State    `json:"state"`
	// Mergeability is the forge's own word for it, such as GitLab's
	// detailedMergeStatus. MergeableByMe says whether the person can merge.
	Mergeability string    `json:"mergeability"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`

	Reviewers []Reviewer `json:"reviewers"`
	Assignees []User     `json:"assignees"`
	// Participants are everyone the forge counts as taking part.
	Participants []User          `json:"participants"`
	Requests     []ReviewRequest `json:"requests"`
	Reviews      []Review        `json:"reviews"`
	Pushes       []Push          `json:"pushes"`
	Threads      []Thread        `json:"threads"`
	Files        []FileDiff      `json:"files"`
	// Drafts are the person's own pending comments on the forge.
	Drafts []Draft `json:"drafts"`
}

// DiffRefs are the SHAs an inline comment's position needs.
type DiffRefs struct {
	Base  string `json:"base"`  // the merge base
	Start string `json:"start"` // the target's head when the diff was taken
	Head  string `json:"head"`  // the change's head
}

// User is an account on the forge.
type User struct {
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

// ReviewState is a reviewer's current state, where the forge keeps one.
type ReviewState string

const (
	Unreviewed       ReviewState = "unreviewed"
	Reviewed         ReviewState = "reviewed"
	RequestedChanges ReviewState = "requested_changes"
	Approved         ReviewState = "approved"
)

// Reviewer is a requested reviewer and where they stand.
type Reviewer struct {
	User
	State    ReviewState `json:"state"`
	CanMerge bool        `json:"can_merge"`
}

// ReviewRequest records that a reviewer was asked, at a time. A re-request is
// a new one.
type ReviewRequest struct {
	Reviewer string    `json:"reviewer"`
	By       string    `json:"by"`
	At       time.Time `json:"at"`
}

// Verdict is what a review said.
type Verdict string

const (
	VerdictApproved         Verdict = "approved"
	VerdictChangesRequested Verdict = "changes_requested"
	VerdictCommented        Verdict = "commented"
)

// Review is one reviewer's verdict at a time. SHA is the head it was given at,
// where the forge records it; GitLab does not.
type Review struct {
	Reviewer  string    `json:"reviewer"`
	Verdict   Verdict   `json:"verdict"`
	At        time.Time `json:"at"`
	SHA       string    `json:"sha"`
	Dismissed bool      `json:"dismissed"`
}

// Push is new commits arriving on the change. Head is set where the forge
// says which commit the push ended at.
type Push struct {
	At   time.Time `json:"at"`
	Head string    `json:"head"`
	By   string    `json:"by"`
}

// Thread is one discussion.
type Thread struct {
	ID         string  `json:"id"`
	Anchor     *Anchor `json:"anchor"` // nil for a general discussion
	Resolvable bool    `json:"resolvable"`
	Resolved   bool    `json:"resolved"`
	Notes      []Note  `json:"notes"`
}

// Anchor places a thread on lines of a file at a SHA.
type Anchor struct {
	Path    string   `json:"path"`
	OldPath string   `json:"old_path"`
	NewLine int      `json:"new_line"` // 0 when the line is only on the old side
	OldLine int      `json:"old_line"` // 0 when the line is only on the new side
	Diff    DiffRefs `json:"diff"`
}

// Note is one comment in a thread.
type Note struct {
	ID     string    `json:"id"`
	Author User      `json:"author"`
	Body   string    `json:"body"`
	At     time.Time `json:"at"`
	// System marks a note the forge wrote, such as "changed this line in
	// version 3 of the diff". It never makes it anyone's turn.
	System bool `json:"system"`
}

// FileDiff is one file's change.
type FileDiff struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
	New     bool   `json:"new"`
	Deleted bool   `json:"deleted"`
	Renamed bool   `json:"renamed"`
	// Patch is the unified diff's hunks, without the --- and +++ header.
	Patch string `json:"patch"`
	// Rebuilt says the forge did not send this diff, because it was too
	// large or collapsed, and the adapter built it from both versions.
	Rebuilt bool `json:"rebuilt"`
	// Binary or too large to rebuild: Patch is empty, and the file is
	// listed so the person knows it changed.
	Omitted bool `json:"omitted"`
}

// Draft is a pending comment that the forge holds unpublished.
type Draft struct {
	ID       string  `json:"id"`
	Body     string  `json:"body"`
	ThreadID string  `json:"thread_id"` // set when it replies to a thread
	Anchor   *Anchor `json:"anchor"`    // set when it is a new inline comment
}

// Opener opens one change with everything a review needs.
type Opener interface {
	Open(ctx context.Context, ref Ref) (*Change, error)
}

// NewDraft is a pending comment the person writes or keeps: a reply when
// ThreadID is set, an inline comment when Anchor is, and otherwise a general
// comment. An inline anchor names the line on both sides when the line is
// unchanged, and its Diff is the change's current DiffRefs.
type NewDraft struct {
	Body     string  `json:"body"`
	ThreadID string  `json:"thread_id,omitempty"`
	Anchor   *Anchor `json:"anchor,omitempty"`
}

// Writer is the write half of an adapter. Each call is one explicit action
// of the person's in the UI: no agent reaches it.
type Writer interface {
	// CreateDraft adds a pending comment that only the person sees until
	// they submit.
	CreateDraft(ctx context.Context, ref Ref, d NewDraft) (Draft, error)
	UpdateDraft(ctx context.Context, ref Ref, id, body string) error
	DeleteDraft(ctx context.Context, ref Ref, id string) error
	// Submit publishes every pending comment of the person's at once, so the
	// review lands as one event.
	Submit(ctx context.Context, ref Ref) error
	Resolve(ctx context.Context, ref Ref, threadID string, resolved bool) error
	Approve(ctx context.Context, ref Ref, approve bool) error
}
