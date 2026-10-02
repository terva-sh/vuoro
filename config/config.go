// Package config reads vuoro's configuration file and resolves each forge's
// token (docs/plan.md section 5.1, decision D29).
//
// A token never goes in the file. The file names a command that prints the
// token, which vuoro runs without a shell, or an environment variable that
// holds it. The token lives in memory only, as a Secret that will not print.
package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Config is the configuration file.
type Config struct {
	Forges []Forge `json:"forges"`
	// Presets name the reviewers a run starts. With none, a run starts one
	// code reviewer on the tool's default model.
	Presets []Preset `json:"presets,omitempty"`
	// Profiles are the person's own, beside the built-in code, security, and
	// documentation profiles (D14).
	Profiles []Profile `json:"profiles,omitempty"`
}

// Preset is a named set of reviewers.
type Preset struct {
	Name      string     `json:"name"`
	Reviewers []Reviewer `json:"reviewers"`
	// Synthesizer is the tool and model that merge the reviewers' drafts,
	// when there is more than one reviewer. Its profile is ignored. Empty
	// means the first reviewer's tool on its default model.
	Synthesizer *Reviewer `json:"synthesizer,omitempty"`
}

// Reviewer is one reviewer in a preset: a profile on a tool and a model.
type Reviewer struct {
	Profile string `json:"profile"`
	Tool    string `json:"tool,omitempty"`  // empty for the only tool available
	Model   string `json:"model,omitempty"` // empty for the tool's default
}

// Profile is a review focus the person wrote.
type Profile struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Prompt string `json:"prompt"`
}

// Forge is one forge account.
type Forge struct {
	// Name is how vuoro shows the forge, and how a change names it.
	Name string `json:"name"`
	// Kind is the forge software. The MVP supports "gitlab".
	Kind string `json:"kind"`
	// URL is the forge's base URL, such as https://gitlab.com.
	URL string `json:"url"`
	// Username is the person's account on the forge.
	Username string `json:"username"`
	// TokenCommand is a program and its arguments, run without a shell, that
	// prints the token on stdout. A leading ~/ in the program is the home
	// directory.
	TokenCommand []string `json:"token_command,omitempty"`
	// TokenEnv names an environment variable holding the token. It is used
	// when there is no TokenCommand.
	TokenEnv string `json:"token_env,omitempty"`
	// Clone is the git transport a review fetches the project over, with
	// the person's own keys or credential helper: "ssh", the default, or
	// "https".
	Clone string `json:"clone,omitempty"`
}

// Dir is the directory the configuration file lives in: the platform's
// configuration directory, then vuoro.
func Dir() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "vuoro"), nil
}

// Path is the configuration file's default path.
func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// StateDir is where vuoro keeps its state (docs/plan.md section 8):
// $XDG_STATE_HOME/vuoro or ~/.local/state/vuoro on Linux,
// ~/Library/Application Support/vuoro on macOS, and %LOCALAPPDATA%\vuoro on
// Windows.
func StateDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		d, err := os.UserConfigDir() // ~/Library/Application Support
		if err != nil {
			return "", err
		}
		return filepath.Join(d, "vuoro"), nil
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "vuoro"), nil
		}
		return "", errors.New("LOCALAPPDATA is not set")
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" && filepath.IsAbs(d) {
		return filepath.Join(d, "vuoro"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "vuoro"), nil
}

// Load reads and checks the configuration file at path.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse reads and checks a configuration. A field it does not know is an
// error, so a token pasted into the file is refused rather than kept.
func Parse(b []byte) (*Config, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var c Config
	if err := dec.Decode(&c); err != nil {
		if strings.Contains(err.Error(), `unknown field "token"`) || strings.Contains(err.Error(), `unknown field "password"`) {
			return nil, errors.New("config: a token never goes in the config file; name a token_command or a token_env")
		}
		return nil, fmt.Errorf("config: %w", err)
	}
	seen := map[string]bool{}
	for i, f := range c.Forges {
		if err := f.check(); err != nil {
			return nil, fmt.Errorf("config: forges[%d]: %w", i, err)
		}
		if seen[f.Name] {
			return nil, fmt.Errorf("config: forges[%d]: the name %q is used twice", i, f.Name)
		}
		seen[f.Name] = true
	}
	presets := map[string]bool{}
	for i, p := range c.Presets {
		switch {
		case p.Name == "" || presets[p.Name]:
			return nil, fmt.Errorf("config: presets[%d]: a preset needs a name of its own", i)
		case len(p.Reviewers) == 0:
			return nil, fmt.Errorf("config: presets[%d]: a preset needs at least one reviewer", i)
		}
		for j, r := range p.Reviewers {
			if r.Profile == "" {
				return nil, fmt.Errorf("config: presets[%d].reviewers[%d]: a reviewer needs a profile", i, j)
			}
		}
		presets[p.Name] = true
	}
	profiles := map[string]bool{}
	for i, p := range c.Profiles {
		if p.Name == "" || profiles[p.Name] || strings.TrimSpace(p.Prompt) == "" {
			return nil, fmt.Errorf("config: profiles[%d]: a profile needs a name of its own and a prompt", i)
		}
		profiles[p.Name] = true
	}
	return &c, nil
}

