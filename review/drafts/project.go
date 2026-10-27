package drafts

import (
	"slices"
	"strings"
)

// The project triage role (docs/plan.md section 6.9, D65): one session reads
// a project's open changes and proposes, for each, reviewers, labels, and
// what would unblock it. A proposal is a draft. The person accepts it, and
// then vuoro's code, never the agent, makes the writes.

const maxProposed = 10

func validateProjectTriage(p Proposal) error {
	switch {
	case p.Change < 1:
		return &RuleError{"change.number", "change", "the number of an open change supplied to this run"}
	case len(p.Reviewers) == 0 && len(p.Labels) == 0 && strings.TrimSpace(p.Unblock) == "":
		return &RuleError{"proposal.empty", "reviewers", "at least one of reviewers, labels, or unblock"}
	case len(p.Reviewers) > maxProposed || len(p.Labels) > maxProposed:
		return &RuleError{"proposal.size", "reviewers", "at most 10 reviewers and 10 labels"}
	case strings.TrimSpace(p.Reason) == "" || len(p.Reason) > maxReason:
		return &RuleError{"triage.reason", "reason", "a reason of 1 to 512 bytes"}
	case len(p.Unblock) > maxReason || strings.ContainsAny(p.Unblock, "\r\n"):
		return &RuleError{"unblock.length", "unblock", "one line of at most 512 bytes"}
	case p.Anchor != nil || p.Suggestion != nil || len(p.Sources) > 0 || p.Severity != "" || p.Title != "" ||
		p.Evidence != "" || p.Earlier != "" || p.ThreadID != "" || p.Category != "" || p.Description != "":
		return &RuleError{"proposal.shape", "arguments", "change, reviewers, labels, unblock and reason only"}
	}
	for _, names := range [][]string{p.Reviewers, p.Labels} {
		seen := map[string]bool{}
		for _, n := range names {
			if strings.TrimSpace(n) == "" || len(n) > maxTitle || seen[n] {
				return &RuleError{"proposal.name", "reviewers", "distinct names that are not blank"}
			}
			seen[n] = true
		}
	}
	return nil
}

func (d *Draft) applyProject(p Proposal) {
	d.Change, d.Reason, d.Unblock = p.Change, strings.TrimSpace(p.Reason), strings.TrimSpace(p.Unblock)
	d.Reviewers, d.Labels = slices.Clone(p.Reviewers), slices.Clone(p.Labels)
}

// AcceptProposal serializes the person's acceptance of a project triage
// proposal against the agent's revisions and another acceptance. accept is
// supplied only by the UI's API, which makes the writes; no MCP tool reaches
// this method.
func (st *Store) AcceptProposal(id string, accept func(Draft) error) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	if d == nil || d.Change == 0 || d.Status != Pending || d.Decision != "" {
		return &RuleError{"proposal.pending", "id", "a pending project triage proposal"}
	}
	if err := accept(*d); err != nil {
		return err
	}
	d.Decision = Kept
	d.Updated = st.now()
	return nil
}
