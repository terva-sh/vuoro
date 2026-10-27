// Package forgejo reads Forgejo pull requests into the neutral forge model,
// and writes the person's reviews to them (write.go).
package forgejo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

const maxBody = 32 << 20

var _ forge.Provider = (*Client)(nil)

type Client struct {
	base  *url.URL
	token config.Secret
	http  *http.Client

	// drafts keeps the person's pending comments in vuoro (D43); nil until
	// the application gives the client a store.
	draftMu sync.Mutex
	drafts  forge.DraftStore

	meMu  sync.Mutex
	login string
}

func New(base string, token config.Secret) (*Client, error) {
	u, err := url.Parse(strings.TrimSuffix(base, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("forgejo: invalid base URL")
	}
	u.Host = strings.ToLower(u.Host)
	return &Client{base: u, token: token, http: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("forgejo: redirects are refused") }}}, nil
}
func ParseURL(raw string) (forge.Ref, error) { return forge.URLRef(raw, "/pulls/", 2) }
func (c *Client) scrub(s string) string {
	if t := c.token.Reveal(); t != "" {
		s = strings.ReplaceAll(s, t, "[redacted]")
	}
	return s
}
func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, q, nil)
}

// do sends one API request. A body is sent as JSON. Errors never carry the
// token, and a redirect is refused rather than followed with it.
func (c *Client) do(ctx context.Context, method, path string, q url.Values, body any) ([]byte, error) {
	target := c.base.String() + "/api/v1" + path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if !c.token.Empty() {
		req.Header.Set("Authorization", "token "+c.token.Reveal())
	}
	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("forgejo: %s %s: %s", method, path, c.scrub(err.Error()))
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("forgejo: reading response: %s", c.scrub(err.Error()))
	}
	if len(b) > maxBody {
		return nil, errors.New("forgejo: response exceeds 32 MiB")
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		var m struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &m)
		msg := c.scrub(m.Message)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, &forge.APIError{Forge: forge.Forgejo, Method: method, Path: path, Status: res.StatusCode, Message: msg}
	}
	return b, nil
}
func read[T any](ctx context.Context, c *Client, path string) (T, error) {
	var out T
	b, err := c.get(ctx, path, nil)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(b, &out); err != nil {
		return out, fmt.Errorf("forgejo: invalid response for %s", path)
	}
	return out, nil
}

// reviewComments is every comment of a review. The endpoint returns the
// complete list and has no page/limit API: Forgejo 15 ignores both
// parameters (verified live), so paging would repeat the same comments until
// pages gave up.
func reviewComments(ctx context.Context, c *Client, prPath string, review int64) ([]apiComment, error) {
	return read[[]apiComment](ctx, c, fmt.Sprintf("%s/reviews/%d/comments", prPath, review))
}
func pages[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var out []T
	// Follow until an empty page rather than guessing the server's maximum limit
	// or trusting an absolute Link URL with the person's token.
	path, query, _ := strings.Cut(path, "?")
	q, err := url.ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("forgejo: invalid query for %s", path)
	}
	for page := 1; page <= 1000; page++ {
		q.Set("page", strconv.Itoa(page))
		q.Set("limit", "100")
		b, err := c.get(ctx, path, q)
		if err != nil {
			return nil, err
		}
		var got []T
		// Forgejo serializes an empty timeline as null (verified live). Other
		// paginated endpoints must return arrays, including when empty.
		if json.Unmarshal(b, &got) != nil || (got == nil && !strings.HasSuffix(path, "/timeline")) {
			return nil, fmt.Errorf("forgejo: invalid page for %s", path)
		}
		if len(got) == 0 {
			return out, nil
		}
		out = append(out, got...)
	}
	return nil, errors.New("forgejo: pagination exceeded 1000 pages")
}

type apiUser struct {
	Login string `json:"login"`
}

func user(u apiUser) forge.User { return forge.User{Username: u.Login} }

