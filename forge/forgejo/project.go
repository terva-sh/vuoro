package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

var (
	_ forge.ProjectLister = (*Client)(nil)
	_ forge.Administrator = (*Client)(nil)
	_ forge.TriageReader  = (*Client)(nil)
)

// ParseProjectURL reads a Forgejo repository URL, owner and name.
func ParseProjectURL(raw string) (forge.Project, error) { return forge.ProjectURL(raw, 2) }

type apiRepoID struct {
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

func (c *Client) repoProject(r apiRepoID) (forge.Project, error) {
	got, err := ParseProjectURL(r.HTMLURL)
	if err != nil || got.Host != c.base.Host || !strings.EqualFold(got.Path, r.FullName) {
		return forge.Project{}, errors.New("forgejo: invalid repository identity")
	}
	return forge.Project{Host: c.base.Host, Path: r.FullName, Name: r.FullName, WebURL: r.HTMLURL}, nil
}

// SearchProjects finds repositories the account can see by name, most
// recently updated first. It returns one page: a search narrows.
func (c *Client) SearchProjects(ctx context.Context, query string) ([]forge.Project, error) {
	if c.token.Empty() {
		return nil, errNoToken
	}
	q := url.Values{"limit": {"20"}, "sort": {"updated"}, "order": {"desc"}}
	if query = strings.TrimSpace(query); query != "" {
		q.Set("q", query)
	}
	b, err := c.get(ctx, "/repos/search", q)
	if err != nil {
		return nil, err
	}
	var res struct {
		OK   bool        `json:"ok"`
		Data []apiRepoID `json:"data"`
	}
	if json.Unmarshal(b, &res) != nil || !res.OK || res.Data == nil {
		return nil, errors.New("forgejo: invalid repository search response")
	}
	out := make([]forge.Project, 0, len(res.Data))
	for _, r := range res.Data {
		p, err := c.repoProject(r)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// Project reads one repository by owner/name. The answer's path is
// Forgejo's, which corrects the case of one typed or pasted.
func (c *Client) Project(ctx context.Context, path string) (forge.Project, error) {
	if c.token.Empty() {
		return forge.Project{}, errNoToken
	}
	if parts := strings.Split(path, "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return forge.Project{}, forge.ErrNotAProject
	}
	r, err := read[apiRepoID](ctx, c, repoPath(forge.Ref{Project: path}))
	if err != nil {
		return forge.Project{}, err
	}
	return c.repoProject(r)
}

// ListOpen returns every open pull request in the repository, paged as the
// adapter pages everything: until an empty page.
func (c *Client) ListOpen(ctx context.Context, path string) ([]forge.Discovered, error) {
	if c.token.Empty() {
		return nil, errNoToken
	}
	rows, err := pages[struct {
		Number  int       `json:"number"`
		HTMLURL string    `json:"html_url"`
		State   string    `json:"state"`
		Updated time.Time `json:"updated_at"`
	}](ctx, c, repoPath(forge.Ref{Project: path})+"/pulls?state=open&sort=oldest")
	if err != nil {
		return nil, err
	}
	out := []forge.Discovered{}
	seen := map[int]bool{}
	for _, row := range rows {
		ref, err := ParseURL(row.HTMLURL)
		if err != nil || ref.Host != c.base.Host || !strings.EqualFold(ref.Project, path) || ref.Number != row.Number || row.State != "open" {
			return nil, errors.New("forgejo: invalid listed identity")
		}
		if seen[ref.Number] {
			// One that moved between pages could be listed twice and another
			// skipped, so the list is not complete.
			return nil, errors.New("forgejo: repository changed while listing; refresh again")
		}
		seen[ref.Number] = true
		out = append(out, forge.Discovered{Ref: ref, Updated: row.Updated})
	}
	return out, nil
}

// wipPrefix is how Forgejo marks a pull request a draft: a title prefix,
// which by default is WIP: or [WIP], in any case.
var wipPrefix = regexp.MustCompile(`(?i)^\s*(wip:|\[wip\])\s*`)

// Administer makes the person's administrative edits, in a fixed order, and
// names those already made when one fails. Reviewers and labels have
// endpoints that add and remove. Assignees and the title, which carries the
// draft state, are written whole, so each is read just before it is written
// and only the person's own edit is changed in it (D65).
func (c *Client) Administer(ctx context.Context, ref forge.Ref, ch forge.Changes) error {
	if c.token.Empty() {
		return forge.ErrUnsupported
	}
	if !strings.EqualFold(ref.Host, c.base.Host) || len(strings.Split(ref.Project, "/")) != 2 || ref.Number < 1 {
		return fmt.Errorf("forgejo: %s does not belong to this client", ref)
	}
	pull := prPath(ref)
	issue := fmt.Sprintf("%s/issues/%d", repoPath(ref), ref.Number)
	type step struct {
		what string
		run  func() error
	}
	var steps []step
	if len(ch.AddReviewers) > 0 {
		steps = append(steps, step{"added reviewers", func() error {
			_, err := c.write(ctx, http.MethodPost, pull+"/requested_reviewers", map[string]any{"reviewers": ch.AddReviewers})
			return err
		}})
	}
	if len(ch.RemoveReviewers) > 0 {
		steps = append(steps, step{"removed reviewers", func() error {
			_, err := c.write(ctx, http.MethodDelete, pull+"/requested_reviewers", map[string]any{"reviewers": ch.RemoveReviewers})
			return err
		}})
	}
	if len(ch.AddAssignees)+len(ch.RemoveAssignees) > 0 {
		steps = append(steps, step{"changed assignees", func() error { return c.setAssignees(ctx, pull, issue, ch.AddAssignees, ch.RemoveAssignees) }})
	}
	if len(ch.AddLabels)+len(ch.RemoveLabels) > 0 {
		steps = append(steps, step{"changed labels", func() error { return c.setLabels(ctx, ref, issue, ch.AddLabels, ch.RemoveLabels) }})
	}
	if ch.Draft != nil {
		draft := *ch.Draft
		steps = append(steps, step{"set the draft state", func() error { return c.setDraft(ctx, pull, draft) }})
	}
	var done []string
	for _, s := range steps {
		if err := s.run(); err != nil {
			if len(done) > 0 {
				return fmt.Errorf("%w (already %s)", err, strings.Join(done, ", "))
			}
			return err
		}
		done = append(done, s.what)
	}
	return nil
}

// setAssignees writes the assignees read just now, with the person's
// additions and removals, since Forgejo takes only the whole list.
func (c *Client) setAssignees(ctx context.Context, pull, issue string, add, remove []string) error {
	p, err := read[apiPR](ctx, c, pull)
	if err != nil {
		return err
	}
	names := []string{}
	for _, a := range p.Assignees {
		if !slices.ContainsFunc(remove, func(r string) bool { return strings.EqualFold(r, a.Login) }) {
			names = append(names, a.Login)
		}
	}
	for _, a := range add {
		if !slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(n, a) }) {
			names = append(names, a)
		}
	}
	_, err = c.write(ctx, http.MethodPatch, issue, map[string]any{"assignees": names})
	return err
}

type apiLabel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// setLabels adds and removes labels by ID, which every Forgejo takes. An
// added label is looked up among the repository's and its organization's; a
// removed one among those the pull request carries now.
func (c *Client) setLabels(ctx context.Context, ref forge.Ref, issue string, add, remove []string) error {
	if len(add) > 0 {
		known, err := c.labels(ctx, ref.Project)
		if err != nil {
			return err
		}
		ids := []int64{}
		for _, name := range add {
			i := slices.IndexFunc(known, func(l apiLabel) bool { return l.Name == name })
			if i < 0 {
				return fmt.Errorf("forgejo: %s has no label %q", ref.Project, name)
			}
			ids = append(ids, known[i].ID)
		}
		if _, err := c.write(ctx, http.MethodPost, issue+"/labels", map[string]any{"labels": ids}); err != nil {
			return err
		}
	}
	if len(remove) > 0 {
		current, err := read[[]apiLabel](ctx, c, issue+"/labels")
		if err != nil {
			return err
		}
		for _, name := range remove {
			i := slices.IndexFunc(current, func(l apiLabel) bool { return l.Name == name })
			if i < 0 {
				continue // already gone
			}
			if _, err := c.write(ctx, http.MethodDelete, issue+"/labels/"+strconv.FormatInt(current[i].ID, 10), nil); err != nil && !errors.Is(err, forge.ErrNotFound) {
				return err
			}
		}
	}
	return nil
}

// setDraft adds or strips the WIP prefix on the title read just now.
func (c *Client) setDraft(ctx context.Context, pull string, draft bool) error {
	p, err := read[apiPR](ctx, c, pull)
	if err != nil {
		return err
	}
	title := wipPrefix.ReplaceAllString(p.Title, "")
	if draft {
		title = "WIP: " + title
	}
	if title == p.Title {
		return nil
	}
	_, err = c.write(ctx, http.MethodPatch, pull, map[string]any{"title": title})
	return err
}

// labels are the labels the repository's pull requests can carry: its own,
// and its organization's when its owner is one.
func (c *Client) labels(ctx context.Context, project string) ([]apiLabel, error) {
	out, err := pages[apiLabel](ctx, c, repoPath(forge.Ref{Project: project})+"/labels")
	if err != nil {
		return nil, err
	}
	owner, _, _ := strings.Cut(project, "/")
	org, err := pages[apiLabel](ctx, c, "/orgs/"+url.PathEscape(owner)+"/labels")
	if err != nil && !errors.Is(err, forge.ErrNotFound) {
		return nil, err
	}
	return append(out, org...), nil
}

// Members lists the accounts that can be assigned to the repository's pull
// requests, matching query by login or name. Forgejo's endpoint takes no
// query, so the list is narrowed here.
func (c *Client) Members(ctx context.Context, project, query string) ([]forge.Member, error) {
	if c.token.Empty() {
		return nil, forge.ErrUnsupported
	}
	rows, err := read[[]struct {
		Login    string `json:"login"`
		FullName string `json:"full_name"`
	}](ctx, c, repoPath(forge.Ref{Project: project})+"/assignees")
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := []forge.Member{}
	for _, r := range rows {
		if query == "" || strings.Contains(strings.ToLower(r.Login), query) || strings.Contains(strings.ToLower(r.FullName), query) {
			out = append(out, forge.Member{Username: r.Login, Name: r.FullName})
		}
	}
	return out, nil
}

// Labels lists the names of the labels the repository's pull requests can
// carry.
func (c *Client) Labels(ctx context.Context, project string) ([]string, error) {
	if c.token.Empty() {
		return nil, forge.ErrUnsupported
	}
	all, err := c.labels(ctx, project)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, l := range all {
		if !slices.Contains(out, l.Name) {
			out = append(out, l.Name)
		}
	}
	return out, nil
}

// codeOwnersPaths are where Forgejo looks for CODEOWNERS, in its order.
var codeOwnersPaths = []string{"CODEOWNERS", "docs/CODEOWNERS", ".forgejo/CODEOWNERS", ".gitea/CODEOWNERS"}

// maxCodeOwners bounds the CODEOWNERS text given to an agent.
const maxCodeOwners = 64 << 10

// CodeOwners reads the first CODEOWNERS Forgejo would use at ref.
func (c *Client) CodeOwners(ctx context.Context, project, ref string) (string, error) {
	for _, p := range codeOwnersPaths {
		b, err := c.get(ctx, repoPath(forge.Ref{Project: project})+"/raw/"+p, url.Values{"ref": {ref}})
		if errors.Is(err, forge.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		if len(b) > maxCodeOwners {
			b = b[:maxCodeOwners]
		}
		return string(b), nil
	}
	return "", nil
}

// RecentAuthors reads up to thirty of the year's commits to path on ref, and
// returns their authors' logins, or names where Forgejo links no account.
func (c *Client) RecentAuthors(ctx context.Context, project, ref, path string) ([]string, error) {
	q := url.Values{"sha": {ref}, "path": {path}, "limit": {"30"}, "stat": {"false"}, "verification": {"false"}, "files": {"false"},
		"since": {time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339)}}
	b, err := c.get(ctx, repoPath(forge.Ref{Project: project})+"/commits", q)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Author *struct {
			Login string `json:"login"`
		} `json:"author"`
		Commit struct {
			Author struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commit"`
	}
	if json.Unmarshal(b, &rows) != nil || rows == nil {
		return nil, errors.New("forgejo: invalid commits response")
	}
	out := []string{}
	for _, r := range rows {
		who := r.Commit.Author.Name
		if r.Author != nil && r.Author.Login != "" {
			who = r.Author.Login
		}
		if who != "" && !slices.Contains(out, who) {
			out = append(out, who)
		}
	}
	return out, nil
}
