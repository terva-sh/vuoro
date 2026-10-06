package forge

import (
	"context"
	"time"
)

// Role is the person's relationship to a change. A change can have several.
type Role string

const (
	AuthorRole      Role = "author"
	ReviewerRole    Role = "reviewer"
	AssigneeRole    Role = "assignee"
	ParticipantRole Role = "participant"
)

// Discovered is an open change's membership and its forge update stamp.
// A zero Updated means the adapter cannot safely support cached reads.
type Discovered struct {
	Ref     Ref       `json:"ref"`
	Roles   []Role    `json:"roles"`
	Updated time.Time `json:"updated"`
}

// Discoverer is optional alongside Provider. It returns the complete current
// open membership for username, including unchanged changes. Adapters own all
// pagination and return an error rather than a partial membership. The inbox
// reuses unchanged snapshots; removals are safe only with complete membership.
type Discoverer interface {
	Discover(ctx context.Context, username string) ([]Discovered, error)
}

// Account says whose token a client holds, so setup can confirm a token
// source names the person's own account before it is saved.
type Account interface {
	Me(ctx context.Context) (string, error)
}

// FactsReader optionally reads fresh review facts without patches or personal
// drafts. The inbox calls it every refresh, even when Updated is unchanged:
// merge readiness and the signed-in person's permissions can change separately.
type FactsReader interface {
	ReadFacts(ctx context.Context, ref Ref) (*Change, error)
}
