package drafts

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// rpc posts one JSON-RPC request as the session holding token, and returns
// the HTTP status and the decoded response.
func rpc(t *testing.T, url, token, method string, params any) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// callTool calls a tool and returns its text and whether it was an error.
func callTool(t *testing.T, url, token, name string, args any) (string, bool) {
	t.Helper()
	code, out := rpc(t, url, token, "tools/call", map[string]any{"name": name, "arguments": args})
	if code != http.StatusOK {
		t.Fatalf("tools/call %s: HTTP %d", name, code)
	}
	res := out["result"].(map[string]any)
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	return text, res["isError"] == true
}

func setup(t *testing.T) (*Store, *Server, string) {
	t.Helper()
	st := NewStore()
	srv := NewServer()
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	return st, srv, hs.URL + "/mcp"
}

func TestEachRoleListsOnlyItsOwnTools(t *testing.T) {
	st, srv, url := setup(t)
	edit := []string{"list_drafts", "propose_draft", "revise_draft", "withdraw_draft"}
	for role, want := range map[Role][]string{
		Reviewer:    edit,
		Synthesizer: edit,
		Verifier:    {"list_drafts", "rate_draft"},
		Suggester:   {"read_draft", "set_suggestion"},
	} {
		tok := srv.Register(st, Session{ID: "s", Role: role})
		_, out := rpc(t, url, tok, "tools/list", nil)
		var names []string
		for _, tool := range out["result"].(map[string]any)["tools"].([]any) {
			names = append(names, tool.(map[string]any)["name"].(string))
		}
		slices.Sort(names)
		if !slices.Equal(names, want) {
			t.Fatalf("role %d tools %q, want exactly %q", role, names, want)
		}
		if got := slices.Sorted(slices.Values(ToolNames(role))); !slices.Equal(got, want) {
			t.Fatalf("role %d allowlist %q, want %q", role, got, want)
		}
		for _, n := range names {
			for _, forge := range []string{"post", "publish", "submit", "approve", "merge", "comment", "push"} {
				if strings.Contains(n, forge) {
					t.Fatalf("tool %q reaches a forge", n)
				}
			}
		}
	}
}

func TestOverMCPOnlyAVerifierRatesAndAVerifierOnlyRates(t *testing.T) {
	st, srv, url := setup(t)
	ta, tv := srv.Register(st, alice), srv.Register(st, Session{ID: "v", Role: Verifier, Drafted: "Claude Code · opus"})
	text, isErr := callTool(t, url, ta, "propose_draft", map[string]any{"severity": "high", "title": "x", "description": "y"})
	if isErr {
		t.Fatal(text)
	}
	if text, isErr := callTool(t, url, ta, "rate_draft", map[string]any{"id": "d1", "level": "confirmed", "reason": "mine"}); !isErr || !strings.Contains(text, "tool.unknown") {
		t.Fatalf("a reviewer rated: %q", text)
	}
	if text, isErr := callTool(t, url, tv, "propose_draft", map[string]any{"severity": "high", "title": "x", "description": "y"}); !isErr || !strings.Contains(text, "tool.unknown") {
		t.Fatalf("a verifier proposed: %q", text)
	}
	if text, isErr := callTool(t, url, tv, "rate_draft", map[string]any{"id": "d1", "level": "sure", "reason": "r"}); !isErr || !strings.Contains(text, "confidence.level") {
		t.Fatalf("a bad level: %q", text)
	}
	if text, isErr := callTool(t, url, tv, "rate_draft", map[string]any{"id": "d1", "level": "confirmed", "reason": "calc.go:9 divides by n unchecked"}); isErr {
		t.Fatal(text)
	}
	d, _ := st.Get("d1")
	if d.Confidence == nil || d.Confidence.Level != Confirmed || d.Confidence.By != "v" || d.Revision != 1 {
		t.Fatalf("mark %+v, revision %d", d.Confidence, d.Revision)
	}
}

