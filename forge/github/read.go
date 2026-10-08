package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

// connection is one of a pull request's GraphQL lists. The read asks for the
// first hundred of each; one with more is paged by cursor in its own query.
type connection struct {
	name, args, nodes string
}

const actor = `login __typename`

const threadComment = `id author { ` + actor + ` } body createdAt commit { oid } originalCommit { oid } pullRequestReview { id state }`

// threadPageQuery reads one later page of a review thread's comments.
const threadPageQuery = "query($id: ID!, $after: String!) { node(id: $id) { ... on PullRequestReviewThread { comments(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { " + threadComment + " } } } } }"

var connections = []connection{
	{"reviewRequests", "", `requestedReviewer { __typename ... on User { login } ... on Bot { login } ... on Mannequin { login } }`},
	{"reviews", "", `id author { ` + actor + ` } state body submittedAt commit { oid }`},
	{"reviewThreads", "", `id isResolved path line originalLine diffSide comments(first: 100) { pageInfo { hasNextPage endCursor } nodes { ` + threadComment + ` } }`},
	{"comments", "", `id author { ` + actor + ` } body createdAt`},
	{"assignees", "", `login`},
	{"timelineItems", "itemTypes: [REVIEW_REQUESTED_EVENT, REVIEW_DISMISSED_EVENT, HEAD_REF_FORCE_PUSHED_EVENT]", `__typename
		... on ReviewRequestedEvent { createdAt actor { login } requestedReviewer { __typename ... on User { login } ... on Bot { login } ... on Mannequin { login } } }
		... on ReviewDismissedEvent { previousReviewState review { id } }
		... on HeadRefForcePushedEvent { createdAt actor { login } afterCommit { oid } }`},
}

// maxPages bounds the paging of one connection, at a hundred nodes a page.
const maxPages = 50

const prFields = `number title body url state isDraft createdAt updatedAt changedFiles
	author { ` + actor + ` } baseRefName headRefName baseRefOid headRefOid mergeStateStatus`

func (k connection) field(after bool) string {
	args := "first: 100"
	if after {
		args += ", after: $after"
	}
	if k.args != "" {
		args += ", " + k.args
	}
	return fmt.Sprintf("%s(%s) { pageInfo { hasNextPage endCursor } nodes { %s } }", k.name, args, k.nodes)
}

// readQuery reads the repository and the pull request with the first page of
// every connection.
func readQuery() string {
	var b strings.Builder
	b.WriteString("query($owner: String!, $name: String!, $number: Int!) { repository(owner: $owner, name: $name) { nameWithOwner url sshUrl viewerPermission pullRequest(number: $number) { ")
	b.WriteString(prFields)
	for _, k := range connections {
		b.WriteString(" " + k.field(false))
	}
	b.WriteString(" } } }")
	return b.String()
}

// pageQuery reads one later page of k, with the fields that tell whether the
// pull request moved since the first page.
func pageQuery(k connection) string {
	return "query($owner: String!, $name: String!, $number: Int!, $after: String!) { repository(owner: $owner, name: $name) { pullRequest(number: $number) { updatedAt headRefOid " + k.field(true) + " } } }"
}

const checkQuery = "query($owner: String!, $name: String!, $number: Int!) { repository(owner: $owner, name: $name) { pullRequest(number: $number) { updatedAt headRefOid baseRefOid state } } }"

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type rawConnection struct {
	PageInfo pageInfo          `json:"pageInfo"`
	Nodes    []json.RawMessage `json:"nodes"`
}

type gqlActor struct {
	Login    string `json:"login"`
	Typename string `json:"__typename"`
}

func (a *gqlActor) user() forge.User {
	if a == nil {
		return forge.User{Username: "ghost"} // a deleted account
	}
	return forge.User{Username: a.Login, Bot: a.Typename == "Bot" || strings.HasSuffix(a.Login, "[bot]")}
}

type gqlOID struct {
	OID string `json:"oid"`
}

