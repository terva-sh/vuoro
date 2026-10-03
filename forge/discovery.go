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
