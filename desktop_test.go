package main

import (
	"bytes"
	"errors"
	"image/png"
	"testing"
)

// A tray is taken to exist on macOS and Windows, and on Linux and the BSDs
// only when the bus says a StatusNotifierWatcher runs; when the bus cannot
// say, there is none, so closing the window quits rather than losing it.
func TestATrayIsUsedOnlyWhereTheDesktopShowsOne(t *testing.T) {
	asked := 0
	ask := func(has bool, err error) func() (bool, error) {
		return func() (bool, error) { asked++; return has, err }
	}
	for _, c := range []struct {
		goos string
		has  bool
		err  error
		want bool
	}{
		{"darwin", false, errors.New("no bus"), true},
		{"windows", false, errors.New("no bus"), true},
		{"linux", true, nil, true},
		{"linux", false, nil, false},
		{"linux", true, errors.New("no session bus"), false},
		{"freebsd", true, nil, true},
		{"openbsd", true, nil, false},
	} {
		if got := trayHost(c.goos, ask(c.has, c.err)); got != c.want {
			t.Errorf("%s, has %v, err %v: %v", c.goos, c.has, c.err, got)
		}
	}
	if asked != 4 {
		t.Errorf("the bus was asked %d times, want only on linux and freebsd", asked)
	}
}

// The mark is a square PNG: a violet disc on a clear ground.
func TestTheMarkIsAVioletDisc(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(icon(64)))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("bounds %v", b)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("the corner is not clear: alpha %d", a)
	}
	r, g, b, a := img.At(32, 32).RGBA()
	if r>>8 != 0x75 || g>>8 != 0x46 || b>>8 != 0xA4 || a>>8 != 0xFF {
		t.Errorf("the centre is %x %x %x %x, want the violet", r>>8, g>>8, b>>8, a>>8)
	}
}
