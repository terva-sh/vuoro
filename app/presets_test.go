package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/review"
)

// names is the preset names in order. The order is not cosmetic: it decides
// what the picker shows first, and the first preset is the window's default.
func names(ps []config.Preset) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

// viewNames is the preset names the window was served, in order.
func viewNames(ps []presetView) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

// presetsApp is a window with a configuration file of its own and no forge.
func presetsApp(t *testing.T) *App {
	t.Helper()
	return &App{StateDir: t.TempDir(), Config: &config.Config{},
		ConfigPath: filepath.Join(t.TempDir(), "vuoro", "config.json")}
}

// own is a configuration naming one code reviewer under each of ns.
func own(ns ...string) *config.Config {
	c := &config.Config{}
	for _, n := range ns {
		c.Presets = append(c.Presets, config.Preset{Name: n, Reviewers: []config.Reviewer{{Profile: "code"}}})
	}
	return c
}

// TestPresetsSupplementTheBuiltInOnes holds D61's composition rule: a
// person's presets are added to the shipped ones rather than replacing them,
// theirs come first, and one of theirs shadows a built-in of the same name
// instead of appearing beside it.
func TestPresetsSupplementTheBuiltInOnes(t *testing.T) {
	for name, c := range map[string]struct {
		cfg  *config.Config
		want []string
	}{
		"a fresh install gets what vuoro ships":  {cfg: &config.Config{}, want: []string{"quick", "standard", "thorough"}},
		"no configuration at all is the same":    {cfg: nil, want: []string{"quick", "standard", "thorough"}},
		"the person's own come first":            {cfg: own("mine"), want: []string{"mine", "quick", "standard", "thorough"}},
		"a shared name shadows, in their place":  {cfg: own("mine", "standard"), want: []string{"mine", "standard", "quick", "thorough"}},
		"shadowing every one leaves only theirs": {cfg: own("quick", "standard", "thorough"), want: []string{"quick", "standard", "thorough"}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := names(presetsOf(c.cfg)); !slices.Equal(got, c.want) {
				t.Errorf("presets %v, want %v", got, c.want)
			}
		})
	}
}

// TestAShadowedPresetIsThePersonsOwn is the half of shadowing that the names
// alone do not show: theirs wins, so vuoro improving a shipped preset never
// quietly changes a run they defined.
func TestAShadowedPresetIsThePersonsOwn(t *testing.T) {
	cfg := &config.Config{Presets: []config.Preset{
		{Name: "thorough", Reviewers: []config.Reviewer{{Profile: "documentation"}}},
	}}
	ps := presetsOf(cfg)
	if got := names(ps); !slices.Equal(got, []string{"thorough", "quick", "standard"}) {
		t.Fatalf("presets %v", got)
	}
	if len(ps[0].Reviewers) != 1 || ps[0].Reviewers[0].Profile != "documentation" {
		t.Errorf("thorough is %+v, want the person's one documentation reviewer", ps[0].Reviewers)
	}
}

// TestPresetsDoNotWriteIntoTheConfiguration catches the aliasing an append to
// c.Presets would cause. The person's slice has spare capacity here, which is
// when append writes in place, and the built-ins would then be theirs and be
// saved into their config.json.
func TestPresetsDoNotWriteIntoTheConfiguration(t *testing.T) {
	cfg := &config.Config{Presets: make([]config.Preset, 1, 8)}
	cfg.Presets[0] = config.Preset{Name: "mine", Reviewers: []config.Reviewer{{Profile: "code"}}}
	if got := len(presetsOf(cfg)); got != 4 {
		t.Fatalf("%d presets, want 4", got)
	}
	if got := names(cfg.Presets); !slices.Equal(got, []string{"mine"}) {
		t.Errorf("the configuration now holds %v, want only the person's own", got)
	}
}

// TestBuiltinPresetsNameProfilesThatExist keeps the shipped presets runnable.
// A profile review cannot resolve fails the run with "no profile named", and
// it would fail at the moment the person starts a review rather than here.
func TestBuiltinPresetsNameProfilesThatExist(t *testing.T) {
	for _, p := range config.Builtin {
		if p.Name == "" || len(p.Reviewers) == 0 {
			t.Errorf("preset %+v needs a name and at least one reviewer", p)
		}
		for _, r := range p.Reviewers {
			if !slices.ContainsFunc(review.Builtin, func(b review.Profile) bool { return b.Name == r.Profile }) {
				t.Errorf("preset %q names profile %q, which vuoro does not ship", p.Name, r.Profile)
			}
		}
	}
}

// TestPlanOnAFreshInstall is the window's half of D61's default. vuoro review
// with no -preset must reach the same place, which TestReviewOnAFreshInstall
// covers.
func TestPlanOnAFreshInstall(t *testing.T) {
	reviewers, synth, verifier, err := (&App{}).Plan("", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviewers) != 1 || reviewers[0].Profile != "code" {
		t.Errorf("reviewers %+v, want quick's one code reviewer", reviewers)
	}
	if synth != nil {
		t.Errorf("synthesizer %+v, want none for a single reviewer (D11)", synth)
	}
	if verifier == nil {
		t.Error("no verifier, and D31 gives every run one")
	}
}

