package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// The reviewer lockdown for terva, as the spike settles it.
func tervaArgs(model string) []string {
	args := []string{
		"rpc",
		// Only the read tools from the built-in set.
		"--tools", "read,grep,glob",
		// None of the person's extensions or MCP servers.
		"--no-ext", "--no-mcp",
		// Read-only tools run, and anything else is refused, because RPC has no
		// prompt and no approval carrier is set.
		"--approval", "plan",
		// No harness self-context and no discovery: the briefing stands alone.
		"--portable=strict",
		"--no-memory", "--no-ticket", "--no-skill", "--no-lore",
		"--no-project",
		"--system-prompt", reviewerPrompt,
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

func runTerva(args []string) error {
	fs := flag.NewFlagSet("terva", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "the worktree to review")
	record := fs.String("record", "", "file for the raw NDJSON transcript")
	drafts := fs.String("drafts", "", "file the drafts server appends to")
	model := fs.String("model", "", "model, or terva's default")
	session := fs.String("session", "", "session file, for a resumable session")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cwd == "" || *record == "" || *drafts == "" || fs.NArg() == 0 {
		return errors.New("terva needs -cwd, -record, -drafts, and at least one turn")
	}
	var err error
	if *drafts, err = filepath.Abs(*drafts); err != nil {
		return err
	}

	rec, err := os.Create(*record)
	if err != nil {
		return err
	}
	defer rec.Close()

	a := tervaArgs(*model)
	if *session != "" {
		a = append(a, "--session", *session)
	} else {
		a = append(a, "--no-session")
	}
	cmd := exec.Command("terva", a...)
	cmd.Dir = *cwd
	// A per-launch token, so only this process can drive the child.
	token := newToken()
	cmd.Env = append(childEnv(), "TERVACORE_RPC_TOKEN="+token)
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
	send := func(v map[string]any) error {
		b, _ := json.Marshal(v)
		_, err := stdin.Write(append(b, '\n'))
		return err
	}

	if err := send(map[string]any{"id": "0", "type": "hello", "token": token}); err != nil {
		return err
	}
	if err := readTervaResponse(sc, "0"); err != nil {
		return err
	}
	for i, turn := range fs.Args() {
		fmt.Printf("\n== turn %d: %s\n", i+1, turn)
		if err := send(map[string]any{"id": fmt.Sprint(i + 1), "type": "prompt", "message": turn}); err != nil {
			return err
		}
		if err := readTervaTurn(sc); err != nil {
			return err
		}
	}
	stdin.Close()
	return cmd.Wait()
}

func readTervaResponse(sc *bufio.Scanner, id string) error {
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return fmt.Errorf("not JSON: %q", sc.Text())
		}
		if ev["type"] == "response" && ev["id"] == id {
			if ev["success"] != true {
				return fmt.Errorf("%v failed: %v", ev["command"], ev["error"])
			}
			d, _ := ev["data"].(map[string]any)
			fmt.Printf("[session] tool=terva version=%v protocol=%v provider=%v model=%v\n",
				d["version"], d["protocol_version"], d["provider"], d["model"])
			return nil
		}
	}
	return errors.New("stream ended before the hello response")
}

// readTervaTurn prints normalised events until the prompt's done.
func readTervaTurn(sc *bufio.Scanner) error {
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return fmt.Errorf("not JSON: %q", sc.Text())
		}
		switch ev["type"] {
		case "text_delta":
			fmt.Print(ev["delta"])
		case "tool_call":
			in, _ := json.Marshal(ev["args"])
			fmt.Printf("\n[tool_call] %v %s\n", ev["name"], in)
		case "tool_result":
			c, _ := json.Marshal(ev["content"])
			fmt.Printf("[tool_result] error=%v %s\n", ev["is_error"], clip(string(c)))
		case "usage":
			c, _ := ev["cumulative"].(map[string]any)
			fmt.Printf("\n[usage] cumulative cost_usd=%v\n", c["cost_usd"])
		case "error":
			fmt.Printf("\n[error] %v\n", ev["error"])
		case "done":
			fmt.Printf("[done] error=%v\n", ev["error"])
			return nil
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("stream ended before the turn's done")
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
