package drafts

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
)

// Server serves a Store to agent sessions as an MCP server over Streamable
// HTTP, mounted on vuoro's loopback server behind its guard. A session proves
// which one it is with its own bearer token, from Register, so a reviewer
// cannot act as another session or as the synthesizer.
//
// It answers each request with one JSON response and opens no event stream,
// which the transport allows. Its only tools are the ones Tools lists for a
// session's role. None of them reaches a forge.
type Server struct {
	mu       sync.Mutex
	sessions map[[32]byte]entry // by the SHA-256 of the bearer token
}

// entry is a registered session and the store of the run it belongs to.
type entry struct {
	store *Store
	sess  Session
}

// NewServer returns a server with no sessions. One server serves every run,
// and each session's token names its run's store.
func NewServer() *Server {
	return &Server{sessions: map[[32]byte]entry{}}
}

// Register admits a session to store and returns the bearer token it must
// send.
func (s *Server) Register(store *Store, sess Session) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	s.sessions[sha256.Sum256([]byte(tok))] = entry{store, sess}
	s.mu.Unlock()
	return tok
}

// Unregister turns a session's token away from now on.
func (s *Server) Unregister(token string) {
	s.mu.Lock()
	delete(s.sessions, sha256.Sum256([]byte(token)))
	s.mu.Unlock()
}

