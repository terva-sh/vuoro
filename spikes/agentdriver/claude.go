package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The reviewer lockdown for Claude Code, as the spike settles it.
func claudeArgs(model, mcpConfig string) []string {
	return []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--model", model,
		// Only the read tools, from the built-in set. --restricted also drops
		// every code-running tool and WebFetch, confines the file tools to the
		// working directory, and ignores user, project, and local settings.
		"--tools", "Read,Grep,Glob",
		"--restricted",
		// No MCP server but vuoro's drafts server.
		"--strict-mcp-config",
		"--mcp-config", mcpConfig,
		// Nothing prompts and nothing is asked: a call that is not allowed
		// below is denied.
		"--permission-mode", "dontAsk",
		"--permission-prompts", "none",
		"--allowedTools", "Read,Grep,Glob,mcp__vuoro__propose_draft",
		"--setting-sources", "",
		"--disable-slash-commands",
		"--system-prompt", reviewerPrompt,
	}
}

const reviewerPrompt = `You are a code reviewer working for a person who will read your drafts before anything is posted.
You can read the repository in your working directory. You cannot run commands, and you cannot change files.
Record each finding with the propose_draft tool. Keep your chat replies short.`

func runClaude(args []string) error {
	fs := flag.NewFlagSet("claude", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "the worktree to review")
	record := fs.String("record", "", "file for the raw stream-json transcript")
	drafts := fs.String("drafts", "", "file the drafts server appends to")
	model := fs.String("model", "haiku", "model")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cwd == "" || *record == "" || *drafts == "" || fs.NArg() == 0 {
		return errors.New("claude needs -cwd, -record, -drafts, and at least one turn")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if *drafts, err = filepath.Abs(*drafts); err != nil {
		return err
	}
	cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"vuoro": map[string]any{"type": "stdio", "command": self, "args": []string{"mcp-drafts", "-out", *drafts}},
	}})

	rec, err := os.Create(*record)
	if err != nil {
		return err
	}
	defer rec.Close()

	cmd := exec.Command("claude", claudeArgs(*model, string(cfg))...)
	cmd.Dir = *cwd
	cmd.Env = childEnv()
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(io.TeeReader(stdout, rec))
	sc.Buffer(make([]byte, 1<<20), 64<<20)

	for i, turn := range fs.Args() {
		fmt.Printf("\n== turn %d: %s\n", i+1, turn)
		msg, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": turn}})
		if _, err := stdin.Write(append(msg, '\n')); err != nil {
			return err
		}
		if err := readClaudeTurn(sc); err != nil {
			return err
		}
	}
	stdin.Close()
	return cmd.Wait()
}

// readClaudeTurn prints normalised events until the turn's result.
func readClaudeTurn(sc *bufio.Scanner) error {
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return fmt.Errorf("not JSON: %q", sc.Text())
		}
		switch ev["type"] {
		case "system":
			if ev["subtype"] == "init" {
				fmt.Printf("[session] tool=claude version=%v model=%v tools=%v mcp=%v\n",
					ev["claude_code_version"], ev["model"], ev["tools"], ev["mcp_servers"])
			}
		case "stream_event":
			e, _ := ev["event"].(map[string]any)
			if d, ok := e["delta"].(map[string]any); ok && d["type"] == "text_delta" {
				fmt.Print(d["text"])
			}
		case "assistant":
			for _, b := range blocks(ev) {
				if b["type"] == "tool_use" {
					in, _ := json.Marshal(b["input"])
					fmt.Printf("\n[tool_call] %v %s\n", b["name"], in)
				}
			}
		case "user":
			for _, b := range blocks(ev) {
				if b["type"] == "tool_result" {
					fmt.Printf("[tool_result] error=%v %s\n", b["is_error"], clip(fmt.Sprint(b["content"])))
				}
			}
		case "result":
			u, _ := json.Marshal(ev["usage"])
			fmt.Printf("\n[usage] cost_usd=%v %s\n[done] %v\n", ev["total_cost_usd"], clip(string(u)), ev["subtype"])
			return nil
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("stream ended before the turn's result")
}

func blocks(ev map[string]any) []map[string]any {
	m, _ := ev["message"].(map[string]any)
	c, _ := m["content"].([]any)
	var out []map[string]any
	for _, b := range c {
		if bm, ok := b.(map[string]any); ok {
			out = append(out, bm)
		}
	}
	return out
}

func clip(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 240 {
		return s[:240] + "…"
	}
	return s
}

// childEnv passes the person's environment through, so the tool finds its own
// login, and drops the markers that tell a tool it is nested in a Claude Code
// session, which vuoro is not.
func childEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CLAUDECODE=") || strings.HasPrefix(kv, "CLAUDE_CODE_ENTRYPOINT=") {
			continue
		}
		env = append(env, kv)
	}
	return env
}
