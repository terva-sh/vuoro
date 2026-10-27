package projects

import (
	"cmp"
	"slices"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

// Row is one open change as the project view shows it.
type Row struct {
	Ref          forge.Ref  `json:"ref"`
	Kind         string     `json:"kind"`
	Title        string     `json:"title"`
	WebURL       string     `json:"web_url"`
	Author       string     `json:"author"`
	Draft        bool       `json:"draft"`
	SourceBranch string     `json:"source_branch"`
	TargetBranch string     `json:"target_branch"`
	Opened       *time.Time `json:"opened"`
	Sitting
	Reviewers []RowReviewer `json:"reviewers"`
	Assignees []string      `json:"assignees"`
	Labels    []string      `json:"labels"`
	Pipeline  string        `json:"pipeline,omitempty"`
	Conflicts bool          `json:"conflicts,omitempty"`
	Threads   Threads       `json:"threads"`
	Stack     *Stack        `json:"stack,omitempty"`
	// Error says why the last refresh could not read this change, whose
	// facts are then the last ones read.
	Error string `json:"error,omitempty"`
}

// RowReviewer is a human reviewer and where they stand.
type RowReviewer struct {
	Username string            `json:"username"`
	State    forge.ReviewState `json:"state"`
}

// Threads counts a change's discussion by people: every thread with a real
// note, the unresolved ones, and of those whose turn each is.
type Threads struct {
	Total      int `json:"total"`
	Unresolved int `json:"unresolved"`
	// Author are unresolved threads whose last real note is not the
	// author's, and Reviewers those whose last real note is.
	Author    int `json:"author"`
	Reviewers int `json:"reviewers"`
}

// Group is the rows in one place, oldest first.
type Group struct {
	Place Place `json:"place"`
	Rows  []Row `json:"rows"`
}

// Display is a project's view: its refresh state and its rows by place.
type Display struct {
	Project     Entry     `json:"project"`
	LastAttempt time.Time `json:"last_attempt"`
	LastSuccess time.Time `json:"last_success"`
	Error       string    `json:"error,omitempty"`
	Warning     string    `json:"warning,omitempty"`
	// Dependencies says whether the forge answered for every change's
	// dependencies. Without it, stacks come from target branches alone.
	Dependencies bool `json:"dependencies"`
	// Open counts every open change in the snapshot.
	Open   int     `json:"open"`
	Groups []Group `json:"groups"`
	// Unread were listed but never read, so where they sit is unknown.
	Unread []Row `json:"unread"`
}

// Display derives the view of a classified snapshot.
func (snap Snapshot) Display() Display {
	out := Display{Project: snap.Project, LastAttempt: snap.LastAttempt, LastSuccess: snap.LastSuccess,
		Error: snap.Error, Warning: snap.Warning, Dependencies: snap.Dependencies, Open: len(snap.Items),
		Groups: []Group{}, Unread: []Row{}}
	var read []forge.Change
	for _, item := range snap.Items {
		if !item.Unread {
			read = append(read, item.Change)
		}
	}
	belows, stackOf := stacks(snap.Project.Path, read)
	byPlace := map[Place][]Row{}
	for _, item := range snap.Items {
		ch := item.Change
		row := Row{Ref: ch.Ref, Kind: ch.Kind, Title: ch.Title, WebURL: ch.WebURL, Author: ch.Author.Username,
			Draft: ch.Draft, SourceBranch: ch.SourceBranch, TargetBranch: ch.TargetBranch, Opened: stamp(ch.Created),
			Reviewers: []RowReviewer{}, Assignees: []string{}, Labels: slices.Clone(ch.Labels),
			Pipeline: ch.Pipeline, Conflicts: ch.Conflicts, Error: item.Error}
		if row.Labels == nil {
			row.Labels = []string{}
		}
		if item.Unread {
			row.Holder, row.Reasons = []string{}, []Reason{}
			out.Unread = append(out.Unread, row)
			continue
		}
		row.Sitting = Sit(ch, belows[ch.Ref.Number])
		row.Stack = stackOf[ch.Ref.Number]
		for _, r := range humanReviewers(ch) {
			row.Reviewers = append(row.Reviewers, RowReviewer{Username: r.Username, State: r.State})
		}
		for _, u := range ch.Assignees {
			if !u.Bot {
				row.Assignees = append(row.Assignees, u.Username)
			}
		}
		row.Threads = threads(ch)
		byPlace[row.Place] = append(byPlace[row.Place], row)
	}
	for _, p := range Places {
		rows := byPlace[p]
		if len(rows) == 0 {
			continue
		}
		slices.SortStableFunc(rows, compareRows)
		out.Groups = append(out.Groups, Group{Place: p, Rows: rows})
	}
	return out
}

// compareRows puts the longest known wait first, then unknown waits by when
// the change was opened, then by number.
func compareRows(a, b Row) int {
	switch {
	case a.Since != nil && b.Since == nil:
		return -1
	case a.Since == nil && b.Since != nil:
		return 1
	case a.Since != nil && b.Since != nil:
		if n := a.Since.Compare(*b.Since); n != 0 {
			return n
		}
	}
	switch {
	case a.Opened != nil && b.Opened == nil:
		return -1
	case a.Opened == nil && b.Opened != nil:
		return 1
	case a.Opened != nil && b.Opened != nil:
		if n := a.Opened.Compare(*b.Opened); n != 0 {
			return n
		}
	}
	return cmp.Compare(a.Ref.Number, b.Ref.Number)
}

func threads(ch forge.Change) Threads {
	var out Threads
	for _, t := range ch.Threads {
		var last *forge.Note
		for i := range t.Notes {
			if n := &t.Notes[i]; !n.System && !n.Author.Bot {
				last = n
			}
		}
		if last == nil {
			continue
		}
		out.Total++
		if !t.Resolvable || t.Resolved {
			continue
		}
		out.Unresolved++
		if last.Author.Username == ch.Author.Username {
			out.Reviewers++
		} else {
			out.Author++
		}
	}
	return out
}

func stamp(at time.Time) *time.Time {
	if at.IsZero() {
		return nil
	}
	return &at
}
