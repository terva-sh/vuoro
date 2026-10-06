package gitlab

import (
	"fmt"
	"github.com/terva-sh/vuoro/forge"
)

var _ forge.Provider = (*Client)(nil)

func ParseURL(raw string) (forge.Ref, error) { return forge.URLRef(raw, "/-/merge_requests/", 0) }
func (c *Client) Capabilities() forge.Capabilities {
	write := !c.token.Empty()
	return forge.Capabilities{Drafts: write, Reply: write, Resolve: write, Approve: write, Unapprove: write, RequestChanges: write, Suggestions: write}
}
func (c *Client) checkout(ref forge.Ref) forge.Checkout {
	return forge.Checkout{HTTPS: "https://" + ref.Host + "/" + ref.Project + ".git", SSH: "git@" + c.base.Hostname() + ":" + ref.Project + ".git", HeadRef: fmt.Sprintf("refs/merge-requests/%d/head", ref.Number)}
}
