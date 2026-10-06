package local

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Remote is one of a project's remotes, read as the forge project it names
// (docs/plan.md section 6.8, D49). Only the host and the path are kept: a
// remote's URL can carry a credential, and none is read past parsing.
type Remote struct {
	Name string `json:"name"`
	Host string `json:"host"` // lower case, without a port
	Path string `json:"path"` // such as group/sub/project, without .git
}

// scpLike is git's user@host:path form, which has no scheme.
var scpLike = regexp.MustCompile(`^(?:[^@/]+@)?([^:/]+):(.+)$`)

// ParseRemoteURL reads a remote's URL, in the SSH, scp-like, or HTTPS form,
// as a host and a project path. It answers false for anything else, such as
// a path on disk.
func ParseRemoteURL(raw string) (host, path string, ok bool) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", false
		}
		switch u.Scheme {
		case "https", "http", "ssh", "git+ssh", "ssh+git":
		default:
			return "", "", false
		}
		host, path = u.Hostname(), u.Path
	} else if m := scpLike.FindStringSubmatch(raw); m != nil && !strings.HasPrefix(raw, "/") {
		host, path = m[1], m[2]
	} else {
		return "", "", false
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	path = strings.Trim(path, "/")
	if host == "" || path == "" || strings.Contains(path, "..") {
		return "", "", false
	}
	return strings.ToLower(host), path, true
}

// readRemotes reads the project's remotes that name a forge project.
func (p *Project) readRemotes(ctx context.Context) {
	out, err := p.git.Run(ctx, "config", "--get-regexp", `^remote\..*\.url$`)
	if err != nil {
		return // a repository with no remote
	}
	for _, l := range lines(out) {
		key, raw, ok := strings.Cut(l, " ")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		if host, path, ok := ParseRemoteURL(raw); ok {
			p.Remotes = append(p.Remotes, Remote{Name: name, Host: host, Path: path})
		}
	}
	sort.SliceStable(p.Remotes, func(i, j int) bool { return p.Remotes[i].Name < p.Remotes[j].Name })
}

// Remote is the remote named name, or nil.
func (p *Project) Remote(name string) *Remote {
	for i := range p.Remotes {
		if p.Remotes[i].Name == name {
			return &p.Remotes[i]
		}
	}
	return nil
}

// Compare counts the commits in tip and not in other, and in other and not
// in tip. known is false when the checkout does not have other, as when a
// change has commits not fetched here.
func (p *Project) Compare(ctx context.Context, tip, other string) (ahead, behind int, known bool) {
	if !isHash(tip) || !isHash(other) {
		return 0, 0, false
	}
	out, err := p.git.Run(ctx, "rev-list", "--left-right", "--count", other+"..."+tip)
	if err != nil {
		return 0, 0, false
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0, false
	}
	behind, err1 := strconv.Atoi(f[0])
	ahead, err2 := strconv.Atoi(f[1])
	return ahead, behind, err1 == nil && err2 == nil
}

func isHash(s string) bool {
	if len(s) < 7 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}