func (s *Server) session(r *http.Request) (entry, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || tok == "" {
		return entry{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.sessions[sha256.Sum256([]byte(tok))]
	return e, ok
}

// protocolVersions are the MCP revisions this server speaks, newest first.
var protocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e, ok := s.session(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		// No server-initiated stream: every answer rides its request.
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil || req.JSONRPC != "2.0" || req.Method == "" {
		writeRPC(w, nil, nil, &rpcError{-32600, "invalid request"})
		return
	}
	if req.ID == nil { // a notification wants no answer
		w.WriteHeader(http.StatusAccepted)
		return
	}
	result, rerr := e.dispatch(req)
	writeRPC(w, req.ID, result, rerr)
}

func writeRPC(w http.ResponseWriter, id json.RawMessage, result any, rerr *rpcError) {
	if id == nil {
		id = json.RawMessage("null")
	}
	msg := map[string]any{"jsonrpc": "2.0", "id": id}
	if rerr != nil {
		msg["error"] = rerr
	} else {
		msg["result"] = result
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msg)
}

func (e entry) dispatch(req rpcRequest) (any, *rpcError) {
	sess := e.sess
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		v := protocolVersions[0]
		if slices.Contains(protocolVersions, p.ProtocolVersion) {
			v = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "vuoro-drafts", "version": "1"},
			"instructions":    instructions(sess.Role),
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": Tools(sess.Role)}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		return e.call(p.Name, p.Arguments), nil
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func instructions(role Role) string {
	if role == Triage {
		return "Group each supplied human thread as must_fix, discuss, or nit with a reason. Use propose_draft with thread_id, category, reason and optional description (a reply). You only write local drafts. The person edits, accepts and posts."
	}
	if role == Suggester {
		return "You write the suggested change for one finding. read_draft shows it. Set the replacement lines with set_suggestion, or reply saying why no replacement of those lines fixes it. You cannot change anything else."
	}
	if role == Verifier {
		return "You check the findings other agents drafted. list_drafts shows them. Mark each one with rate_draft: confirmed, plausible, or doubtful, with a one-line reason. You cannot propose, change, or withdraw a draft."
	}
	if role == Synthesizer {
		return "You merge the reviewers' drafts. list_drafts shows every session's drafts. Propose each merged draft with the ids of the drafts it merges as sources, and keep a finding only one reviewer raised. Nothing you draft is posted until the person posts it."
	}
	return "Record each finding as a pending draft with propose_draft. You can revise or withdraw your own drafts. Nothing you draft is posted until the person posts it."
}

// Tools lists the server's tools for a session of the given role. A
// reviewer and a synthesizer list, propose, revise, and withdraw. A verifier
// lists and rates, and nothing else. None reaches a forge.
func Tools(role Role) []map[string]any {
	if role == Summary {
		return nil
	}
	list := map[string]any{
		"name":        "list_drafts",
		"description": "List the pending drafts this session can see.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
			"include_withdrawn": map[string]any{"type": "boolean"},
		}},
		"annotations": map[string]any{"readOnlyHint": true},
	}
	if role == Suggester {
		return []map[string]any{{
			"name":        "read_draft",
			"description": "Read the finding this session writes a suggestion for.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
			"annotations": map[string]any{"readOnlyHint": true},
		}, {
			"name":        "set_suggestion",
			"description": "Set the finding's suggested change: the exact text that replaces its anchored head lines. Setting it again replaces it.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"id", "suggestion"}, "properties": map[string]any{
				"id":         map[string]any{"type": "string"},
				"suggestion": map[string]any{"type": "string", "description": "Whole lines with their indentation, with no code fence, replacing every line of the anchor."},
				"anchor": map[string]any{
					"type":        "object",
					"description": "Only to widen the range the suggestion replaces: the same file, head side, still covering the finding's lines, on lines the diff shows.",
					"required":    []string{"path", "start_line", "end_line"},
					"properties": map[string]any{
						"path":       map[string]any{"type": "string"},
						"side":       map[string]any{"type": "string", "enum": []Side{Head}},
						"start_line": map[string]any{"type": "integer", "minimum": 1},
						"end_line":   map[string]any{"type": "integer", "minimum": 1},
					},
				},
			}},
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false},
		}}
	}
	if role == Verifier {
		return []map[string]any{list, {
			"name":        "rate_draft",
			"description": "Mark how far a draft held up against the code: confirmed, plausible, or doubtful, with a one-line reason. Rating again replaces your mark.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"id", "level", "reason"}, "properties": map[string]any{
				"id":     map[string]any{"type": "string"},
				"level":  map[string]any{"type": "string", "enum": levels},
				"reason": map[string]any{"type": "string", "description": "One line, at most 512 bytes: what in the code settles it."},
			}},
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false},
		}}
	}
	proposal := map[string]any{
		"severity":    map[string]any{"type": "string", "enum": severities},
		"title":       map[string]any{"type": "string", "description": "One line, at most 256 bytes."},
		"description": map[string]any{"type": "string", "description": "What is wrong and why it matters."},
		"evidence":    map[string]any{"type": "string", "description": "What in the code shows it."},
		"suggestion":  map[string]any{"type": "string", "description": "For a critical, high, or medium finding whose fix is a replacement of the anchored lines: the exact text that replaces them from start_line to end_line at the head, as whole lines with their indentation, with no code fence. Needs a head-side anchor."},
		"anchor": map[string]any{
			"type":     "object",
			"required": []string{"path", "start_line"},
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "Relative to the repository root."},
				"side":       map[string]any{"type": "string", "enum": []Side{Head, Base}, "description": "head (the change) unless the lines are only in the base."},
				"start_line": map[string]any{"type": "integer", "minimum": 1},
				"end_line":   map[string]any{"type": "integer", "minimum": 1},
			},
		},
	}
	required := []string{"severity", "title", "description"}
	if role == Triage {
		proposal = map[string]any{
			"thread_id":   map[string]any{"type": "string"},
			"category":    map[string]any{"type": "string", "enum": categories},
			"reason":      map[string]any{"type": "string", "description": "Why this thread belongs in the category; at most 512 bytes."},
			"description": map[string]any{"type": "string", "description": "Optional proposed reply in the author's voice, at most 20000 bytes. Omit when the evidence does not support a reply."},
		}
		required = []string{"thread_id", "category", "reason"}
	}
	if role == Synthesizer {
		proposal["sources"] = map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"},
			"description": "The ids of the drafts this one merges."}
		required = append(required, "sources")
	}
	withID := func(props map[string]any) map[string]any {
		out := map[string]any{"id": map[string]any{"type": "string"}}
		for k, v := range props {
			out[k] = v
		}
		return out
	}
	return []map[string]any{
		list,
		{
			"name":        "propose_draft",
			"description": "Propose a pending review comment. The person reads it before anything is posted.",
			"inputSchema": map[string]any{"type": "object", "required": required, "properties": proposal},
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false},
		},
		{
			"name":        "revise_draft",
			"description": "Replace the content of one of your own pending drafts.",
			"inputSchema": map[string]any{"type": "object", "required": append([]string{"id"}, required...), "properties": withID(proposal)},
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false},
		},
		{
			"name":        "withdraw_draft",
			"description": "Withdraw one of your own pending drafts.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}},
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false},
		},
	}
}

