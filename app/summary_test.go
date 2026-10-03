package app

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/server"
)

type summaryProbe struct{}

type summaryForge struct{ *triageForge }

func (p *summaryForge) Open(ctx context.Context, ref forge.Ref) (*forge.Change, error) {
	ch, err := p.triageForge.Open(ctx, ref)
	if err == nil {
		ch.Checkout = p.ch.Checkout
	}
	return ch, err
}

func (summaryProbe) Name() string { return "summary fixture" }
func (summaryProbe) Probe(context.Context) (agent.Probe, error) {
	return agent.Probe{Version: "fixture"}, nil
}
func (summaryProbe) Start(context.Context, agent.Spec) (agent.Session, error) {
	return nil, errors.New("fixture must never start an agent")
}

func TestSummaryRoutePinsFreshHeadAndRejectsUnrelatedBaselineWithoutWriting(t *testing.T) {
	a, p, url := triageApp(t)
	a.Config.Forges[0].Clone = "https"
	p.ch.Diff = forge.DiffRefs{Base: strings.Repeat("a", 40), Head: strings.Repeat("b", 40)}
	p.ch.TargetBranch = "main"
	p.ch.Checkout = forge.Checkout{HTTPS: "https://lab.test/team/repo.git", HeadRef: "refs/merge-requests/1/head"}
	a.Runs.Repos = &review.Repos{Dir: filepath.Join(a.Runs.StateDir, "repos"), Git: filepath.Join(t.TempDir(), "missing-git-fixture")}
	a.Tools = map[string]agent.Driver{"fixture": summaryProbe{}}
	a.Dial = func(string, string, config.Secret) (forge.Provider, error) { return &summaryForge{p}, nil }
	h := a.Handler()
	code, b := call(t, h, "/api/summaries", map[string]any{"url": url})
	if code != 201 || b["mode"] != "summary" || b["head"] != p.ch.Diff.Head {
		t.Fatalf("fresh summary %d %v", code, b)
	}
	code, _ = call(t, h, "/api/summaries", map[string]any{"url": url, "review_id": "triage-run"})
	if code == 201 {
		t.Fatal("triage accepted as review baseline")
	}
	p.ch.Diff.Head = strings.Repeat("c", 40)
	code, b = call(t, h, "/api/summaries", map[string]any{"url": url})
	if code != 201 || b["head"] != p.ch.Diff.Head {
		t.Fatal("summary used cached head")
	}
	code, _ = call(t, h, "/api/summaries", map[string]any{"url": url, "from": strings.Repeat("d", 40)})
	if code == 201 {
		t.Fatal("caller supplied arbitrary baseline SHA")
	}
	if p.writes != 0 || p.published != 0 {
		t.Fatal("summary posted")
	}
	a.Runs.Shutdown()
}

func TestSummaryRoutesRequireWindowCredential(t *testing.T) {
	a, _, _ := triageApp(t)
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "summary-window-fixture", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, path := range []string{"/api/summaries", "/api/runs/triage-run/sessions/s1/stop"} {
		for _, mode := range []string{"missing", "agent", "origin", "host"} {
			r := httptest.NewRequest("POST", "http://127.0.0.1:9123"+path, strings.NewReader(`{}`))
			switch mode {
			case "agent":
				r.Header.Set("Authorization", "Bearer summary-agent-fixture")
			case "origin":
				r.Header.Set("X-Vuoro-Token", "summary-window-fixture")
				r.Header.Set("Origin", "https://evil.test")
			case "host":
				r.Header.Set("X-Vuoro-Token", "summary-window-fixture")
				r.Host = "evil.test"
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("%s %s: %d", path, mode, w.Code)
			}
		}
	}
}
