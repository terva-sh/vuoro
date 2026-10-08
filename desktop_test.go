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

// The mark is a square PNG: a violet tile on a clear ground, with a white v
// and an amber dot, at the window's size and the tray's.
func TestTheMarkIsAWhiteVAndAnAmberDotOnViolet(t *testing.T) {
	for _, size := range []int{256, 22} {
		img, err := png.Decode(bytes.NewReader(icon(size)))
		if err != nil {
			t.Fatal(err)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Fatalf("bounds %v", b)
		}
		at := func(x, y float64) [4]uint32 { // x and y run from -1 to 1
			r, g, b, a := img.At(int((x+1)/2*float64(size)), int((y+1)/2*float64(size))).RGBA()
			return [4]uint32{r >> 8, g >> 8, b >> 8, a >> 8}
		}
		for _, c := range []struct {
			what string
			x, y float64
			want [4]uint32
		}{
			{"the corner", -0.99, -0.99, [4]uint32{0, 0, 0, 0}},
			{"the tile", 0, -0.7, [4]uint32{0x75, 0x46, 0xA4, 0xFF}},
			{"the v", -0.02, 0.36, [4]uint32{0xFF, 0xFF, 0xFF, 0xFF}},
			{"the dot", 0.48, -0.46, [4]uint32{0xF2, 0xB8, 0x4B, 0xFF}},
		} {
			if got := at(c.x, c.y); got[3] != c.want[3] || got[3] != 0 && got != c.want {
				t.Errorf("%d px: %s is %x, want %x", size, c.what, got, c.want)
			}
		}
	}
}

// Drawing the marks is quick enough to do at every launch.
func BenchmarkTheMarks(b *testing.B) {
	for b.Loop() {
		icon(64)
		icon(256)
	}
}
