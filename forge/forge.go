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
	"time"
)

// Ref names one change on one forge.
type Ref struct {
	Host    string `json:"host"`    // such as gitlab.com
	Project string `json:"project"` // the project's full path, such as group/sub/project
	Number  int    `json:"number"`  // the merge request's iid, or the pull request's number
}

// String is the stable identity used by existing run/cache records. Its
// historical separator is preserved; Display supplies the forge notation.
func (r Ref) String() string { return fmt.Sprintf("%s/%s!%d", r.Host, r.Project, r.Number) }

func (r Ref) Display(kind string) string {
	separator := "!"
	if kind == Forgejo || kind == GitHub {
		separator = "#"
	}
	return fmt.Sprintf("%s/%s%s%d", r.Host, r.Project, separator, r.Number)
}

var (
	ErrNotAChange  = errors.New("not a merge request or pull request URL")
	ErrNotFound    = errors.New("change not found, or not visible to this account")
	ErrUnsupported = errors.New("this forge does not support that action in this version")
)

// State is where a change stands.
type State string

const (
	Open   State = "open"
	Merged State = "merged"
	Closed State = "closed"
)

// Change is one merge request or pull request, as the forge reports it.
type Change struct {
	Kind         string       `json:"kind"`
	Capabilities Capabilities `json:"capabilities"`
	// Checkout is adapter-supplied git metadata, used only by the server.
	Checkout     Checkout `json:"-"`
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
	// detailedMergeStatus. CanMerge says whether the signed-in person can merge.
	Mergeability string `json:"mergeability"`
	// CanMerge is permission of the authenticated person, nil when unknown.
	CanMerge *bool     `json:"can_merge"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`

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
	Bot       bool      `json:"bot"`
	Verdict   Verdict   `json:"verdict"`
	At        time.Time `json:"at"`
	SHA       string    `json:"sha"`
	Dismissed bool      `json:"dismissed"`
	Stale     bool      `json:"stale"`
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
	Path    string `json:"path"`
	OldPath string `json:"old_path"`
	NewLine int    `json:"new_line"` // 0 when the line is only on the old side
	OldLine int    `json:"old_line"` // 0 when the line is only on the new side
	// StartLine is the first new-side line of a comment that spans several,
	// ending at NewLine, for a forge whose suggestion replaces the lines its
	// comment spans. 0 means the comment is on its one line.
	StartLine int      `json:"start_line,omitempty"`
	Diff      DiffRefs `json:"diff"`
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
	// AfterReview marks a reply left pending after the review it was
	// submitted with was published, so retrying it publishes no review.
	AfterReview bool `json:"after_review,omitempty"`
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
	// Submit publishes every pending comment of the person's at once, with
	// the person's verdict, so the review lands as one event. Commented
	// publishes the comments only. Approved also approves. ChangesRequested
	// marks the review as asking for changes, where Capabilities.RequestChanges
	// says the forge can.
	Submit(ctx context.Context, ref Ref, verdict Verdict) error
	Resolve(ctx context.Context, ref Ref, threadID string, resolved bool) error
	// Approve approves, or withdraws the person's approval where
	// Capabilities.Unapprove says the forge can.
	Approve(ctx context.Context, ref Ref, approve bool) error
}

// Uploader attaches a file to a change's project, for a pending comment to
// link, and returns the Markdown that links it (D59). It is the person's
// confirmed action, like every write.
type Uploader interface {
	Upload(ctx context.Context, ref Ref, name string, data []byte) (markdown string, err error)
}

// DraftStore keeps the person's pending comments on a change for a forge that
// cannot keep them privately until they submit (D43). It is vuoro's own
// state, never the forge's, and holds no credential.
type DraftStore interface {
	Drafts(ref Ref) ([]Draft, error)
	SaveDrafts(ref Ref, drafts []Draft) error
}

// LocalDrafter is a client whose pending comments live in a DraftStore. The
// application gives it one after dialing; until then it reports no drafts
// capability.
type LocalDrafter interface {
	KeepDraftsIn(DraftStore)
}

// Stacker opens a change onto another change's source branch, which is how
// vuoro hands back an editor's fixes (D39). The person's token opens it, and
// only at their confirmed action.
type Stacker interface {
	// StackTarget says where a change stacked on ref goes. It refuses a
	// change whose source branch is in a fork, which the person may not be
	// able to push to and whose author owns it.
	StackTarget(ctx context.Context, ref Ref) (StackTarget, error)
	// OpenStacked opens a draft change from s.Branch onto s.Target in ref's
	// project, and returns its web URL. A change the forge opened, but not as
	// a draft, returns its URL with ErrNotDraft.
	OpenStacked(ctx context.Context, ref Ref, s NewStacked) (string, error)
}

// StackTarget is the branch a stacked change goes onto, and its head.
type StackTarget struct {
	Branch string `json:"branch"`
	Head   string `json:"head"`
}

// NewStacked is a draft change to open onto another change's branch.
type NewStacked struct {
	Branch      string // the pushed branch
	Target      string // the reviewed change's source branch
	Title       string
	Description string
}

// ErrNotDraft means a stacked change opened, but the forge did not mark it a
// draft, so anyone who may merge it could.
var ErrNotDraft = errors.New("it opened as an ordinary change, not a draft")

// ErrFork means the change's source branch is in a fork.
var ErrFork = errors.New("this change's source branch is in a fork, which vuoro does not push to; its author owns it")