type apiRepo struct {
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	SSHURL   string `json:"ssh_url"`
}
type apiBranch struct {
	Ref  string   `json:"ref"`
	SHA  string   `json:"sha"`
	Repo *apiRepo `json:"repo"`
}
type apiPR struct {
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	User      apiUser   `json:"user"`
	State     string    `json:"state"`
	Draft     bool      `json:"draft"`
	Merged    bool      `json:"merged"`
	Mergeable bool      `json:"mergeable"`
	Base      apiBranch `json:"base"`
	Head      apiBranch `json:"head"`
	MergeBase string    `json:"merge_base"`
	Assignees []apiUser `json:"assignees"`
	Requested []apiUser `json:"requested_reviewers"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Created time.Time `json:"created_at"`
	Updated time.Time `json:"updated_at"`
}
type apiReview struct {
	ID        int64     `json:"id"`
	User      apiUser   `json:"user"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Commit    string    `json:"commit_id"`
	Dismissed bool      `json:"dismissed"`
	Stale     bool      `json:"stale"`
	Submitted time.Time `json:"submitted_at"`
	Comments  int       `json:"comments_count"`
}
type apiComment struct {
	ID             int64     `json:"id"`
	ReviewID       int64     `json:"pull_request_review_id"`
	User           apiUser   `json:"user"`
	Body           string    `json:"body"`
	Path           string    `json:"path"`
	Position       int       `json:"position"`
	OldPosition    int       `json:"original_position"`
	Commit         string    `json:"commit_id"`
	OriginalCommit string    `json:"original_commit_id"`
	Resolver       *apiUser  `json:"resolver"`
	Created        time.Time `json:"created_at"`
}
type apiEvent struct {
	ID              int64     `json:"id"`
	Type            string    `json:"type"`
	Body            string    `json:"body"`
	User            apiUser   `json:"user"`
	Assignee        *apiUser  `json:"assignee"`
	RemovedAssignee bool      `json:"removed_assignee"`
	Created         time.Time `json:"created_at"`
}

func repoPath(ref forge.Ref) string {
	parts := strings.Split(ref.Project, "/")
	return "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
}
func (c *Client) Open(ctx context.Context, ref forge.Ref) (*forge.Change, error) {
	return c.readChange(ctx, ref, true)
}

// ReadFacts reads current review facts without patches or private drafts.
func (c *Client) ReadFacts(ctx context.Context, ref forge.Ref) (*forge.Change, error) {
	return c.readChange(ctx, ref, false)
}

