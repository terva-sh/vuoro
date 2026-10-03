package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

// The test binary stands in for a token command: with fakeEnv set, it prints
// its arguments' instruction and exits.
const fakeEnv = "VUORO_FAKE_TOKEN_COMMAND"

func TestMain(m *testing.M) {
	if mode, ok := os.LookupEnv(fakeEnv); ok {
		switch mode {
		case "print":
			fmt.Print("glpat-from-the-command\n")
		case "args":
			fmt.Print(strings.Join(os.Args[1:], "|"))
		case "fail":
			fmt.Fprint(os.Stderr, "glpat-leaked-on-stderr")
			fmt.Print("glpat-leaked-on-stdout")
			os.Exit(3)
		case "empty":
		case "twolines":
			fmt.Print("glpat-a\nglpat-b\n")
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// example is the documented example in docs/plan.md section 5.1.
const example = `{
  "forges": [
    {
      "name": "gitlab.com",
      "kind": "gitlab",
      "url": "https://gitlab.com",
      "username": "drewshort",
      "token_command": ["~/.claude/scripts/ritual-secret", "get", "gitlab-pat"]
    }
  ]
}`

func TestTheDocumentedExampleParses(t *testing.T) {
	c, err := Parse([]byte(example))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Forges) != 1 || c.Forges[0].TokenCommand[2] != "gitlab-pat" {
		t.Fatalf("%+v", c)
	}
	doc, err := os.ReadFile("../docs/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), example) {
		t.Fatal("docs/plan.md section 5.1 does not carry this example verbatim")
	}
}

func TestATokenInTheFileIsRefused(t *testing.T) {
	_, err := Parse([]byte(`{"forges":[{"name":"g","kind":"gitlab","url":"https://gitlab.com","username":"u","token":"glpat-x"}]}`))
	if err == nil || !strings.Contains(err.Error(), "never goes in the config file") {
		t.Fatalf("err %v", err)
	}
	if strings.Contains(err.Error(), "glpat-x") {
		t.Fatal("the error quotes the token")
	}
}

func TestBadForgesAreRefused(t *testing.T) {
	good := Forge{Name: "g", Kind: "gitlab", URL: "https://gitlab.com", Username: "u", TokenEnv: "T"}
	for name, edit := range map[string]func(*Forge){
		"no name":         func(f *Forge) { f.Name = "" },
		"github":          func(f *Forge) { f.Kind = "github" },
		"http":            func(f *Forge) { f.URL = "http://gitlab.com" },
		"user in url":     func(f *Forge) { f.URL = "https://u:p@gitlab.com" },
		"path":            func(f *Forge) { f.URL = "https://gitlab.com/api/v4" },
		"no username":     func(f *Forge) { f.Username = "" },
		"no token source": func(f *Forge) { f.TokenEnv = "" },
		"empty program":   func(f *Forge) { f.TokenCommand = []string{""} },
		"clone over ftp":  func(f *Forge) { f.Clone = "ftp" },
	} {
		f := good
		edit(&f)
		b, _ := json.Marshal(Config{Forges: []Forge{f}})
		if _, err := Parse(b); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	b, _ := json.Marshal(Config{Forges: []Forge{good, good}})
	if _, err := Parse(b); err == nil {
		t.Error("a repeated name was accepted")
	}
}

func TestTheTokenCommandRunsWithoutAShell(t *testing.T) {
	t.Setenv(fakeEnv, "args")
	f := Forge{Name: "g", TokenCommand: []string{os.Args[0], "$(echo hi)", "a b", ";", "*"}}
	s, err := f.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Reveal(); got != "$(echo hi)|a b|;|*" {
		t.Fatalf("the arguments were changed on the way: %q", got)
	}
}

func TestTheTokenIsTrimmedAndKeptOutOfEveryPrint(t *testing.T) {
	t.Setenv(fakeEnv, "print")
	s, err := Forge{Name: "g", TokenCommand: []string{os.Args[0]}}.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Reveal() != "glpat-from-the-command" {
		t.Fatalf("token %q", s.Reveal())
	}
	var logged strings.Builder
	log.New(&logged, "", 0).Print(s)
	j, _ := json.Marshal(map[string]any{"t": s})
	for _, out := range []string{
		fmt.Sprint(s), fmt.Sprintf("%v %+v %#v %s %q %x", s, s, s, s, s, s),
		fmt.Sprintf("%+v", struct{ T Secret }{s}), logged.String(), string(j),
	} {
		if strings.Contains(out, "glpat") {
			t.Fatalf("the token printed: %q", out)
		}
	}
}

func TestATokenCommandErrorQuotesNothingItPrinted(t *testing.T) {
	for mode, want := range map[string]string{
		"fail":     "exited with status 3",
		"empty":    "printed nothing",
		"twolines": "more than one line",
	} {
		t.Setenv(fakeEnv, mode)
		_, err := Forge{Name: "g", TokenCommand: []string{os.Args[0]}}.Token(context.Background())
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", mode, err, want)
			continue
		}
		if strings.Contains(err.Error(), "glpat") {
			t.Errorf("%s: the error quotes output: %v", mode, err)
		}
	}
	_, err := Forge{Name: "g", TokenCommand: []string{"/nonexistent/token-command"}}.Token(context.Background())
	if err == nil || !strings.Contains(err.Error(), "could not run") {
		t.Errorf("missing program: %v", err)
	}
}

func TestTheEnvironmentIsTheFallback(t *testing.T) {
	t.Setenv("VUORO_TEST_TOKEN", " glpat-env ")
	s, err := Forge{Name: "g", TokenEnv: "VUORO_TEST_TOKEN"}.Token(context.Background())
	if err != nil || s.Reveal() != "glpat-env" {
		t.Fatalf("%v", err)
	}
	t.Setenv("VUORO_TEST_TOKEN", "")
	if _, err := (Forge{Name: "g", TokenEnv: "VUORO_TEST_TOKEN"}).Token(context.Background()); err == nil {
		t.Fatal("an empty variable was accepted")
	}
}

func TestPresetsAndProfilesAreChecked(t *testing.T) {
	ok := `{"forges":[],"presets":[{"name":"standard","reviewers":[{"profile":"code"},{"profile":"go","model":"opus"}]}],
		"profiles":[{"name":"go","title":"Go","prompt":"Check the Go."}]}`
	if _, err := Parse([]byte(ok)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		`{"presets":[{"name":"p","reviewers":[]}]}`,
		`{"presets":[{"name":"","reviewers":[{"profile":"code"}]}]}`,
		`{"presets":[{"name":"p","reviewers":[{"profile":"code"}]},{"name":"p","reviewers":[{"profile":"code"}]}]}`,
		`{"presets":[{"name":"p","reviewers":[{"model":"opus"}]}]}`,
		`{"profiles":[{"name":"go","title":"Go","prompt":" "}]}`,
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestForgejoConfigAndDuplicateHosts(t *testing.T) {
	f := Forge{Name: "fj", Kind: "forgejo", URL: "https://forge.example", Username: "me", TokenEnv: "VUORO_FORGEJO_TOKEN"}
	b, _ := json.Marshal(Config{Forges: []Forge{f}})
	if _, err := Parse(b); err != nil {
		t.Fatal(err)
	}
	second := f
	second.Name = "another"
	second.URL = "https://FORGE.example/"
	b, _ = json.Marshal(Config{Forges: []Forge{f, second}})
	if _, err := Parse(b); err == nil {
		t.Fatal("ambiguous account routing accepted")
	}
	f.URL = "https://user:private-value@forge.example"
	b, _ = json.Marshal(Config{Forges: []Forge{f}})
	if _, err := Parse(b); err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatal("URL credentials accepted or exposed")
	}
}
