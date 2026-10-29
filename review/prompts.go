package review

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Prompt is a canned question to ask about a change (section 6.11, D68).
// vuoro ships some, and the person adds their own as Markdown files.
type Prompt struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Scope   string `json:"scope,omitempty"` // change or since_review; empty is change
	Tool    string `json:"tool,omitempty"`
	Model   string `json:"model,omitempty"`
	Text    string `json:"text"`
	Builtin bool   `json:"builtin,omitempty"`
}

// PromptError is a prompt file vuoro could not use, and why. The other files
// still load.
type PromptError struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// Explain is the shipped prompt a summary run is listed under.
const Explain = "explain"

// BuiltinPrompts are the prompts vuoro ships, in the order it offers them.
var BuiltinPrompts = []Prompt{
	{Name: Explain, Title: "Explain this change", Builtin: true, Text: `Explain this change to me. Say what behaviour it changes and why, which files
matter most and what each one does, and what its tests cover and leave out.`},
	{Name: "risks", Title: "What is risky here", Builtin: true, Text: `What in this change is most likely to go wrong? Name the risks in order of
what they would cost, with the code that shows each, and say what would
reduce it. Say so when you find nothing worth worrying about.`},
	{Name: "manual-test", Title: "How would I test this by hand", Builtin: true, Text: `How would I test this change by hand? Give the steps that exercise what it
changes, the edge cases worth trying, and what I should see at each step.`},
	{Name: "start-here", Title: "Where should I start reading", Builtin: true, Text: `I am about to review this change. Where should I start reading, and in what
order should I go through the files? Say what to look for in each.`},
}

var promptNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// LoadPrompts answers the person's prompts in dir, sorted by name, then the
// shipped ones their names do not shadow (as presets do, D61). A file that
// cannot be used is reported, never dropped in silence, and a missing dir
// means the person has none.
func LoadPrompts(dir string) ([]Prompt, []PromptError) {
	var theirs []Prompt
	var bad []PromptError
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		bad = append(bad, PromptError{File: dir, Reason: "vuoro could not read the prompts directory"})
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok || e.IsDir() {
			continue
		}
		if !promptNameRE.MatchString(name) {
			bad = append(bad, PromptError{File: e.Name(), Reason: "a prompt's file name is lowercase letters, digits, and dashes, then .md"})
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			bad = append(bad, PromptError{File: e.Name(), Reason: "vuoro could not read the file"})
			continue
		}
		p, err := parsePrompt(name, string(b))
		if err != nil {
			bad = append(bad, PromptError{File: e.Name(), Reason: err.Error()})
			continue
		}
		theirs = append(theirs, p)
	}
	slices.SortFunc(theirs, func(a, b Prompt) int { return strings.Compare(a.Name, b.Name) })
	out := theirs
	for _, p := range BuiltinPrompts {
		if !slices.ContainsFunc(theirs, func(t Prompt) bool { return t.Name == p.Name }) {
			out = append(out, p)
		}
	}
	return out, bad
}

// parsePrompt reads a prompt file: an optional front matter of key: value
// lines between --- lines, then the prompt.
func parsePrompt(name, text string) (Prompt, error) {
	p := Prompt{Name: name, Title: name}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		head, body, found := strings.Cut(rest, "\n---\n")
		if !found {
			if head, found = strings.CutSuffix(rest, "\n---"); !found {
				return p, errors.New("the front matter opens with --- and never closes")
			}
		}
		for i, line := range strings.Split(head, "\n") {
			if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				return p, fmt.Errorf("front matter line %d is not key: value", i+1)
			}
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			switch strings.TrimSpace(key) {
			case "title":
				p.Title = value
			case "scope":
				if value != "change" && value != "since_review" {
					return p, fmt.Errorf("scope is change or since_review, not %q", value)
				}
				p.Scope = value
			case "tool":
				p.Tool = value
			case "model":
				p.Model = value
			default:
				return p, fmt.Errorf("front matter has no field %q; it takes title, scope, tool, and model", strings.TrimSpace(key))
			}
		}
		text = body
	}
	p.Text = strings.TrimSpace(text)
	if p.Text == "" {
		return p, errors.New("the prompt is empty")
	}
	if p.Title == "" {
		p.Title = name
	}
	return p, nil
}

// FindPrompt is the prompt named name in list.
func FindPrompt(list []Prompt, name string) (Prompt, bool) {
	i := slices.IndexFunc(list, func(p Prompt) bool { return p.Name == name })
	if i < 0 {
		return Prompt{}, false
	}
	return list[i], true
}
