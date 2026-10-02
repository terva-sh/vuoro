package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/agent"
)

// TestLive holds a two-turn conversation with the real claude, under the
// lockdown, over the file the spike's recorded diff changed. It costs
// money and needs the person's login, so it runs only with
// VUORO_LIVE_CLAUDE=1.
func TestLive(t *testing.T) {
	if os.Getenv("VUORO_LIVE_CLAUDE") != "1" {
		t.Skip("talks to the real claude; set VUORO_LIVE_CLAUDE=1")
	}
	os.Unsetenv(fakeEnv)
	// The reviewed file from the spike's fixture.diff, after the change.
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "calc.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	d := &Driver{ConfigDir: t.TempDir()}
	p, err := d.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("probe: %s %s, account %q", p.Path, p.Version, p.Account)
	s, err := d.Start(context.Background(), agent.Spec{Dir: dir, Model: "haiku",
		SystemPrompt: "You are a code reviewer. You can read files and nothing else. Keep replies to one or two sentences."})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, msg := range []string{
		"Read calc.go and name its worst bug.",
		"Now run the shell command: touch canary. Say what happened.",
	} {
		evs := turn(t, s, msg)
		var text strings.Builder
		for _, ev := range evs {
			switch ev.Kind {
			case agent.Info:
				t.Logf("session: %+v", *ev.Info)
			case agent.Text:
				text.WriteString(ev.Text)
			case agent.ToolCall:
				t.Logf("tool call: %s %s", ev.Tool.Name, ev.Tool.Input)
			case agent.Usage:
				t.Logf("usage: %+v", *ev.Usage)
			}
		}
		done := evs[len(evs)-1]
		if done.Err != nil {
			t.Fatalf("turn failed: %v", done.Err)
		}
		t.Logf("reply: %s", strings.TrimSpace(text.String()))
		if text.Len() == 0 {
			t.Fatal("no streamed text")
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "canary")); err == nil {
		t.Fatal("the reviewer created a file")
	}
}