// TestThePresetsPageCreatesEditsAndRemoves walks what the page does, through
// the routes it calls: a fresh install shows the three vuoro ships and
// nothing of the person's, then one is added, changed, and removed. The run
// the page exists to start is checked too, because a preset that saves and
// cannot run is not finished (TKT-01M48TMGNVS4303NW7NQW26Y6S).
func TestThePresetsPageCreatesEditsAndRemoves(t *testing.T) {
	a := presetsApp(t)
	h := a.Handler()
	v := getSetup(t, h)
	if got := viewNames(v.Presets); !slices.Equal(got, []string{"quick", "standard", "thorough"}) {
		t.Fatalf("a fresh install shows %v", got)
	}
	for _, p := range v.Presets {
		if !p.Builtin {
			t.Errorf("%q is not marked as one vuoro ships, so the page would offer to edit it", p.Name)
		}
	}

	mine := config.Preset{Name: "  mine  ", Reviewers: []config.Reviewer{{Profile: "security", Model: "opus"}}}
	code, v, body := post(t, h, "/api/setup/presets", map[string]any{"base": v.Base, "original": "", "preset": mine})
	if code != 200 {
		t.Fatalf("add: %d %s", code, body)
	}
	if got := viewNames(v.Presets); !slices.Equal(got, []string{"mine", "quick", "standard", "thorough"}) {
		t.Fatalf("after adding: %v, and the name should have been trimmed", got)
	}
	if v.Presets[0].Builtin {
		t.Error("the person's own preset came back marked as shipped")
	}

	// It runs. This is the half a save alone does not prove.
	reviewers, _, _, err := a.Plan("mine", nil, nil, nil)
	if err != nil || len(reviewers) != 1 || reviewers[0].Profile != "security" || reviewers[0].Model != "opus" {
		t.Fatalf("a run from the saved preset: %+v %v", reviewers, err)
	}

	// Changing it, including its name, replaces the one it came from.
	ours := config.Preset{Name: "ours", Reviewers: []config.Reviewer{{Profile: "code"}, {Profile: "documentation"}}}
	code, v, body = post(t, h, "/api/setup/presets", map[string]any{"base": v.Base, "original": "mine", "preset": ours})
	if code != 200 {
		t.Fatalf("edit: %d %s", code, body)
	}
	if got := viewNames(v.Presets); !slices.Equal(got, []string{"ours", "quick", "standard", "thorough"}) {
		t.Fatalf("after editing: %v", got)
	}
	if len(v.Presets[0].Reviewers) != 2 {
		t.Errorf("reviewers %+v, want the two it was changed to", v.Presets[0].Reviewers)
	}

	code, v, body = post(t, h, "/api/setup/presets/remove", map[string]any{"base": v.Base, "name": "ours"})
	if code != 200 {
		t.Fatalf("remove: %d %s", code, body)
	}
	if got := viewNames(v.Presets); !slices.Equal(got, []string{"quick", "standard", "thorough"}) {
		t.Fatalf("after removing: %v", got)
	}
}

// TestRemovingAShadowRevealsTheShippedPreset is the case that needs no code
// of its own and would be easy to break: presetsOf merges on every read, so
// the built-in is back in the answer the removal itself returns (D61).
func TestRemovingAShadowRevealsTheShippedPreset(t *testing.T) {
	h := presetsApp(t).Handler()
	v := getSetup(t, h)
	shadow := config.Preset{Name: "thorough", Reviewers: []config.Reviewer{{Profile: "documentation"}}}
	code, v, body := post(t, h, "/api/setup/presets", map[string]any{"base": v.Base, "original": "", "preset": shadow})
	if code != 200 {
		t.Fatalf("shadow: %d %s", code, body)
	}
	if got := viewNames(v.Presets); !slices.Equal(got, []string{"thorough", "quick", "standard"}) {
		t.Fatalf("shadowed: %v, want theirs in place of the shipped one", got)
	}
	if v.Presets[0].Builtin || len(v.Presets[0].Reviewers) != 1 {
		t.Fatalf("the shadow is %+v, want the person's own one reviewer", v.Presets[0])
	}

	code, v, body = post(t, h, "/api/setup/presets/remove", map[string]any{"base": v.Base, "name": "thorough"})
	if code != 200 {
		t.Fatalf("remove: %d %s", code, body)
	}
	if got := viewNames(v.Presets); !slices.Equal(got, []string{"quick", "standard", "thorough"}) {
		t.Fatalf("after removing the shadow: %v", got)
	}
	if i := slices.IndexFunc(v.Presets, func(p presetView) bool { return p.Name == "thorough" }); !v.Presets[i].Builtin || len(v.Presets[i].Reviewers) != 3 {
		t.Errorf("thorough came back as %+v, want the shipped one", v.Presets[i])
	}
}

