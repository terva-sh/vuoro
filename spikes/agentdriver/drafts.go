package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

// serveDrafts is a stdio MCP server, newline-delimited JSON-RPC 2.0, with one
// tool. propose_draft appends the draft to -out and answers with its number,
// or refuses a draft that breaks a rule and names the rule.
func serveDrafts(args []string) error {
	fs := flag.NewFlagSet("mcp-drafts", flag.ContinueOnError)
	out := fs.String("out", "", "file that drafts are appended to, one JSON object per line")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("mcp-drafts needs -out")
	}
	enc := json.NewEncoder(os.Stdout)
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	n := 0
	for sc.Scan() {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			continue
		}
		if req.ID == nil { // a notification
			continue
		}
		var result any
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &p)
			result = map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "vuoro-drafts", "version": "spike"},
			}
		case "tools/list":
			result = map[string]any{"tools": []any{proposeDraftTool}}
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			text, isErr := callDraft(p.Name, p.Arguments, *out, &n)
			result = map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
				"isError": isErr,
			}
		case "ping":
			result = map[string]any{}
		default:
			_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID,
				"error": map[string]any{"code": -32601, "message": "method not found: " + req.Method}})
			continue
		}
		_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}
	return sc.Err()
}

var proposeDraftTool = map[string]any{
	"name":        "propose_draft",
	"description": "Propose one pending review comment. It is a draft: a person reviews it before anything is posted.",
	"inputSchema": map[string]any{
		"type":     "object",
		"required": []string{"severity", "title", "body"},
		"properties": map[string]any{
			"severity": map[string]any{"type": "string", "enum": []string{"critical", "high", "medium", "low", "info"}},
			"title":    map[string]any{"type": "string"},
			"body":     map[string]any{"type": "string"},
			"path":     map[string]any{"type": "string"},
			"line":     map[string]any{"type": "integer", "minimum": 1},
		},
	},
}

type draft struct {
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
}

func callDraft(name string, raw json.RawMessage, out string, n *int) (string, bool) {
	if name != "propose_draft" {
		return "unknown tool " + name, true
	}
	var d draft
	if err := json.Unmarshal(raw, &d); err != nil {
		return "rule: arguments must be a JSON object matching the schema: " + err.Error(), true
	}
	switch {
	case strings.TrimSpace(d.Title) == "":
		return "rule: title must not be empty", true
	case d.Line != 0 && d.Path == "":
		return "rule: a line needs a path", true
	}
	f, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "drafts store unavailable", true
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(d); err != nil {
		return "drafts store unavailable", true
	}
	*n++
	return fmt.Sprintf("draft d%d recorded as pending", *n), false
}
