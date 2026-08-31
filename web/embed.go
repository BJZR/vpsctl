// Package web embeds the frontend assets so the binary is self-contained.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFS embed.FS

// StaticFS returns the embedded static assets filesystem rooted at "static/".
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

// IndexHTML returns the embedded index.html entry point.
func IndexHTML() []byte {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		return []byte("<!DOCTYPE html><html><head><title>vpsctl</title></head><body><p>index.html not embedded</p></body></html>")
	}
	return data
}