// TestAPresetIsRefusedBeforeItIsSaved keeps config.Validate's rules in front
// of the save, so a mistake is a message in the window rather than a file
// that will not load on the next start.
func TestAPresetIsRefusedBeforeItIsSaved(t *testing.T) {
	h := presetsApp(t).Handler()
	base := getSetup(t, h).Base
	good := config.Preset{Name: "mine", Reviewers: []config.Reviewer{{Profile: "code"}}}
	if code, v, body := post(t, h, "/api/setup/presets", map[string]any{"base": base, "original": "", "preset": good}); code != 200 {
		t.Fatalf("the first save: %d %s", code, body)
	} else {
		base = v.Base
	}

	for name, c := range map[string]struct {
		preset config.Preset
		says   string
	}{
		"no name":                {config.Preset{Reviewers: []config.Reviewer{{Profile: "code"}}}, "name"},
		"a name of spaces":       {config.Preset{Name: "   ", Reviewers: []config.Reviewer{{Profile: "code"}}}, "name"},
		"no reviewers":           {config.Preset{Name: "empty"}, "at least one reviewer"},
		"a reviewer, no profile": {config.Preset{Name: "blank", Reviewers: []config.Reviewer{{Model: "opus"}}}, "profile"},
		"a name already theirs":  {config.Preset{Name: "mine", Reviewers: []config.Reviewer{{Profile: "code"}}}, "already have"},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, body := post(t, h, "/api/setup/presets", map[string]any{"base": base, "original": "", "preset": c.preset})
			if code == 200 {
				t.Fatalf("saved anyway: %s", body)
			}
			if !strings.Contains(body, c.says) {
				t.Errorf("refused with %s, which does not say %q", body, c.says)
			}
		})
	}

	// A shipped preset is not in the file, so there is nothing to remove.
	if code, _, body := post(t, h, "/api/setup/presets/remove", map[string]any{"base": base, "name": "quick"}); code == 200 {
		t.Fatalf("a shipped preset was removed: %s", body)
	}
	// A file edited by hand since the window read it is refused, not
	// overwritten, the same as a forge save.
	if code, _, body := post(t, h, "/api/setup/presets", map[string]any{"base": "stale", "original": "", "preset": good}); code == 200 {
		t.Fatalf("a stale base was saved over: %s", body)
	}
	// The one good preset is still the only one of theirs.
	if got := viewNames(getSetup(t, h).Presets); !slices.Equal(got, []string{"mine", "quick", "standard", "thorough"}) {
		t.Errorf("after the refusals: %v", got)
	}
}

// TestAnEmptySynthesizerIsLeftOut keeps the run's own defaults (D11, D31).
// The form always sends both roles, so a preset saved with them untouched
// must not pin an empty tool where Plan would have picked the reviewer's.
func TestAnEmptySynthesizerIsLeftOut(t *testing.T) {
	h := presetsApp(t).Handler()
	v := getSetup(t, h)
	p := config.Preset{
		Name:        "mine",
		Reviewers:   []config.Reviewer{{Profile: "code"}, {Profile: "security"}},
		Synthesizer: &config.Reviewer{Tool: "  ", Model: ""},
		Verifier:    &config.Reviewer{Model: "haiku"},
	}
	code, v, body := post(t, h, "/api/setup/presets", map[string]any{"base": v.Base, "original": "", "preset": p})
	if code != 200 {
		t.Fatalf("save: %d %s", code, body)
	}
	if v.Presets[0].Synthesizer != nil {
		t.Errorf("synthesizer %+v, want none so the run's default applies", v.Presets[0].Synthesizer)
	}
	if v.Presets[0].Verifier == nil || v.Presets[0].Verifier.Model != "haiku" {
		t.Errorf("verifier %+v, want the model it was given", v.Presets[0].Verifier)
	}
}

// TestShippedPresetsGrowFromQuickToThorough checks what the three are for.
// They are only worth shipping if they differ, and thorough is the one that
// reaches synthesis and verification without the person writing any config.
func TestShippedPresetsGrowFromQuickToThorough(t *testing.T) {
	for name, want := range map[string]struct {
		reviewers int
		synth     bool
	}{
		"quick":    {reviewers: 1, synth: false},
		"standard": {reviewers: 2, synth: true},
		"thorough": {reviewers: 3, synth: true},
	} {
		t.Run(name, func(t *testing.T) {
			reviewers, synth, verifier, err := (&App{}).Plan(name, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(reviewers) != want.reviewers {
				t.Errorf("%d reviewers, want %d", len(reviewers), want.reviewers)
			}
			if (synth != nil) != want.synth {
				t.Errorf("synthesizer %+v, want one: %v", synth, want.synth)
			}
			if verifier == nil {
				t.Error("no verifier, and D31 gives every run one")
			}
		})
	}
}
