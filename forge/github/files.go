package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/terva-sh/vuoro/forge"
)

type apiFile struct {
	Filename string `json:"filename"`
	Previous string `json:"previous_filename"`
	Status   string `json:"status"`
	Patch    string `json:"patch"`
	Changes  int    `json:"changes"`
}

// maxFiles is as many files as GitHub lists for a pull request.
const maxFiles = 3000

// files reads every changed file with its patch; want is how many the pull
// request changes. GitHub leaves the patch out of a binary file and of one
// too large to send, so those are listed as omitted; a rename that changes
// nothing has no patch either, and is not. A pull request changing more
// files than GitHub lists is refused rather than shown in part.
func (c *Client) files(ctx context.Context, ref forge.Ref, want int) ([]forge.FileDiff, error) {
	if want > maxFiles {
		return nil, errors.New("github: the pull request changes more files than GitHub lists")
	}
	path := fmt.Sprintf("%s/pulls/%d/files", repoPath(ref), ref.Number)
	var out []forge.FileDiff
	for page := 1; ; page++ {
		b, err := c.get(ctx, path, url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}})
		if err != nil {
			return nil, err
		}
		var got []apiFile
		if json.Unmarshal(b, &got) != nil || got == nil {
			return nil, fmt.Errorf("github: invalid page for %s", path)
		}
		for _, f := range got {
			old := f.Previous
			if old == "" {
				old = f.Filename
			}
			out = append(out, forge.FileDiff{OldPath: old, NewPath: f.Filename, New: f.Status == "added", Deleted: f.Status == "removed",
				Renamed: f.Status == "renamed", Patch: f.Patch, Omitted: f.Patch == "" && (f.Status != "renamed" || f.Changes > 0)})
		}
		if len(got) < 100 || len(out) >= want {
			if len(out) != want {
				return nil, errMoved
			}
			return out, nil
		}
	}
}
