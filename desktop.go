package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"runtime"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/terva-sh/tuohi/autostart"
	"github.com/terva-sh/tuohi/notify"
	"github.com/terva-sh/vuoro/app"
)

// The tray, notifications, and start at login (D24, D55). Each is the
// platform's own, through tuohi, without cgo.

// autostartID names vuoro's login item.
const autostartID = "vuoro"

// backgroundFlag starts vuoro in the tray with its window closed, as start
// at login does.
const backgroundFlag = "-background"

// icon is vuoro's mark (D56), drawn rather than shipped: a white lowercase v
// on a violet tile, with an amber dot at its right arm's tip, the turn that
// is the person's. The same mark is the window's, the tray's, and the
// notifications'. Size is in pixels.
func icon(size int) []byte {
	violet := color.NRGBA{0x75, 0x46, 0xA4, 0xFF}
	white := color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
	amber := color.NRGBA{0xF2, 0xB8, 0x4B, 0xFF}
	// Each part is a test on a point, with the tile spanning -1 to 1 and y
	// running down. A point takes the colour of the first part it falls in,
	// so the parts run from the top down.
	tile := func(x, y float64) bool { // a squircle, |x|^5 + |y|^5 <= 0.92^5
		x, y = math.Abs(x), math.Abs(y)
		return x*x*x*x*x+y*y*y*y*y <= 0.92*0.92*0.92*0.92*0.92
	}
	stroke := func(ax, ay, bx, by, r float64) func(x, y float64) bool {
		return func(x, y float64) bool {
			px, py, dx, dy := x-ax, y-ay, bx-ax, by-ay
			t := max(0, min(1, (px*dx+py*dy)/(dx*dx+dy*dy)))
			ex, ey := px-t*dx, py-t*dy
			return ex*ex+ey*ey <= r*r
		}
	}
	left, right := stroke(-0.4, -0.3, -0.02, 0.4, 0.12), stroke(-0.02, 0.4, 0.36, -0.3, 0.12)
	parts := []struct {
		in func(x, y float64) bool
		c  color.NRGBA
	}{
		{func(x, y float64) bool { return (x-0.48)*(x-0.48)+(y+0.46)*(y+0.46) <= 0.12*0.12 }, amber},
		{func(x, y float64) bool { return left(x, y) || right(x, y) }, white},
		{tile, violet},
	}
	which := func(x, y float64) int { // the part a point is in, or -1
		for i, p := range parts {
			if p.in(x, y) {
				return i
			}
		}
		return -1
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	samples := 4 // per axis, to smooth the edges
	if size < 64 {
		samples = 8
	}
	n := float64(samples * samples)
	scale := 2 / float64(size)
	for py := range size {
		for px := range size {
			// A pixel whose corners all fall in one part lies inside it, since
			// no part is narrower than a pixel, and only the edges are sampled.
			x0, y0 := float64(px)*scale-1, float64(py)*scale-1
			if i := which(x0, y0); i == which(x0+scale, y0) && i == which(x0, y0+scale) && i == which(x0+scale, y0+scale) {
				if i >= 0 {
					img.SetNRGBA(px, py, parts[i].c)
				}
				continue
			}
			var r, g, b, hit float64
			for sy := range samples {
				for sx := range samples {
					x := x0 + (float64(sx)+0.5)/float64(samples)*scale
					y := y0 + (float64(sy)+0.5)/float64(samples)*scale
					if i := which(x, y); i >= 0 {
						c := parts[i].c
						r, g, b, hit = r+float64(c.R), g+float64(c.G), b+float64(c.B), hit+1
					}
				}
			}
			if hit > 0 {
				img.SetNRGBA(px, py, color.NRGBA{uint8(r / hit), uint8(g / hit), uint8(b / hit), uint8(255 * hit / n)})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// trayHost says whether this desktop shows a tray icon. macOS and Windows
// always do. On Linux and the BSDs the icon is a StatusNotifierItem, which
// shows only when a StatusNotifierWatcher is on the session bus; GNOME
// without the AppIndicator extension has none, and the icon then never
// appears. ask is the bus's answer; when it cannot be had, there is taken to
// be no tray, so closing the window quits and the window is never lost.
func trayHost(goos string, ask func() (bool, error)) bool {
	switch goos {
	case "darwin", "windows":
		return true
	case "linux", "freebsd", "netbsd":
		has, err := ask()
		return err == nil && has
	}
	return false
}

// askWatcher asks the session bus whether a StatusNotifierWatcher runs.
func askWatcher() (bool, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var has bool
	err = conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher").Store(&has)
	return has, err
}

// notifier shows vuoro's notifications at low urgency, with its mark where
// the platform takes one from memory.
func notifier(mark []byte) app.Notifier {
	return func(title, body string) error {
		opts := notify.Options{Urgency: notify.UrgencyLow, IconData: mark}
		if runtime.GOOS == "windows" { // Windows reads no PNG from memory
			opts = notify.Options{Urgency: notify.UrgencyLow, Icon: "info"}
		}
		return notify.ShowOpts("vuoro", title, body, opts)
	}
}

// loginItem is app.Autostarter over tuohi's autostart. A login item starts
// vuoro in the tray.
type loginItem struct{ a *autostart.Autostart }

func (l loginItem) Enabled() (bool, error) { return l.a.Enabled(), nil }
func (l loginItem) Enable() error          { return l.a.Enable(backgroundFlag) }
func (l loginItem) Disable() error         { return l.a.Disable() }

// startsAtLogin is vuoro's login item, or nil where the platform has none.
func startsAtLogin() app.Autostarter {
	switch runtime.GOOS {
	case "darwin", "windows", "linux", "freebsd", "netbsd":
		return loginItem{autostart.New(autostartID)}
	}
	return nil
}
