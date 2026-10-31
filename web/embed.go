// Package web embeds the built React frontend (web/dist) into the binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built frontend. It is empty except for a placeholder
// until `npm run build` has been run.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
