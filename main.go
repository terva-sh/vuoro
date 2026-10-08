// Command vuoro is one desktop window for every code review you give and get.
//
// It serves its interface over a loopback HTTP server and shows it in a tuohi
// window (docs/plan.md section 3). The server runs in a goroutine, and main
// keeps the UI loop, which macOS requires to be on the main thread.
//
//	vuoro            open the window; closing it leaves vuoro in the tray,
//	                 or quits where the desktop has no tray (D55)
//	vuoro -background  start in the tray with the window closed, as start
//	                 at login does
//	vuoro -browser   serve on loopback and print the URL, until interrupted
//	vuoro -check     load the page, check it reached the server and Go, and
//	                 exit 0 or 1
//	vuoro review     review a local range or a change headless, and print
//	                 the findings (docs/plan.md section 6.7)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/terva-sh/tuohi"
	"github.com/terva-sh/tuohi/dialog"
	"github.com/terva-sh/tuohi/instance"
	"github.com/terva-sh/tuohi/tray"
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
	background := fs.Bool(backgroundFlag[1:], false, "start in the tray with the window closed, as start at login does")
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

	// One vuoro runs per person: a later launch opens the running one's
	// window and exits. It sends no arguments, and the running one reads
	// none, since any local process can send them.
	window := !*browser && !*check
	opens := make(chan struct{}, 1)
	if window {
		lock, err := instance.Acquire(autostartID, func(instance.Message) {
			select {
			case opens <- struct{}{}:
			default:
			}
		})
		switch {
		case errors.Is(err, instance.ErrAlreadyRunning):
			if err := instance.Send(autostartID, nil); err != nil {
				logger.Printf("vuoro is already running, and did not answer: %v", err)
				return 1
			}
			return 0
		case err != nil:
			logger.Printf("vuoro cannot tell whether it is already running: %v", err)
		default:
			defer lock.Release()
		}
	}

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
	d := desktop{opens: opens}
	if window {
		mark := icon(64)
		a.Notify, a.Autostart, a.Log = notifier(mark), startsAtLogin(), logger.Printf
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go a.Background(ctx)
		d.tray = trayHost(runtime.GOOS, askWatcher)
		d.hidden = *background && d.tray
		d.mark = mark
		d.refresh = func() { a.RefreshNow(ctx) }
	}
	return showWindow(url, token, *check, d, logger)
}

// desktop is how the window lives beside the tray (D55).
type desktop struct {
	tray    bool // the desktop shows a tray icon, so closing hides to it
	hidden  bool // start with the window hidden in the tray
	mark    []byte
	refresh func()
	opens   <-chan struct{} // a later launch asks for the window
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
		Finished: a.RunFinished,
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

// chooseFolder is the vuoro.chooseFolder binding: the native directory
// chooser, titled by the page, answering the chosen folder, or "" when the
// person cancelled or no chooser could be shown. It only answers a path.
// The page adds it through the guarded API, which checks it as it checks a
// typed one (docs/plan.md section 6.8).
func chooseFolder(show func(dialog.Options) ([]string, error)) func(title string) string {
	return func(title string) string {
		paths, err := show(dialog.Options{Type: dialog.TypeDirectory, Title: title})
		if err != nil || len(paths) == 0 {
			return ""
		}
		return paths[0]
	}
}

// showWindow opens the window on url and runs the UI loop on the main thread.
// With a tray, closing the window leaves vuoro running in it, and the tray
// opens the window again and quits.
func showWindow(url, token string, check bool, d desktop, logger *log.Logger) int {
	var app *tuohi.App
	reports := make(chan report, 1)

	var view *tuohi.View
	view = &tuohi.View{
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
			// The folder chooser answers a path and adds nothing itself.
			"vuoro.chooseFolder": chooseFolder(func(o dialog.Options) ([]string, error) { return view.Dialog(o) }),
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
	// The tray's clicks wait until the app and its first window exist: on
	// Linux they arrive on the bus's goroutines, and the tray is up before
	// the app is made. Closing ready also publishes app to them.
	ready := make(chan struct{})
	open := func() {
		<-ready
		if err := app.Show(view); err != nil {
			logger.Printf("open the window: %v", err)
		}
	}
	menu := tray.Config{Icon: d.mark, AppName: "vuoro", Title: "vuoro", Tooltip: "vuoro", OnClick: open,
		Items: []tray.Item{
			{Label: "Open vuoro", OnClick: open},
			{Label: "Refresh now", OnClick: func() { go d.refresh() }},
			{Separator: true},
			{Label: "Quit vuoro", OnClick: func() { <-ready; app.Quit() }},
		}}

	// Whether closing the window hides vuoro or quits it is fixed when the
	// first window opens, so the tray must be up by then wherever it can
	// fail. On Linux and the BSDs it is the session bus's, needs no UI loop,
	// and is set here; a failure there means closing quits, and vuoro does
	// not start hidden. On macOS and Windows the tray needs the loop, so it
	// is set as the loop starts, after closing has been fixed to hide. It
	// fails there only in a broken session, and then vuoro reports it and
	// quits rather than run on with no icon to quit it from.
	inTray := d.tray
	early := runtime.GOOS != "darwin" && runtime.GOOS != "windows"
	if inTray && early {
		if err := tray.Set(menu); err != nil {
			logger.Printf("tray: %v; closing the window quits", err)
			inTray = false
		} else {
			defer tray.Remove()
		}
	}
	app = &tuohi.App{Name: "vuoro", Exit: !inTray}
	if inTray && !early {
		app.Start = func() error {
			if err := tray.Set(menu); err != nil {
				return fmt.Errorf("vuoro could not put its icon in the tray, so it has quit: %w", err)
			}
			if d.hidden {
				view.Hide()
			}
			return nil
		}
		defer tray.Remove()
	}

	if check {
		go func() {
			time.Sleep(checkTimeout)
			logger.Printf("the page did not report within %v", checkTimeout)
			app.Quit()
		}()
	}

	// The first window is made here, on the main thread, even when vuoro
	// starts hidden: on Linux that pins the UI thread, so a later launch can
	// open the window from another goroutine.
	if err := app.Show(view); err != nil {
		logger.Print(err)
		return 1
	}
	if d.hidden && inTray && early {
		view.Hide()
	}
	close(ready)
	go func() {
		for range d.opens {
			open()
		}
	}()
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