type gqlPR struct {
	Number           int       `json:"number"`
	Title            string    `json:"title"`
	Body             string    `json:"body"`
	URL              string    `json:"url"`
	State            string    `json:"state"`
	IsDraft          bool      `json:"isDraft"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Author           *gqlActor `json:"author"`
	BaseRefName      string    `json:"baseRefName"`
	HeadRefName      string    `json:"headRefName"`
	BaseRefOID       string    `json:"baseRefOid"`
	HeadRefOID       string    `json:"headRefOid"`
	MergeStateStatus string    `json:"mergeStateStatus"`
	ChangedFiles     int       `json:"changedFiles"`

	conns map[string]*rawConnection
	// paged says a thread was read in more than one query, which a move
	// between them could have split.
	paged bool
}

type gqlRepo struct {
	NameWithOwner    string          `json:"nameWithOwner"`
	URL              string          `json:"url"`
	SSHURL           string          `json:"sshUrl"`
	ViewerPermission string          `json:"viewerPermission"`
	PullRequest      json.RawMessage `json:"pullRequest"`
}

type gqlRequested struct {
	Typename string `json:"__typename"`
	Login    string `json:"login"`
}

type gqlReview struct {
	ID          string     `json:"id"`
	Author      *gqlActor  `json:"author"`
	State       string     `json:"state"`
	Body        string     `json:"body"`
	SubmittedAt *time.Time `json:"submittedAt"`
	Commit      *gqlOID    `json:"commit"`
}

type gqlComment struct {
	ID             string    `json:"id"`
	Author         *gqlActor `json:"author"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"createdAt"`
	Commit         *gqlOID   `json:"commit"`
	OriginalCommit *gqlOID   `json:"originalCommit"`
	Review         *struct {
		ID    string `json:"id"`
		State string `json:"state"`
	} `json:"pullRequestReview"`
}

type gqlThread struct {
	ID           string `json:"id"`
	IsResolved   bool   `json:"isResolved"`
	Path         string `json:"path"`
	Line         *int   `json:"line"`
	OriginalLine *int   `json:"originalLine"`
	DiffSide     string `json:"diffSide"`
	Comments     struct {
		PageInfo pageInfo     `json:"pageInfo"`
		Nodes    []gqlComment `json:"nodes"`
	} `json:"comments"`
}

type gqlEvent struct {
	Typename            string        `json:"__typename"`
	CreatedAt           time.Time     `json:"createdAt"`
	Actor               *gqlActor     `json:"actor"`
	RequestedReviewer   *gqlRequested `json:"requestedReviewer"`
	PreviousReviewState string        `json:"previousReviewState"`
	Review              *struct {
		ID string `json:"id"`
	} `json:"review"`
	AfterCommit *gqlOID `json:"afterCommit"`
}

func ownerName(ref forge.Ref) (string, string) {
	owner, name, _ := strings.Cut(ref.Project, "/")
	return owner, name
}

func (c *Client) Open(ctx context.Context, ref forge.Ref) (*forge.Change, error) {
	return c.readChange(ctx, ref, true)
}

// ReadFacts reads current review facts without patches or private drafts.
func (c *Client) ReadFacts(ctx context.Context, ref forge.Ref) (*forge.Change, error) {
	return c.readChange(ctx, ref, false)
}

var errMoved = errors.New("github: pull request moved while reading; refresh it and try again")

// fetch reads the pull request and every page of its connections. It
// refuses a pull request that moved between pages.
func (c *Client) fetch(ctx context.Context, ref forge.Ref) (*gqlRepo, *gqlPR, error) {
	owner, name := ownerName(ref)
	vars := map[string]any{"owner": owner, "name": name, "number": ref.Number}
	var out struct {
		Repository *gqlRepo `json:"repository"`
	}
	if err := c.query(ctx, readQuery(), vars, &out); err != nil {
		return nil, nil, err
	}
	if out.Repository == nil || len(out.Repository.PullRequest) == 0 || string(out.Repository.PullRequest) == "null" {
		return nil, nil, forge.ErrNotFound
	}
	var p gqlPR
	if err := json.Unmarshal(out.Repository.PullRequest, &p); err != nil {
		return nil, nil, errors.New("github: invalid pull request")
	}
	var all map[string]json.RawMessage
	_ = json.Unmarshal(out.Repository.PullRequest, &all)
	p.conns = map[string]*rawConnection{}
	for _, k := range connections {
		var conn rawConnection
		if err := json.Unmarshal(all[k.name], &conn); err != nil {
			return nil, nil, fmt.Errorf("github: invalid %s", k.name)
		}
		for page := 1; conn.PageInfo.HasNextPage; page++ {
			if page >= maxPages {
				return nil, nil, fmt.Errorf("github: the pull request has more %s than vuoro reads", k.name)
			}
			var next struct {
				Repository struct {
					PullRequest map[string]json.RawMessage `json:"pullRequest"`
				} `json:"repository"`
			}
			v := map[string]any{"owner": owner, "name": name, "number": ref.Number, "after": conn.PageInfo.EndCursor}
			if err := c.query(ctx, pageQuery(k), v, &next); err != nil {
				return nil, nil, err
			}
			pr := next.Repository.PullRequest
			var updated time.Time
			var head string
			_ = json.Unmarshal(pr["updatedAt"], &updated)
			_ = json.Unmarshal(pr["headRefOid"], &head)
			if !updated.Equal(p.UpdatedAt) || head != p.HeadRefOID {
				return nil, nil, errMoved
			}
			var more rawConnection
			if err := json.Unmarshal(pr[k.name], &more); err != nil {
				return nil, nil, fmt.Errorf("github: invalid %s", k.name)
			}
			conn.Nodes = append(conn.Nodes, more.Nodes...)
			conn.PageInfo = more.PageInfo
		}
		p.conns[k.name] = &conn
	}
	if err := c.threadPages(ctx, &p); err != nil {
		return nil, nil, err
	}
	return out.Repository, &p, nil
}

// threadPages reads the rest of every thread with more than a hundred
// comments. A thread's page carries no pull request fields, so the read is
// checked against a move once at the end, by unmoved.
func (c *Client) threadPages(ctx context.Context, p *gqlPR) error {
	conn := p.conns["reviewThreads"]
	for i, raw := range conn.Nodes {
		var t gqlThread
		if err := json.Unmarshal(raw, &t); err != nil {
			return errors.New("github: invalid reviewThreads")
		}
		if !t.Comments.PageInfo.HasNextPage {
			continue
		}
		for page := 1; t.Comments.PageInfo.HasNextPage; page++ {
			if page >= maxPages {
				return fmt.Errorf("github: a thread on %s has more comments than vuoro reads", t.Path)
			}
			var next struct {
				Node struct {
					Comments struct {
						PageInfo pageInfo     `json:"pageInfo"`
						Nodes    []gqlComment `json:"nodes"`
					} `json:"comments"`
				} `json:"node"`
			}
			if err := c.query(ctx, threadPageQuery, map[string]any{"id": t.ID, "after": t.Comments.PageInfo.EndCursor}, &next); err != nil {
				return err
			}
			t.Comments.Nodes = append(t.Comments.Nodes, next.Node.Comments.Nodes...)
			t.Comments.PageInfo = next.Node.Comments.PageInfo
		}
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		conn.Nodes[i] = b
		p.paged = true
	}
	return nil
}

func nodes[T any](p *gqlPR, name string) ([]T, error) {
	out := make([]T, 0, len(p.conns[name].Nodes))
	for _, raw := range p.conns[name].Nodes {
		var n T
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, fmt.Errorf("github: invalid %s", name)
		}
		out = append(out, n)
	}
	return out, nil
}

var verdicts = map[string]forge.Verdict{"APPROVED": forge.VerdictApproved, "CHANGES_REQUESTED": forge.VerdictChangesRequested, "COMMENTED": forge.VerdictCommented}

func (c *Client) readChange(ctx context.Context, ref forge.Ref, workspace bool) (*forge.Change, error) {
	if !strings.EqualFold(ref.Host, c.web.Host) || len(strings.Split(ref.Project, "/")) != 2 || ref.Number < 1 {
		return nil, errors.New("github: change does not belong to this client")
	}
	repo, p, err := c.fetch(ctx, ref)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(repo.NameWithOwner, ref.Project) || p.Number != ref.Number {
		return nil, errors.New("github: repository metadata is missing or mismatched")
	}
	ch := &forge.Change{Kind: forge.GitHub, Ref: ref, Title: p.Title, Description: p.Body, Author: p.Author.user(),
		WebURL: c.web.String() + "/" + ref.Project + "/pull/" + strconv.Itoa(ref.Number), SourceBranch: p.HeadRefName, TargetBranch: p.BaseRefName,
		Diff: forge.DiffRefs{Start: p.BaseRefOID, Head: p.HeadRefOID}, Draft: p.IsDraft, Created: p.CreatedAt, Updated: p.UpdatedAt,
		Mergeability: strings.ToLower(p.MergeStateStatus),
		Checkout:     forge.Checkout{HTTPS: repo.URL + ".git", SSH: repo.SSHURL, HeadRef: fmt.Sprintf("refs/pull/%d/head", ref.Number)}}
	switch p.State {
	case "MERGED":
		ch.State = forge.Merged
	case "CLOSED":
		ch.State = forge.Closed
	default:
		ch.State = forge.Open
	}
	// Merging needs write access; branch protection is in mergeStateStatus,
	// which is CLEAN only when the pull request may merge now.
	canMerge := slices.Contains([]string{"ADMIN", "MAINTAIN", "WRITE"}, repo.ViewerPermission)
	ch.CanMerge = &canMerge
	if err := forge.ValidateCheckout(ch.Checkout, ref.Host); err != nil {
		return nil, fmt.Errorf("github: %w", err)
	}
	if err := convert(ch, p, workspace); err != nil {
		return nil, err
	}
	// The person's comments kept in vuoro follow those in their pending
	// review.
	if workspace {
		c.draftMu.Lock()
		s := c.drafts
		c.draftMu.Unlock()
		if s != nil {
			local, err := s.Drafts(ref)
			if err != nil {
				return nil, err
			}
			ch.Drafts = append(ch.Drafts, local...)
		}
	}
	if !workspace && p.paged {
		if err := c.unmoved(ctx, ref, p); err != nil {
			return nil, err
		}
	}
	if workspace {
		if ch.Diff.Base, err = c.mergeBase(ctx, ref, p.BaseRefOID, p.HeadRefOID); err != nil {
			return nil, err
		}
		if ch.Files, err = c.files(ctx, ref, p.ChangedFiles); err != nil {
			return nil, err
		}
		// The patches are read from the current pull request, not the head
		// the query saw.
		if err := c.unmoved(ctx, ref, p); err != nil {
			return nil, err
		}
	}
	ch.Capabilities = c.Capabilities()
	return ch, nil
}

// convert fills ch's review facts from p's connections, and with drafts its
// viewer's pending review as drafts.
func convert(ch *forge.Change, p *gqlPR, drafts bool) error {
	requested, err := nodes[struct {
		RequestedReviewer *gqlRequested `json:"requestedReviewer"`
	}](p, "reviewRequests")
	if err != nil {
		return err
	}
	reviews, err := nodes[gqlReview](p, "reviews")
	if err != nil {
		return err
	}
	threads, err := nodes[gqlThread](p, "reviewThreads")
	if err != nil {
		return err
	}
	comments, err := nodes[gqlComment](p, "comments")
	if err != nil {
		return err
	}
	assignees, err := nodes[gqlActor](p, "assignees")
	if err != nil {
		return err
	}
	events, err := nodes[gqlEvent](p, "timelineItems")
	if err != nil {
		return err
	}
	for _, a := range assignees {
		ch.Assignees = append(ch.Assignees, a.user())
	}
	// A dismissed review keeps its verdict only in the timeline (§4.2).
	dismissed := map[string]forge.Verdict{}
	for _, e := range events {
		switch e.Typename {
		case "ReviewRequestedEvent":
			// A team, or an account this token cannot see, names nobody.
			if e.RequestedReviewer != nil && e.RequestedReviewer.Login != "" {
				ch.Requests = append(ch.Requests, forge.ReviewRequest{Reviewer: e.RequestedReviewer.Login, By: e.Actor.user().Username, At: e.CreatedAt})
			}
		case "ReviewDismissedEvent":
			if e.Review != nil {
				dismissed[e.Review.ID] = verdicts[e.PreviousReviewState]
			}
		case "HeadRefForcePushedEvent":
			push := forge.Push{At: e.CreatedAt, By: e.Actor.user().Username}
			if e.AfterCommit != nil {
				push.Head = e.AfterCommit.OID
			}
			ch.Pushes = append(ch.Pushes, push)
		}
	}
	// A pending review has no submit time. It is the person's draft, which
	// stays out of the facts. GitHub shows only the viewer their own.
	for _, r := range reviews {
		if r.State == "PENDING" && drafts && strings.TrimSpace(r.Body) != "" {
			ch.Drafts = append(ch.Drafts, forge.Draft{ID: "review-" + r.ID, Body: r.Body})
		}
	}
	reviews = slices.DeleteFunc(reviews, func(r gqlReview) bool { return r.State == "PENDING" || r.SubmittedAt == nil })
	slices.SortStableFunc(reviews, func(a, b gqlReview) int { return a.SubmittedAt.Compare(*b.SubmittedAt) })
	for _, r := range reviews {
		author := r.Author.user()
		if r.Body != "" {
			ch.Threads = append(ch.Threads, forge.Thread{ID: r.ID, Notes: []forge.Note{{ID: r.ID, Author: author, Body: r.Body, At: *r.SubmittedAt}}})
		}
		verdict, ok := verdicts[r.State]
		if r.State == "DISMISSED" {
			verdict, ok = dismissed[r.ID], true
			if verdict == "" {
				// GitHub dismisses only approvals and change requests.
				verdict = forge.VerdictChangesRequested
			}
		}
		if !ok {
			continue
		}
		sha := ""
		if r.Commit != nil {
			sha = r.Commit.OID
		}
		addReview(ch, forge.Review{Reviewer: author.Username, Bot: author.Bot, Verdict: verdict, At: *r.SubmittedAt, SHA: sha, Dismissed: r.State == "DISMISSED"}, author)
	}
	// GitHub lists a reviewer in reviewRequests only while the request is
	// unanswered, so a listed reviewer has not reviewed since it was made.
	for _, r := range requested {
		if r.RequestedReviewer == nil || r.RequestedReviewer.Login == "" {
			continue
		}
		u := forge.User{Username: r.RequestedReviewer.Login, Bot: r.RequestedReviewer.Typename == "Bot"}
		if i := slices.IndexFunc(ch.Reviewers, func(x forge.Reviewer) bool { return x.Username == u.Username }); i >= 0 {
			ch.Reviewers[i].State = forge.Unreviewed
		} else {
			ch.Reviewers = append(ch.Reviewers, forge.Reviewer{User: u, State: forge.Unreviewed})
		}
	}
	for _, t := range threads {
		th := forge.Thread{ID: t.ID, Resolvable: true, Resolved: t.IsResolved}
		for i, n := range t.Comments.Nodes {
			if n.Review != nil && n.Review.State == "PENDING" {
				// A draft: a new thread's first comment, or a reply.
				if drafts && i == 0 {
					ch.Drafts = append(ch.Drafts, forge.Draft{ID: n.ID, Body: n.Body, Anchor: anchor(t, n)})
				} else if drafts {
					ch.Drafts = append(ch.Drafts, forge.Draft{ID: n.ID, Body: n.Body, ThreadID: t.ID})
				}
				continue
			}
			if th.Anchor == nil {
				th.Anchor = anchor(t, n)
			}
			th.Notes = append(th.Notes, forge.Note{ID: n.ID, Author: n.Author.user(), Body: n.Body, At: n.CreatedAt})
		}
		if len(th.Notes) > 0 {
			ch.Threads = append(ch.Threads, th)
		}
	}
	for _, n := range comments {
		ch.Threads = append(ch.Threads, forge.Thread{ID: n.ID, Notes: []forge.Note{{ID: n.ID, Author: n.Author.user(), Body: n.Body, At: n.CreatedAt}}})
	}
	slices.SortStableFunc(ch.Threads, func(a, b forge.Thread) int { return a.Notes[0].At.Compare(b.Notes[0].At) })
	seen := map[string]bool{}
	add := func(u forge.User) {
		if u.Username != "" && !seen[u.Username] {
			seen[u.Username] = true
			ch.Participants = append(ch.Participants, u)
		}
	}
	add(ch.Author)
	for _, t := range ch.Threads {
		for _, n := range t.Notes {
			add(n.Author)
		}
	}
	return nil
}

// anchor places a thread from its first comment. A thread whose lines left
// the diff has no current line, so it stays where it was written, on the
// comment's original commit.
func anchor(t gqlThread, first gqlComment) *forge.Anchor {
	a := &forge.Anchor{Path: t.Path, OldPath: t.Path}
	line, commit := t.Line, first.Commit
	if line == nil {
		line, commit = t.OriginalLine, first.OriginalCommit
	}
	if line != nil {
		if t.DiffSide == "LEFT" {
			a.OldLine = *line
		} else {
			a.NewLine = *line
		}
	}
	if commit != nil {
		a.Diff.Head = commit.OID
	}
	return a
}

// addReview records r and folds it into its reviewer's current state. A
// verdict dismissed, or given at an earlier head, no longer stands, and a
// comment never replaces a verdict that does.
func addReview(ch *forge.Change, r forge.Review, author forge.User) {
	ch.Reviews = append(ch.Reviews, r)
	i := slices.IndexFunc(ch.Reviewers, func(u forge.Reviewer) bool { return u.Username == r.Reviewer })
	if i < 0 {
		i = len(ch.Reviewers)
		ch.Reviewers = append(ch.Reviewers, forge.Reviewer{User: author, State: forge.Reviewed})
	}
	if r.Dismissed || r.SHA != ch.Diff.Head {
		if r.Verdict != forge.VerdictCommented {
			ch.Reviewers[i].State = forge.Reviewed
		}
		return
	}
	state := map[forge.Verdict]forge.ReviewState{forge.VerdictApproved: forge.Approved, forge.VerdictChangesRequested: forge.RequestedChanges, forge.VerdictCommented: forge.Reviewed}[r.Verdict]
	if r.Verdict == forge.VerdictCommented && (ch.Reviewers[i].State == forge.Approved || ch.Reviewers[i].State == forge.RequestedChanges) {
		return
	}
	ch.Reviewers[i].State = state
}

func repoPath(ref forge.Ref) string {
	owner, name := ownerName(ref)
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}

// mergeBase is where the pull request's branch left its target, which the
// patches are taken against.
func (c *Client) mergeBase(ctx context.Context, ref forge.Ref, base, head string) (string, error) {
	b, err := c.get(ctx, repoPath(ref)+"/compare/"+url.PathEscape(base)+"..."+url.PathEscape(head), url.Values{"per_page": {"1"}})
	if err != nil {
		return "", err
	}
	var out struct {
		MergeBase struct {
			SHA string `json:"sha"`
		} `json:"merge_base_commit"`
	}
	if json.Unmarshal(b, &out) != nil || out.MergeBase.SHA == "" {
		return "", errors.New("github: the comparison names no merge base")
	}
	return out.MergeBase.SHA, nil
}

// unmoved refuses a pull request that moved since p was read.
func (c *Client) unmoved(ctx context.Context, ref forge.Ref, p *gqlPR) error {
	owner, name := ownerName(ref)
	var out struct {
		Repository struct {
			PullRequest struct {
				UpdatedAt  time.Time `json:"updatedAt"`
				HeadRefOID string    `json:"headRefOid"`
				BaseRefOID string    `json:"baseRefOid"`
				State      string    `json:"state"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	if err := c.query(ctx, checkQuery, map[string]any{"owner": owner, "name": name, "number": ref.Number}, &out); err != nil {
		return err
	}
	now := out.Repository.PullRequest
	if !now.UpdatedAt.Equal(p.UpdatedAt) || now.HeadRefOID != p.HeadRefOID || now.BaseRefOID != p.BaseRefOID || now.State != p.State {
		return errMoved
	}
	return nil
}