func (c *Client) readChange(ctx context.Context, ref forge.Ref, workspace bool) (*forge.Change, error) {
	if !strings.EqualFold(ref.Host, c.base.Host) || len(strings.Split(ref.Project, "/")) != 2 || ref.Number < 1 {
		return nil, errors.New("forgejo: change does not belong to this client")
	}
	root := repoPath(ref)
	prPath := fmt.Sprintf("%s/pulls/%d", root, ref.Number)
	p, err := read[apiPR](ctx, c, prPath)
	if err != nil {
		return nil, err
	}
	if p.Base.Repo == nil || p.Base.Repo.FullName != ref.Project {
		return nil, errors.New("forgejo: target repository metadata is missing or mismatched")
	}
	ch := &forge.Change{Kind: forge.Forgejo, Ref: ref, Title: p.Title, Description: p.Body, Author: user(p.User), WebURL: c.base.String() + "/" + ref.Project + "/pulls/" + strconv.Itoa(ref.Number),
		SourceBranch: p.Head.Ref, TargetBranch: p.Base.Ref, Diff: forge.DiffRefs{Base: p.MergeBase, Start: p.Base.SHA, Head: p.Head.SHA}, Draft: p.Draft, State: forge.State(p.State), Created: p.Created, Updated: p.Updated,
		Checkout: forge.Checkout{HTTPS: p.Base.Repo.CloneURL, SSH: p.Base.Repo.SSHURL, HeadRef: fmt.Sprintf("refs/pull/%d/head", ref.Number)}}
	if ch.State == "" {
		ch.State = forge.Open
	}
	if p.Merged {
		ch.State = forge.Merged
	}
	if p.Mergeable {
		ch.Mergeability = "mergeable"
	}
	if err := forge.ValidateCheckout(ch.Checkout, ref.Host); err != nil {
		return nil, fmt.Errorf("forgejo: %w", err)
	}
	for _, u := range p.Assignees {
		ch.Assignees = append(ch.Assignees, user(u))
	}
	for _, u := range p.Requested {
		ch.Reviewers = append(ch.Reviewers, forge.Reviewer{User: user(u), State: forge.Unreviewed})
	}
	// Project triage's facts (docs/plan.md section 6.9). Forgejo's
	// mergeable is false for a draft or while it checks as well as for a
	// conflict, so conflicts stay unknown.
	for _, l := range p.Labels {
		ch.Labels = append(ch.Labels, l.Name)
	}
	if p.Head.Repo != nil {
		ch.SourceProject = p.Head.Repo.FullName
	}
	if ch.Pipeline, err = c.pipeline(ctx, root, p.Head.SHA); err != nil {
		return nil, err
	}
	fs, err := pages[apiFile](ctx, c, prPath+"/files")
	if err != nil {
		return nil, err
	}
	for _, f := range fs {
		t := forge.Touched{Path: f.Filename}
		if f.Additions != nil {
			t.Added = *f.Additions
		}
		if f.Deletions != nil {
			t.Removed = *f.Deletions
		}
		ch.Touched = append(ch.Touched, t)
	}
	if workspace {
		diff, err := c.get(ctx, prPath+".diff", nil)
		if err != nil {
			return nil, err
		}
		ch.Files = fileDiffs(fs, string(diff))
	}
	reviews, err := pages[apiReview](ctx, c, prPath+"/reviews")
	if err != nil {
		return nil, err
	}
	published := map[string][]apiReview{}
	type relationshipEvent struct {
		username  string
		requested bool
		at        time.Time
	}
	var relationshipEvents []relationshipEvent
	for _, r := range reviews {
		pending := r.State == "PENDING"
		if pending && !workspace {
			continue
		}
		if r.State == "REQUEST_REVIEW" && !r.Dismissed {
			ch.Requests = append(ch.Requests, forge.ReviewRequest{Reviewer: r.User.Login, At: r.Submitted})
			relationshipEvents = append(relationshipEvents, relationshipEvent{r.User.Login, true, r.Submitted})
		}
		if r.State == "APPROVED" || r.State == "REQUEST_CHANGES" || r.State == "COMMENT" {
			published[r.User.Login] = append(published[r.User.Login], r)
		}
		if r.Body != "" {
			if pending {
				ch.Drafts = append(ch.Drafts, forge.Draft{ID: reviewPrefix + fmt.Sprint(r.ID), Body: r.Body})
			} else if r.State != "REQUEST_REVIEW" {
				ch.Threads = append(ch.Threads, forge.Thread{ID: fmt.Sprintf("review-%d", r.ID), Notes: []forge.Note{{ID: fmt.Sprint(r.ID), Author: user(r.User), Body: r.Body, At: r.Submitted}}})
			}
		}
		if r.Comments == 0 {
			continue
		}
		cs, err := reviewComments(ctx, c, prPath, r.ID)
		if err != nil {
			return nil, err
		}
		if len(cs) != r.Comments {
			return nil, errors.New("forgejo: review comments changed while reading; refresh again")
		}
		addComments(ch, cs, pending)
	}
	// Pending drafts and request records have no verdict timestamp. They must
	// not prevent a person's dated published reviews from being chronological.
	names := make([]string, 0, len(published))
	for name := range published {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		history := published[name]
		if !slices.ContainsFunc(history, func(r apiReview) bool { return r.Submitted.IsZero() }) {
			slices.SortStableFunc(history, func(a, b apiReview) int { return a.Submitted.Compare(b.Submitted) })
		}
		for _, r := range history {
			addReview(ch, r)
		}
	}
	timeline, err := pages[apiEvent](ctx, c, fmt.Sprintf("%s/issues/%d/timeline", root, ref.Number))
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(timeline, func(e apiEvent) bool { return e.Created.IsZero() }) {
		slices.SortStableFunc(timeline, func(a, b apiEvent) int { return a.Created.Compare(b.Created) })
	}
	for _, e := range timeline {
		switch e.Type {
		case "comment":
			ch.Threads = append(ch.Threads, forge.Thread{ID: fmt.Sprintf("comment-%d", e.ID), Notes: []forge.Note{{ID: fmt.Sprint(e.ID), Author: user(e.User), Body: e.Body, At: e.Created}}})
		case "review_request":
			if e.Assignee != nil {
				relationshipEvents = append(relationshipEvents, relationshipEvent{e.Assignee.Login, !e.RemovedAssignee, e.Created})
				if !e.RemovedAssignee {
					ch.Requests = append(ch.Requests, forge.ReviewRequest{Reviewer: e.Assignee.Login, By: e.User.Login, At: e.Created})
				}
			}
		case "pull_push":
			var push struct {
				Commits []string `json:"commit_ids"`
			}
			_ = json.Unmarshal([]byte(e.Body), &push)
			head := ""
			if len(push.Commits) > 0 {
				head = push.Commits[len(push.Commits)-1]
			}
			ch.Pushes = append(ch.Pushes, forge.Push{At: e.Created, Head: head, By: e.User.Login})
		}
	}
	// A historical voluntary review is a verdict, not a requested relationship.
	// Current PR requests win; otherwise the latest timeline request/removal wins.
	relationships := map[string]bool{}
	byPerson := map[string][]relationshipEvent{}
	for _, e := range relationshipEvents {
		byPerson[e.username] = append(byPerson[e.username], e)
	}
	for username, events := range byPerson {
		// An undated request/removal cannot be placed across the two sources.
		// Retain an uncertain relationship instead of silently dropping work;
		// its unanswered age stays unknown in the neutral request history.
		if slices.ContainsFunc(events, func(e relationshipEvent) bool { return e.at.IsZero() }) {
			for _, e := range events {
				relationships[username] = relationships[username] || e.requested
			}
		} else {
			slices.SortStableFunc(events, func(a, b relationshipEvent) int { return a.at.Compare(b.at) })
			relationships[username] = events[len(events)-1].requested
		}
	}
	for _, u := range p.Requested {
		relationships[u.Login] = true
	}
	ch.Reviewers = slices.DeleteFunc(ch.Reviewers, func(r forge.Reviewer) bool { return !relationships[r.Username] })
	for _, req := range ch.Requests {
		if relationships[req.Reviewer] && !slices.ContainsFunc(ch.Reviewers, func(r forge.Reviewer) bool { return r.Username == req.Reviewer }) {
			ch.Reviewers = append(ch.Reviewers, forge.Reviewer{User: forge.User{Username: req.Reviewer}, State: forge.Unreviewed})
		}
	}
	reconcileRequests(ch, p.Requested)
	// The diff endpoint reads the current PR, not a requested commit. Refuse a
	// mixed snapshot if a push or target update raced these paginated reads.
	current, err := read[apiPR](ctx, c, prPath)
	if err != nil {
		return nil, err
	}
	if current.Head.SHA != p.Head.SHA || current.Base.SHA != p.Base.SHA || current.MergeBase != p.MergeBase || !current.Updated.Equal(p.Updated) || current.State != p.State || current.Merged != p.Merged {
		return nil, errors.New("forgejo: pull request moved while reading; refresh it and try again")
	}
	slices.SortFunc(ch.Threads, func(a, b forge.Thread) int { return a.Notes[0].At.Compare(b.Notes[0].At) })
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
	// The person's own pending comments, kept in vuoro until they submit,
	// follow any pending review they started in Forgejo's web UI.
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
	ch.Capabilities = c.Capabilities()
	return ch, nil
}

