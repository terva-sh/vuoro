package forge

import "context"

// Access is how far a token reaches, as the forge tells it (D57). vuoro
// learns it only from what the forge reports and from reads; it never sends
// a write to see whether a write would succeed.
type Access struct {
	Level AccessLevel `json:"level"`
	// Scopes are what the forge says the token holds, when it says.
	Scopes []string `json:"scopes,omitempty"`
	// Broad are the scopes or roles beyond what vuoro needs, and Missing
	// what posting reviews would still need.
	Broad   []string `json:"broad,omitempty"`
	Missing []string `json:"missing,omitempty"`
	// Note is what the person should know that the level does not say, such
	// as a forge that cannot tell whether the token may post.
	Note string `json:"note,omitempty"`
}

// AccessLevel places a token on the setup window's gauge.
type AccessLevel string

const (
	// AccessUnknown is a token the forge does not describe.
	AccessUnknown AccessLevel = "unknown"
	// AccessNone cannot read changes and their reviews.
	AccessNone AccessLevel = "none"
	// AccessRead reads changes and reviews, so vuoro reviews them and the
	// person posts by hand.
	AccessRead AccessLevel = "read"
	// AccessFull also posts the reviews the person sends.
	AccessFull AccessLevel = "full"
	// AccessBroad reaches beyond reviewing, such as administration.
	AccessBroad AccessLevel = "broad"
)

// AccessReader optionally says how far the client's token reaches.
type AccessReader interface {
	Access(ctx context.Context) (Access, error)
}
