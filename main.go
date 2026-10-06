// Command vuoro is one desktop window for every code review you give and get.
//
// It serves its interface over a loopback HTTP server and shows it in a tuohi
// window (docs/plan.md section 3). The server runs in a goroutine, and main
// keeps the UI loop, which macOS requires to be on the main thread.
//
//	vuoro            open the window, until it is closed
//	vuoro -browser   serve on loopback and print the URL, until interrupted
//	vuoro -check     load the page, check it reached the server and Go, and
//	                 exit 0 or 1
//	vuoro review     review a local range or a change headless, and print
//	                 the findings (docs/plan.md section 6.7)
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/terva-sh/tuohi"
	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/agent/claude"
	"github.com/terva-sh/vuoro/app"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/server"
	"github.com/terva-sh/vuoro/state"
	"github.com/terva-sh/vuoro/web"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = "dev"

// checkTimeout bounds -check, so a page that never reports cannot hang CI.
const checkTimeout = 30 * time.Second

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "review" {
		return runReview(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("vuoro", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	browser := fs.Bool("browser", false, "serve on loopback and print the URL, for a browser or debugging")
	check := fs.Bool("check", false, "load the page, check it reached the server and Go, and exit 0 or 1")
	addr := fs.String("addr", "127.0.0.1:0", "the loopback address to serve on")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "vuoro %s\n", version)
		return 0
	}
	if *browser && *check {
		fmt.Fprintln(stderr, "vuoro: -browser and -check do not go together")
		return 2
	}
	logger := log.New(stderr, "vuoro: ", 0)

	ln, err := server.Listen(*addr)
	if err != nil {
		logger.Print(err)
		return 1
	}
	// The per-launch token that the API requires (docs/plan.md section 5.2).
	token := server.NewToken()
	a, mcp, err := newApp("http://" + ln.Addr().String() + "/mcp/")
	if err != nil {
		logger.Print(err)
		return 1
	}
	srv := server.HTTPServer(server.New(server.Config{
		Addr: ln.Addr().String(), Token: token, Dist: web.Dist(), Version: version,
		MCP: mcp, API: a.Handler(),
	}))
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("serve: %v", err)
		}
	}()
	defer srv.Close()
	// No agent outlives vuoro, however it ends short of being killed.
	defer a.Runs.Shutdown()
	url := "http://" + ln.Addr().String() + "/"

	if *browser {
		return serveForBrowser(url, token, stdout)
	}
	return showWindow(url, token, *check, logger)
}

// newApp loads the person's configuration and builds the API. A missing
// configuration file is not an error: changes then open anonymously. A
// broken one is shown to the person rather than refused, so they can see why.
func newApp(mcpURL string) (*app.App, *drafts.Server, error) {
	stateDir, err := config.StateDir()
	if err != nil {
		return nil, nil, err
	}
	a := &app.App{StateDir: stateDir}
	if path, err := config.Path(); err != nil {
		a.ConfigErr = err
	} else {
		// The path is kept even when the file does not load, so setup can
		// name it and pick the fix up without a restart.
		a.ConfigPath = path
		if c, base, err := config.Read(path); err == nil {
			a.Config, a.ConfigBase = c, base
		} else {
			a.ConfigErr = err
		}
	}
	mcpDir := filepath.Join(stateDir, "mcp")
	if err := state.MkdirAll(mcpDir); err != nil {
		return nil, nil, err
	}
	srv := drafts.NewServer()
	a.Tools = map[string]agent.Driver{"claude": &claude.Driver{ConfigDir: mcpDir}}
	a.Runs = &review.Manager{
		StateDir: stateDir,
		Repos:    &review.Repos{Dir: filepath.Join(stateDir, "repos")},
		Drafts:   srv,
		MCPURL:   mcpURL,
	}
	if a.Config != nil {
		for _, p := range a.Config.Profiles {
			a.Runs.Profiles = append(a.Runs.Profiles, review.Profile(p))
		}
	}
	// Runs outlive a restart. A record that will not read is skipped, and the
	// rest still load.
	if err := a.Runs.Load(); err != nil {
		return nil, nil, err
	}
	return a, srv, nil
}

// serveForBrowser prints the URL and serves until interrupted. The token rides
// in the fragment, which a browser never sends to a server, and the page moves
// it into sessionStorage.
func serveForBrowser(url, token string, stdout io.Writer) int {
	fmt.Fprintf(stdout, "%s#token=%s\n", url, token)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	return 0
}

// report is what the page sends back through the vuoro.report binding.
type report struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// showWindow opens the window on url and runs the UI loop on the main thread.
func showWindow(url, token string, check bool, logger *log.Logger) int {
	app := &tuohi.App{Name: "vuoro", Exit: true}
	reports := make(chan report, 1)

	view := &tuohi.View{
		URL:    url,
		Title:  "vuoro",
		Frame:  true,
		Width:  1100,
		Height: 760,
		Bind: map[string]any{
			// A binding runs on a goroutine of its own, never the UI thread.
			"vuoro.version": func() string { return version },
			// tuohi offers bindings only to the origin the view trusts, its
			// own loopback server, so only vuoro's page can read the token.
			"vuoro.token": func() string { return token },
			"vuoro.report": func(r report) {
				select {
				case reports <- r:
				default:
				}
				if check {
					app.Quit()
				}
			},
		},
	}
	if check {
		go func() {
			time.Sleep(checkTimeout)
			logger.Printf("the page did not report within %v", checkTimeout)
			app.Quit()
		}()
	}

	if err := app.Show(view); err != nil {
		logger.Print(err)
		return 1
	}
	if err := app.Wait(); err != nil {
		logger.Print(err)
		return 1
	}
	if !check {
		return 0
	}
	select {
	case r := <-reports:
		logger.Printf("check: ok=%v %s", r.OK, r.Detail)
		if !r.OK {
			return 1
		}
		return 0
	default:
		logger.Print("check: no report from the page")
		return 1
	}
}