// statusStates maps a commit's combined status to the pipeline states
// project triage reads.
var statusStates = map[string]string{"success": "success", "failure": "failed", "error": "failed", "pending": "pending", "warning": "warning"}

// pipeline is the head commit's combined status, or empty when it has none.
func (c *Client) pipeline(ctx context.Context, root, sha string) (string, error) {
	if sha == "" {
		return "", nil
	}
	var status struct {
		State string `json:"state"`
		Total int    `json:"total_count"`
	}
	b, err := c.get(ctx, root+"/commits/"+url.PathEscape(sha)+"/status", nil)
	if errors.Is(err, forge.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if json.Unmarshal(b, &status) != nil {
		return "", errors.New("forgejo: invalid commit status")
	}
	if status.Total == 0 {
		return "", nil
	}
	return statusStates[status.State], nil
}

// reconcileRequests applies the PR's requested_reviewers to reviewer state.
// Forgejo lists there both a reviewer with a pending request and one whose
// latest review approved or requested changes (verified live on 15.0.3); a
// request, unlike a verdict, leaves a REQUEST_REVIEW record or a timeline
// event. So a listed reviewer with no request on record, whose verdict on
// today's head stands, has answered.
func reconcileRequests(ch *forge.Change, requested []apiUser) {
	for _, u := range requested {
		var latest time.Time
		recorded := false
		for _, request := range ch.Requests {
			if request.Reviewer == u.Login {
				recorded = true
				if request.At.After(latest) {
					latest = request.At
				}
			}
		}
		if !recorded && standingVerdict(ch, u.Login) {
			continue
		}
		answered := false
		for _, review := range ch.Reviews {
			if !latest.IsZero() && review.Reviewer == u.Login && !review.At.Before(latest) &&
				!review.Dismissed && !review.Stale && review.SHA == ch.Diff.Head {
				answered = true
			}
		}
		if !answered {
			// A same-head re-request takes precedence over an earlier verdict.
			// Without a timestamp, retain the PR's explicit pending request.
			i := slices.IndexFunc(ch.Reviewers, func(r forge.Reviewer) bool { return r.Username == u.Login })
			ch.Reviewers[i].State = forge.Unreviewed
		}
	}
}

// standingVerdict says whether login's newest non-comment review approves or
// requests changes on today's head, undismissed.
func standingVerdict(ch *forge.Change, login string) bool {
	var last *forge.Review
	for i := range ch.Reviews {
		r := &ch.Reviews[i]
		if r.Reviewer == login && r.Verdict != forge.VerdictCommented && (last == nil || !r.At.Before(last.At)) {
			last = r
		}
	}
	return last != nil && !last.Dismissed && !last.Stale && last.SHA == ch.Diff.Head
}

func addReview(ch *forge.Change, r apiReview) {
	verdict := map[string]forge.Verdict{"APPROVED": forge.VerdictApproved, "REQUEST_CHANGES": forge.VerdictChangesRequested, "COMMENT": forge.VerdictCommented}[r.State]
	if verdict == "" {
		return
	}
	ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: r.User.Login, Verdict: verdict, At: r.Submitted, SHA: r.Commit, Dismissed: r.Dismissed, Stale: r.Stale})

	i := slices.IndexFunc(ch.Reviewers, func(u forge.Reviewer) bool { return u.Username == r.User.Login })
	if i < 0 {
		i = len(ch.Reviewers)
		ch.Reviewers = append(ch.Reviewers, forge.Reviewer{User: user(r.User), State: forge.Reviewed})
	}
	if r.Dismissed || r.Stale || r.Commit != ch.Diff.Head {
		// A later excluded verdict supersedes older verdicts; comments do not.
		if verdict != forge.VerdictCommented {
			ch.Reviewers[i].State = forge.Reviewed
		}
		return
	}
	state := map[forge.Verdict]forge.ReviewState{forge.VerdictApproved: forge.Approved, forge.VerdictChangesRequested: forge.RequestedChanges, forge.VerdictCommented: forge.Reviewed}[verdict]
	if verdict == forge.VerdictCommented && (ch.Reviewers[i].State == forge.Approved || ch.Reviewers[i].State == forge.RequestedChanges) {
		return
	}
	ch.Reviewers[i].State = state

}
func addComments(ch *forge.Change, cs []apiComment, pending bool) {
	slices.SortFunc(cs, func(a, b apiComment) int { return a.Created.Compare(b.Created) })
	groups := map[string]int{}
	for _, n := range cs {
		a := &forge.Anchor{Path: n.Path, OldPath: n.Path, NewLine: n.Position, OldLine: n.OldPosition, Diff: forge.DiffRefs{Head: n.Commit, Base: n.OriginalCommit}}
		if pending {
			ch.Drafts = append(ch.Drafts, forge.Draft{ID: fmt.Sprint(n.ID), Body: n.Body, Anchor: a})
			continue
		}
		key := fmt.Sprintf("%d:%s:%d:%d", n.ReviewID, n.Path, n.Position, n.OldPosition)
		i, ok := groups[key]
		if !ok {
			i = len(ch.Threads)
			groups[key] = i
			ch.Threads = append(ch.Threads, forge.Thread{ID: fmt.Sprint(n.ID), Anchor: a, Resolvable: true, Resolved: n.Resolver != nil && n.Resolver.Login != ""})
		}
		ch.Threads[i].Notes = append(ch.Threads[i].Notes, forge.Note{ID: fmt.Sprint(n.ID), Author: user(n.User), Body: n.Body, At: n.Created})
	}
}
