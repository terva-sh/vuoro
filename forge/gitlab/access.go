package gitlab

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/terva-sh/vuoro/forge"
)

// Access says how far the token reaches (D57): a personal access token
// describes itself, and the account says whether it is an administrator.
// Anything else, such as an OAuth token, is not described.
func (c *Client) Access(ctx context.Context) (forge.Access, error) {
	b, _, err := c.do(ctx, "GET", "/api/v4/personal_access_tokens/self", nil, nil)
	if err != nil {
		return forge.Access{Level: forge.AccessUnknown, Note: "GitLab did not describe this token, so vuoro cannot tell what it may do. It needs the api scope to post reviews, or read_api to read them."}, nil
	}
	var self struct {
		Scopes []string `json:"scopes"`
	}
	if json.Unmarshal(b, &self) != nil {
		return forge.Access{Level: forge.AccessUnknown}, nil
	}
	// is_admin is present only for an administrator. An api token's place
	// depends on it, so a lookup that fails leaves that token unknown rather
	// than taking the account for an ordinary one.
	var me struct {
		IsAdmin bool `json:"is_admin"`
	}
	b, _, err = c.do(ctx, "GET", "/api/v4/user", nil, nil)
	if err == nil {
		err = json.Unmarshal(b, &me)
	}
	if err != nil && slices.Contains(self.Scopes, "api") {
		return forge.Access{Level: forge.AccessUnknown, Scopes: self.Scopes, Note: "GitLab did not say whether this account is an administrator, whose api token would reach further than vuoro needs."}, nil
	}
	return scopeAccess(self.Scopes, me.IsAdmin), nil
}

// scopeAccess places a personal access token's scopes on the gauge. An
// administrator's api token acts as the administrator unless the instance
// asks for Admin Mode, which vuoro cannot see, so it counts as broad.
func scopeAccess(scopes []string, admin bool) forge.Access {
	a := forge.Access{Scopes: scopes}
	for _, s := range scopes {
		switch s {
		case "sudo", "admin_mode", "create_runner", "manage_runner", "k8s_proxy", "write_registry", "write_virtual_registry":
			a.Broad = append(a.Broad, s)
		}
	}
	api := slices.Contains(scopes, "api")
	if api && admin {
		a.Broad = append(a.Broad, "api, on an administrator's account")
	}
	// A token that cannot read reviews is placed there whatever else it
	// holds, so it is never saved.
	switch {
	case !api && !slices.Contains(scopes, "read_api"):
		a.Level = forge.AccessNone
		a.Missing = []string{"read_api", "api"}
	case len(a.Broad) > 0:
		a.Level = forge.AccessBroad
	case api:
		a.Level = forge.AccessFull
	default:
		a.Level = forge.AccessRead
		a.Missing = []string{"api"}
	}
	return a
}
