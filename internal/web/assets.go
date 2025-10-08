package web

import (
	"embed"
	"io/fs"
)

// Assets embeds the compiled frontend bundle.
//
//go:embed dist/*
var Assets embed.FS

// FS returns the embedded file system containing the frontend bundle.
func FS() fs.FS {
	return Assets
}