func TestASessionMustProveWhichOneItIs(t *testing.T) {
	st, srv, url := setup(t)
	if code, _ := rpc(t, url, "", "tools/list", nil); code != http.StatusUnauthorized {
		t.Fatalf("no token: HTTP %d", code)
	}
	if code, _ := rpc(t, url, "not-a-token", "tools/list", nil); code != http.StatusUnauthorized {
		t.Fatalf("wrong token: HTTP %d", code)
	}
	tok := srv.Register(st, alice)
	srv.Unregister(tok)
	if code, _ := rpc(t, url, tok, "tools/list", nil); code != http.StatusUnauthorized {
		t.Fatalf("unregistered token: HTTP %d", code)
	}
}

func TestOverMCPAnInvalidDraftNamesTheRule(t *testing.T) {
	st, srv, url := setup(t)
	tok := srv.Register(st, alice)
	text, isErr := callTool(t, url, tok, "propose_draft", map[string]any{
		"severity": "high", "title": "x", "description": "y",
		"anchor": map[string]any{"path": "calc.go", "start_line": 9, "end_line": 2},
	})
	if !isErr || !strings.Contains(text, "rule anchor.range at anchor/end_line") {
		t.Fatalf("got %q, isError %v", text, isErr)
	}
	text, isErr = callTool(t, url, tok, "propose_draft", map[string]any{
		"severity": "high", "title": "x", "description": "y", "line": 3,
	})
	if !isErr || !strings.Contains(text, "rule arguments.shape") {
		t.Fatalf("unknown field: got %q, isError %v", text, isErr)
	}
}

func TestOverMCPReviewersDraftAndASynthesizerMerges(t *testing.T) {
	st, srv, url := setup(t)
	ta, tb, ts := srv.Register(st, alice), srv.Register(st, bob), srv.Register(st, synth)

	_, out := rpc(t, url, ta, "initialize", map[string]any{"protocolVersion": "2025-03-26"})
	if v := out["result"].(map[string]any)["protocolVersion"]; v != "2025-03-26" {
		t.Fatalf("protocol %v", v)
	}

	propose := map[string]any{"severity": "high", "title": "Mean skips xs[0]", "description": "Loop from 1.",
		"anchor": map[string]any{"path": "calc.go", "start_line": 6}}
	if text, isErr := callTool(t, url, ta, "propose_draft", propose); isErr {
		t.Fatal(text)
	}
	if text, isErr := callTool(t, url, tb, "propose_draft", propose); isErr {
		t.Fatal(text)
	}
	if _, isErr := callTool(t, url, ta, "withdraw_draft", map[string]any{"id": "d2"}); !isErr {
		t.Fatal("alice withdrew bob's draft")
	}

	text, _ := callTool(t, url, ts, "list_drafts", map[string]any{})
	var listed struct{ Drafts []Draft }
	if err := json.Unmarshal([]byte(text), &listed); err != nil || len(listed.Drafts) != 2 {
		t.Fatalf("synthesizer listed %q", text)
	}
	merged := map[string]any{"severity": "high", "title": "Mean skips xs[0]", "description": "Both reviewers.",
		"sources": []string{"d1", "d2"}}
	text, isErr := callTool(t, url, ts, "propose_draft", merged)
	if isErr {
		t.Fatal(text)
	}
	all := st.All()
	if len(all) != 3 || all[2].Session != "synth" || !slices.Equal(all[2].Sources, []string{"d1", "d2"}) {
		t.Fatalf("store %+v", all)
	}

	// A notification gets 202 and no body.
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+ta)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("notification: HTTP %d", res.StatusCode)
	}
	// There is no event stream to open.
	req, _ = http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+ta)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET: HTTP %d", res.StatusCode)
	}
}

func TestOneServerKeepsEachRunsDraftsApart(t *testing.T) {
	st, srv, url := setup(t)
	other := NewStore()
	ta, ts := srv.Register(st, alice), srv.Register(other, synth)
	if _, isErr := callTool(t, url, ta, "propose_draft", map[string]any{"severity": "low", "title": "t", "description": "d"}); isErr {
		t.Fatal("propose failed")
	}
	text, _ := callTool(t, url, ts, "list_drafts", nil)
	if text != `{"drafts":[]}` || len(other.All()) != 0 || len(st.All()) != 1 {
		t.Fatalf("another run's synthesizer saw %s", text)
	}
}