func (f Forge) check() error {
	switch {
	case f.Name == "":
		return errors.New("a forge needs a name")
	case f.Kind != "gitlab":
		return fmt.Errorf("kind %q: this version supports gitlab", f.Kind)
	case f.Username == "":
		return errors.New("a forge needs the person's username")
	case len(f.TokenCommand) == 0 && f.TokenEnv == "":
		return errors.New("a forge needs a token_command or a token_env")
	case len(f.TokenCommand) > 0 && f.TokenCommand[0] == "":
		return errors.New("token_command needs a program")
	case f.Clone != "" && f.Clone != "ssh" && f.Clone != "https":
		return fmt.Errorf("clone %q: ssh or https", f.Clone)
	}
	u, err := url.Parse(f.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		return fmt.Errorf("url %q: an https base URL, such as https://gitlab.com", f.URL)
	}
	return nil
}

// Secret is a token held in memory. It prints as [redacted] however it is
// formatted or marshalled, so an accidental log line cannot carry it.
type Secret struct{ v string }

// Reveal returns the token, for the one place that sends it to the forge.
func (s Secret) Reveal() string { return s.v }

func (Secret) String() string               { return "[redacted]" }
func (Secret) GoString() string             { return "[redacted]" }
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }
func (Secret) MarshalText() ([]byte, error) { return []byte("[redacted]"), nil }
func (s Secret) Format(f fmt.State, _ rune) { io.WriteString(f, "[redacted]") }
func (Secret) LogValue() slog.Value         { return slog.StringValue("[redacted]") }
func (s Secret) Empty() bool                { return s.v == "" }

// tokenTimeout bounds the token command, which may wait on a keychain.
const tokenTimeout = 30 * time.Second

// Token resolves the forge's token. An error never includes what the command
// printed, on stdout or stderr, since either could hold the token.
func (f Forge) Token(ctx context.Context) (Secret, error) {
	if len(f.TokenCommand) > 0 {
		return runTokenCommand(ctx, f.Name, f.TokenCommand)
	}
	v := strings.TrimSpace(os.Getenv(f.TokenEnv))
	if v == "" {
		return Secret{}, fmt.Errorf("forge %s: $%s is not set", f.Name, f.TokenEnv)
	}
	return Secret{v}, nil
}

func runTokenCommand(ctx context.Context, forge string, argv []string) (Secret, error) {
	prog := argv[0]
	if rest, ok := strings.CutPrefix(prog, "~/"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return Secret{}, err
		}
		prog = filepath.Join(home, rest)
	}
	ctx, cancel := context.WithTimeout(ctx, tokenTimeout)
	defer cancel()
	// No shell: the arguments reach the program as they are written.
	cmd := exec.CommandContext(ctx, prog, argv[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &limited{w: &out, n: 64 << 10}
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		switch {
		case ctx.Err() != nil:
			return Secret{}, fmt.Errorf("forge %s: the token command %s did not finish in %v", forge, filepath.Base(prog), tokenTimeout)
		case errors.As(err, &ee):
			return Secret{}, fmt.Errorf("forge %s: the token command %s exited with status %d", forge, filepath.Base(prog), ee.ExitCode())
		default:
			return Secret{}, fmt.Errorf("forge %s: the token command %s could not run: %w", forge, filepath.Base(prog), errors.Unwrap(err))
		}
	}
	v := strings.TrimSpace(out.String())
	if v == "" {
		return Secret{}, fmt.Errorf("forge %s: the token command %s printed nothing", forge, filepath.Base(prog))
	}
	if strings.ContainsAny(v, "\r\n") {
		return Secret{}, fmt.Errorf("forge %s: the token command %s printed more than one line", forge, filepath.Base(prog))
	}
	return Secret{v}, nil
}

// limited writes at most n bytes and drops the rest.
type limited struct {
	w io.Writer
	n int
}

func (l *limited) Write(p []byte) (int, error) {
	k := len(p)
	if len(p) > l.n {
		p = p[:l.n]
	}
	l.n -= len(p)
	_, err := l.w.Write(p)
	return k, err
}

// SecretForTest wraps a value as a Secret, for tests in other packages.
func SecretForTest(v string) Secret { return Secret{v} }
