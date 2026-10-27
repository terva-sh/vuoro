package forge

import (
	"context"
	"errors"
	"strings"
)

// Project is one repository on a forge, which project triage reads every open
// change of (docs/plan.md section 6.9).
type Project struct {
	Host string `json:"host"`
	// Path is the project's full path, such as group/sub/project.
	Path   string `json:"path"`
	Name   string `json:"name"`
	WebURL string `json:"web_url"`
}

// ProjectLister is optional beside Provider. A client that needs an account
// returns an error rather than reading anonymously.
type ProjectLister interface {
	// SearchProjects finds projects the account is a member of whose name,
	// path, or group matches query.
	SearchProjects(ctx context.Context, query string) ([]Project, error)
	// Project reads one project by its full path, which confirms that the
	// account can see it.
	Project(ctx context.Context, path string) (Project, error)
	// ListOpen returns the project's complete open membership with update
	// stamps. Adapters own pagination and return an error rather than part
	// of the list.
	ListOpen(ctx context.Context, path string) ([]Discovered, error)
}

// DependencyReader optionally reads the changes that must merge before ref
// can. ErrUnsupported means the forge cannot say for this project, such as a
// GitLab tier without dependencies, which is unknown and never "none".
type DependencyReader interface {
	Dependencies(ctx context.Context, ref Ref) ([]Ref, error)
}

// ErrNotAProject is a URL that names no project.
var ErrNotAProject = errors.New("not a project URL")

// ProjectURL reads a project's browser URL as a host and a full path. parts
// is the required number of path segments, or zero for a forge that allows
// nested groups. A URL deeper in the project, such as a file or a change,
// keeps only the project: GitLab marks the rest with /-/, and a forge with
// fixed parts stops after them.
func ProjectURL(raw string, parts int) (Project, error) {
	u, err := ChangeURL(raw)
	if err != nil {
		return Project{}, ErrNotAProject
	}
	path := strings.Trim(u.Path, "/")
	path, _, _ = strings.Cut(path, "/-/")
	path = strings.TrimSuffix(path, ".git")
	ps := strings.Split(path, "/")
	if parts > 0 {
		if len(ps) < parts {
			return Project{}, ErrNotAProject
		}
		ps = ps[:parts]
	} else if len(ps) < 2 {
		return Project{}, ErrNotAProject
	}
	for _, p := range ps {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, ":\\ \t\r\n?#") {
			return Project{}, ErrNotAProject
		}
	}
	return Project{Host: u.Host, Path: strings.Join(ps, "/")}, nil
}
