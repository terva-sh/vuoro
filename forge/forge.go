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
	Host    string // such as gitlab.com
	Project string // the project's full path, such as group/sub/project
	Number  int    // the merge request's iid, or the pull request's number
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
	Ref          Ref
	Title        string
	Description  string
	Author       User
	WebURL       string
	SourceBranch string
	TargetBranch string
	Diff         DiffRefs
	Draft        bool
	State        State
	// Mergeability is the forge's own word for it, such as GitLab's
	// detailedMergeStatus. MergeableByMe says whether the person can merge.
	Mergeability string
	Created      time.Time
	Updated      time.Time

	Reviewers []Reviewer
	Assignees []User
	// Participants are everyone the forge counts as taking part.
	Participants []User
	Requests     []ReviewRequest
	Reviews      []Review
	Pushes       []Push
	Threads      []Thread
	Files        []FileDiff
	// Drafts are the person's own pending comments on the forge.
	Drafts []Draft
}

// DiffRefs are the SHAs an inline comment's position needs.
type DiffRefs struct {
	Base  string // the merge base
	Start string // the target's head when the diff was taken
	Head  string // the change's head
}

// User is an account on the forge.
type User struct {
	Username string
	Bot      bool
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
	State    ReviewState
	CanMerge bool
}

// ReviewRequest records that a reviewer was asked, at a time. A re-request is
// a new one.
type ReviewRequest struct {
	Reviewer string
	By       string
	At       time.Time
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
	Reviewer  string
	Verdict   Verdict
	At        time.Time
	SHA       string
	Dismissed bool
}

// Push is new commits arriving on the change. Head is set where the forge
// says which commit the push ended at.
type Push struct {
	At   time.Time
	Head string
	By   string
}

// Thread is one discussion.
type Thread struct {
	ID         string
	Anchor     *Anchor // nil for a general discussion
	Resolvable bool
	Resolved   bool
	Notes      []Note
}

// Anchor places a thread on lines of a file at a SHA.
type Anchor struct {
	Path    string
	OldPath string
	NewLine int // 0 when the line is only on the old side
	OldLine int // 0 when the line is only on the new side
	Diff    DiffRefs
}

// Note is one comment in a thread.
type Note struct {
	ID     string
	Author User
	Body   string
	At     time.Time
	// System marks a note the forge wrote, such as "changed this line in
	// version 3 of the diff". It never makes it anyone's turn.
	System bool
}

// FileDiff is one file's change.
type FileDiff struct {
	OldPath string
	NewPath string
	New     bool
	Deleted bool
	Renamed bool
	// Patch is the unified diff's hunks, without the --- and +++ header.
	Patch string
	// Rebuilt says the forge did not send this diff, because it was too
	// large or collapsed, and the adapter built it from both versions.
	Rebuilt bool
	// Binary or too large to rebuild: Patch is empty, and the file is
	// listed so the person knows it changed.
	Omitted bool
}

// Draft is a pending comment that the forge holds unpublished.
type Draft struct {
	ID       string
	Body     string
	ThreadID string  // set when it replies to a thread
	Anchor   *Anchor // set when it is a new inline comment
}

// Opener opens one change with everything a review needs.
type Opener interface {
	Open(ctx context.Context, ref Ref) (*Change, error)
}