// ToolNames lists the tool names for a role, for a driver's allowlist.
func ToolNames(role Role) []string {
	var out []string
	for _, t := range Tools(role) {
		out = append(out, t["name"].(string))
	}
	return out
}

func (e entry) call(name string, args json.RawMessage) map[string]any {
	sess := e.sess
	if !slices.Contains(ToolNames(sess.Role), name) {
		return toolResult((&RuleError{"tool.unknown", "name", "one of " + strings.Join(ToolNames(sess.Role), ", ")}).Error(), true)
	}
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	var (
		out any
		err error
	)
	switch name {
	case "list_drafts":
		var p struct {
			IncludeWithdrawn bool `json:"include_withdrawn"`
		}
		if err = strictUnmarshal(args, &p); err == nil {
			list := e.store.List(sess, p.IncludeWithdrawn)
			if list == nil {
				list = []Draft{}
			}
			out = map[string]any{"drafts": list}
		}
	case "propose_draft":
		var p Proposal
		if err = strictUnmarshal(args, &p); err == nil {
			out, err = e.store.Propose(sess, p)
		}
	case "revise_draft":
		var p struct {
			ID string `json:"id"`
			Proposal
		}
		if err = strictUnmarshal(args, &p); err == nil {
			out, err = e.store.Revise(sess, p.ID, p.Proposal)
		}
	case "withdraw_draft":
		var p struct {
			ID string `json:"id"`
		}
		if err = strictUnmarshal(args, &p); err == nil {
			out, err = e.store.Withdraw(sess, p.ID)
		}
	case "read_draft":
		if err = strictUnmarshal(args, &struct{}{}); err == nil {
			list := e.store.List(sess, false)
			if len(list) != 1 {
				err = &RuleError{"draft.withdrawn", "id", "a draft that is still pending"}
			} else {
				out = list[0]
			}
		}
	case "set_suggestion":
		var p struct {
			ID         string  `json:"id"`
			Suggestion string  `json:"suggestion"`
			Anchor     *Anchor `json:"anchor"`
		}
		if err = strictUnmarshal(args, &p); err == nil {
			out, err = e.store.Suggest(sess, p.ID, p.Anchor, p.Suggestion)
		}
	case "rate_draft":
		var p struct {
			ID     string `json:"id"`
			Level  Level  `json:"level"`
			Reason string `json:"reason"`
		}
		if err = strictUnmarshal(args, &p); err == nil {
			out, err = e.store.Rate(sess, p.ID, p.Level, p.Reason)
		}
	}
	if err != nil {
		var re *RuleError
		if !errors.As(err, &re) {
			re = &RuleError{"arguments.shape", "arguments", "an object matching the tool's input schema"}
		}
		return toolResult(re.Error(), true)
	}
	b, _ := json.Marshal(out)
	return toolResult(string(b), false)
}

// strictUnmarshal refuses a field the schema does not name, so a typo comes
// back as a rule rather than being dropped.
func strictUnmarshal(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func toolResult(text string, isErr bool) map[string]any {
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
		"isError": isErr,
	}
}
