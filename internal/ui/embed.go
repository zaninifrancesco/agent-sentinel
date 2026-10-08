// Package ui embeds the compiled cockpit (web/ built with Vite) into the Go
// binary, so the end user only needs the single `sentinel` executable.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built single-page app rooted at its index.html. When
// the frontend has not been built it only contains a .gitkeep, and the
// server falls back to a placeholder page.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}
	return sub
}
