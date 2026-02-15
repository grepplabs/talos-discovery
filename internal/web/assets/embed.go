package assets

import "embed"

// FS exposes embedded UI assets for HTTP handlers.
//
//go:embed *.html *.css
var FS embed.FS
