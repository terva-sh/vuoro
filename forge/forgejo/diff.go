package forgejo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

type apiFile struct {
	Filename  string `json:"filename"`
	Previous  string `json:"previous_filename"`
	Status    string `json:"status"`
	Additions *int   `json:"additions"`
	Deletions *int   `json:"deletions"`
}

// Forgejo's files endpoint has no patch field. Match its inventory against
// the unified diff, including quoted names, binary files and empty renames.
func fileDiffs(fs []apiFile, diff string) []forge.FileDiff {
	out := make([]forge.FileDiff, 0, len(fs))
	patches := make(map[string]string)
	headers := make(map[string]string)
	for _, f := range fs {
		old := f.Previous
		if old == "" {
			old = f.Filename
		}
		headers["diff --git a/"+old+" b/"+f.Filename] = f.Filename
	}
	seen := map[string]bool{}
	binary := map[string]bool{}
	name := ""
	var buf strings.Builder
	flush := func() {
		if name != "" {
			patches[name] = buf.String()
		}
		buf.Reset()
	}
	for _, line := range strings.SplitAfter(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			name = ""
			header := strings.TrimSuffix(line, "\n")
			name = headers[header]
			// Git quotes paths with spaces/control bytes/non-ASCII according to
			// core.quotePath. Unquote the second token without splitting its spaces.
			if name == "" {
				tokens := gitTokens(strings.TrimPrefix(header, "diff --git "))
				if len(tokens) == 2 {
					name = strings.TrimPrefix(tokens[1], "b/")
				}
			}
			seen[name] = name != ""
			continue
		}
		if strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch") {
			binary[name] = true
		}
		if strings.HasPrefix(line, "@@ ") || buf.Len() > 0 {
			buf.WriteString(line)
		}
	}
	flush()
	for _, f := range fs {
		old := f.Previous
		if old == "" {
			old = f.Filename
		}
		fd := forge.FileDiff{OldPath: old, NewPath: f.Filename, New: f.Status == "added", Deleted: f.Status == "deleted", Renamed: f.Status == "renamed", Patch: patches[f.Filename]}
		if fd.Patch != "" && !completeHunks(fd.Patch) {
			fd.Patch = ""
			fd.Omitted = true
		}
		adds, dels := 0, 0
		for _, line := range strings.Split(fd.Patch, "\n") {
			if strings.HasPrefix(line, "+") {
				adds++
			}
			if strings.HasPrefix(line, "-") {
				dels++
			}
		}
		if f.Additions != nil && adds != *f.Additions || f.Deletions != nil && dels != *f.Deletions {
			fd.Omitted = true
			fd.Patch = ""
		}
		if !seen[f.Filename] || binary[f.Filename] || fd.Patch == "" && !fd.Renamed && (f.Additions == nil || f.Deletions == nil || *f.Additions > 0 || *f.Deletions > 0) {
			fd.Omitted = true
		}
		out = append(out, fd)
	}
	return out
}
func gitTokens(s string) []string {
	var out []string
	for s != "" {
		s = strings.TrimLeft(s, " ")
		if s == "" {
			break
		}
		if s[0] == '"' {
			q, err := strconv.QuotedPrefix(s)
			if err != nil {
				return nil
			}
			v, err := strconv.Unquote(q)
			if err != nil {
				return nil
			}
			out = append(out, v)
			s = s[len(q):]
		} else {
			v, rest, _ := strings.Cut(s, " ")
			out = append(out, v)
			s = rest
		}
	}
	return out
}
func completeHunks(p string) bool {
	old, new := 0, 0
	started := false
	for _, l := range strings.Split(strings.TrimSuffix(p, "\n"), "\n") {
		if strings.HasPrefix(l, "@@ ") {
			if started && (old != 0 || new != 0) {
				return false
			}
			var a, b string
			if _, err := fmt.Sscanf(l, "@@ -%s +%s @@", &a, &b); err != nil {
				return false
			}
			count := func(s string) int {
				_, n, has := strings.Cut(s, ",")
				if !has {
					return 1
				}
				v, err := strconv.Atoi(n)
				if err != nil {
					return -1
				}
				return v
			}
			old, new = count(a), count(b)
			if old < 0 || new < 0 {
				return false
			}
			started = true
			continue
		}
		if !started {
			return false
		}
		switch {
		case strings.HasPrefix(l, "+"):
			new--
		case strings.HasPrefix(l, "-"):
			old--
		case strings.HasPrefix(l, " "):
			old--
			new--
		case strings.HasPrefix(l, "\\"):
			continue
		default:
			return false
		}
		if old < 0 || new < 0 {
			return false
		}
	}
	return started && old == 0 && new == 0
}
