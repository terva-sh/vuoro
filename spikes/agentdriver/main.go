// Command agentdriver is the throwaway program from
// TKT-01M3YK0CFFKAPM2QHZGGZMDHND (Decide the agent driver interface for terva
// and Claude Code). It holds a streamed, multi-turn conversation with one
// agent tool under the reviewer lockdown, and serves the stub drafts MCP
// server that the agent writes through. It is not vuoro: the real drivers go
// in agent/claude and agent/terva, and this directory is deleted once they
// exist.
//
//	agentdriver claude -cwd DIR -record FILE -drafts FILE TURN...
//	agentdriver terva  -cwd DIR -record FILE -drafts FILE TURN...
//	agentdriver mcp-drafts -out FILE
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: agentdriver claude|terva|mcp-drafts ...")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "claude":
		err = runClaude(os.Args[2:])
	case "terva":
		err = runTerva(os.Args[2:])
	case "mcp-drafts":
		err = serveDrafts(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agentdriver:", err)
		os.Exit(1)
	}
}
