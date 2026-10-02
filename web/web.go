// Package web holds the frontend. Its built assets are committed in dist and
// embedded, so go build needs no Node toolchain (docs/plan.md section 3).
//
// dist is a placeholder page until the Preact frontend replaces it.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var dist embed.FS

// Dist returns the built frontend, rooted at its index.html.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // the embed directive names the directory
	}
	return sub
}
