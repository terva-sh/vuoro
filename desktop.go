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

// icon is vuoro's tray and notification mark, drawn rather than shipped: a
// disc in the selected violet with a white ring that does not quite close,
// a turn. Size is in pixels.
func icon(size int) []byte {
	violet := color.NRGBA{0x75, 0x46, 0xA4, 0xFF}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const samples = 4 // per axis, to smooth the edges
	c := float64(size) / 2
	for y := range size {
		for x := range size {
			var disc, ring float64
			for sy := range samples {
				for sx := range samples {
					px := float64(x) + (float64(sx)+0.5)/samples - c
					py := float64(y) + (float64(sy)+0.5)/samples - c
					r := math.Hypot(px, py) / c
					if r > 1 {
						continue
					}
					disc++
					// The ring leaves a gap at the top right.
					angle := math.Atan2(py, px)
					if r > 0.5 && r < 0.72 && !(angle > -math.Pi/2 && angle < -math.Pi/6) {
						ring++
					}
				}
			}
			n := float64(samples * samples)
			if disc == 0 {
				continue
			}
			w := ring / disc
			mix := func(a uint8) uint8 { return uint8(float64(a)*(1-w) + 255*w) }
			img.SetNRGBA(x, y, color.NRGBA{mix(violet.R), mix(violet.G), mix(violet.B), uint8(255 * disc / n)})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
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
