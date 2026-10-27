package forge

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	GitLab  = "gitlab"
	Forgejo = "forgejo"
	GitHub  = "github"
)

// Provider reads one host as one account. Writer is a separate, optional
// contract: adding a reader never implicitly enables posting.
type Provider interface {
	Opener
	Capabilities() Capabilities
}

// Capabilities are implemented actions available with this client's credentials.
// They do not promise permission on a particular change; the forge decides that
// when the person acts. The UI offers only these, and the application refuses
// the rest, so a forge that cannot do something never shows a control for it.
type Capabilities struct {
	// Drafts includes creating, editing and submitting pending comments.
	Drafts bool `json:"drafts"`
	// Reply is a pending comment that answers a thread.
	Reply   bool `json:"reply"`
	Resolve bool `json:"resolve"`
	Approve bool `json:"approve"`
	// Unapprove withdraws the person's own approval.
	Unapprove bool `json:"unapprove"`
	// RequestChanges submits a review that asks for changes.
	RequestChanges bool `json:"request_changes"`
	// Suggestions means the forge applies a suggestion block. Without it a
	// suggestion posts as a plain fenced replacement (D43).
	Suggestions bool `json:"suggestions"`
	// SpanSuggestions means a suggestion replaces the lines its comment
	// spans, written as a bare suggestion fence (GitHub). Without it a block
	// carries its own offsets from the comment's one line (GitLab).
	SpanSuggestions bool `json:"span_suggestions"`
	// Reviewers, Assignees, Labels and SetDraft are the administrative
	// writes of forge.Administrator (D65): adding and removing reviewers,
	// assignees and labels, and marking a change draft or ready.
	Reviewers bool `json:"reviewers"`
	Assignees bool `json:"assignees"`
	Labels    bool `json:"labels"`
	SetDraft  bool `json:"draft"`
}

// Checkout describes how to fetch this change from its target repository.
// A pull-request ref includes fork heads without needing the fork's credentials.
// URLs never contain a forge token. Git uses the person's own transport.
type Checkout struct {
	HTTPS   string
	SSH     string
	HeadRef string
}

// APIError is a scrubbed adapter error. Adapters remove their credentials
// before constructing it; application code can classify it without importing
// the adapter's API types.
type APIError struct {
	Forge, Method, Path string
	Status              int
	Message             string
}

func (e *APIError) Error() string {
	s := fmt.Sprintf("%s: %s %s: %d", e.Forge, e.Method, e.Path, e.Status)
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s
}
func (e *APIError) Is(target error) bool { return target == ErrNotFound && e.Status == 404 }

// ChangeURL validates a browser URL without quoting it in an error (it might
// contain credentials). Per-forge parsers decide which paths name changes.
func ChangeURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" {
		return nil, ErrNotAChange
	}
	u.Host = strings.ToLower(u.Host)
	return u, nil
}

// URLRef extracts a change following marker. parts is the required number of
// project segments, or zero for a forge that allows nested namespaces.
func URLRef(raw, marker string, parts int) (Ref, error) {
	u, err := ChangeURL(raw)
	if err != nil {
		return Ref{}, err
	}
	project, rest, ok := strings.Cut(strings.Trim(u.Path, "/"), marker)
	if !ok || project == "" {
		return Ref{}, ErrNotAChange
	}
	ps := strings.Split(project, "/")
	if parts > 0 && len(ps) != parts {
		return Ref{}, ErrNotAChange
	}
	for _, p := range ps {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, ":\\ \t\r\n?#") {
			return Ref{}, ErrNotAChange
		}
	}
	num, _, _ := strings.Cut(rest, "/")
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 {
		return Ref{}, ErrNotAChange
	}
	return Ref{Host: u.Host, Project: project, Number: n}, nil
}

// ValidateCheckout refuses URLs with embedded credentials and refs that could
// change the meaning of git fetch. SSH usernames and explicit ports are allowed.
func ValidateCheckout(c Checkout, host string) error {
	if !strings.HasPrefix(c.HeadRef, "refs/") || !ValidBranch(strings.TrimPrefix(c.HeadRef, "refs/")) {
		return fmt.Errorf("unsafe change ref")
	}
	u, err := url.Parse(c.HTTPS)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, host) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path == "" {
		return fmt.Errorf("unsafe HTTPS clone URL")
	}
	if c.SSH == "" {
		return nil
	}
	if !strings.Contains(c.SSH, "://") {
		userHost, path, ok := strings.Cut(c.SSH, ":")
		username, sshHost, hasUser := strings.Cut(userHost, "@")
		if !ok || !hasUser || username == "" || !strings.EqualFold(sshHost, u.Hostname()) || path == "" || strings.ContainsAny(c.SSH, " \t\r\n") {
			return fmt.Errorf("unsafe SSH clone URL")
		}
		return nil
	}
	s, err := url.Parse(c.SSH)
	if err != nil || s.Scheme != "ssh" || !strings.EqualFold(s.Hostname(), u.Hostname()) || s.User == nil || s.User.Username() == "" || s.RawQuery != "" || s.Fragment != "" || s.Path == "" {
		return fmt.Errorf("unsafe SSH clone URL")
	}
	if _, pw := s.User.Password(); pw {
		return fmt.Errorf("unsafe SSH clone URL")
	}
	return nil
}

func ValidBranch(s string) bool {
	return s != "" && !strings.ContainsAny(s, ": \t\r\n*?[\\^~") && !strings.HasPrefix(s, "-") && !strings.Contains(s, "..") && !strings.Contains(s, "@{") && !strings.HasSuffix(s, "/")
}