func TestOverMCPASuggestionSessionSetsOnlyItsDraftsSuggestion(t *testing.T) {
	st, srv, url := setup(t)
	ta := srv.Register(st, alice)
	for _, title := range []string{"Add subtracts", "Other"} {
		if text, isErr := callTool(t, url, ta, "propose_draft", map[string]any{"severity": "high", "title": title, "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3, "end_line": 4}}); isErr {
			t.Fatal(text)
		}
	}
	tg := srv.Register(st, Session{ID: "g", Role: Suggester, Drafted: "Claude Code · sonnet", Target: "d1"})
	if text, isErr := callTool(t, url, tg, "read_draft", map[string]any{}); isErr || !strings.Contains(text, "Add subtracts") || strings.Contains(text, "Other") {
		t.Fatalf("read_draft: %q", text)
	}
	for name, args := range map[string]map[string]any{
		"propose_draft":  {"severity": "high", "title": "x", "description": "y"},
		"revise_draft":   {"id": "d1", "severity": "low", "title": "x", "description": "y"},
		"withdraw_draft": {"id": "d1"},
		"list_drafts":    {},
	} {
		if text, isErr := callTool(t, url, tg, name, args); !isErr || !strings.Contains(text, "tool.unknown") {
			t.Fatalf("a suggestion session called %s: %q", name, text)
		}
	}
	refused := map[string]map[string]any{
		"suggest.target":    {"id": "d2", "suggestion": "x"},
		"suggestion.empty":  {"id": "d1", "suggestion": "  "},
		"suggestion.anchor": {"id": "d1", "suggestion": "x", "anchor": map[string]any{"path": "calc.go", "start_line": 4, "end_line": 6}},
	}
	for rule, args := range refused {
		if text, isErr := callTool(t, url, tg, "set_suggestion", args); !isErr || !strings.Contains(text, rule) {
			t.Fatalf("want %s: %q", rule, text)
		}
	}
	if text, isErr := callTool(t, url, tg, "set_suggestion", map[string]any{"id": "d1", "suggestion": "a\nb\nc\n",
		"anchor": map[string]any{"path": "calc.go", "start_line": 2, "end_line": 4}}); isErr {
		t.Fatal(text)
	}
	d, _ := st.Get("d1")
	if d.Suggestion == nil || *d.Suggestion != "a\nb\nc" || d.Anchor.StartLine != 2 || d.Anchor.EndLine != 4 ||
		d.Title != "Add subtracts" || d.Session != alice.ID || d.Revision != 2 {
		t.Fatalf("after set_suggestion %+v", d)
	}
	if other, _ := st.Get("d2"); other.Suggestion != nil {
		t.Fatal("the other draft got a suggestion")
	}
}

// Only vuoro and the person record a fix (D50): no tool takes the fields.
func TestOverMCPNoToolRecordsAFix(t *testing.T) {
	st, srv, url := setup(t)
	tok := srv.Register(st, alice)
	for _, field := range []string{"fixed_in", "fixed_subject", "fixed_by"} {
		text, isErr := callTool(t, url, tok, "propose_draft", map[string]any{
			"severity": "high", "title": "x", "description": "y", field: "abc",
		})
		if !isErr || !strings.Contains(text, "rule arguments.shape") {
			t.Fatalf("propose_draft with %s: got %q, isError %v", field, text, isErr)
		}
	}
	callTool(t, url, tok, "propose_draft", map[string]any{"severity": "high", "title": "x", "description": "y"})
	if text, isErr := callTool(t, url, tok, "revise_draft", map[string]any{"id": "d1", "fixed_in": "abc"}); !isErr || !strings.Contains(text, "rule arguments.shape") {
		t.Fatalf("revise_draft with fixed_in: got %q, isError %v", text, isErr)
	}
	if d := st.All()[0]; d.FixedIn != "" || d.FixedBy != "" {
		t.Fatalf("a tool recorded a fix: %+v", d)
	}
}
