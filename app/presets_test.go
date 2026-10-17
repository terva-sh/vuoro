package app

import (
	"slices"
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
