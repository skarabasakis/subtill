package frontend

import "embed"

// Dist holds the production Vite build for Wails desktop (and server mode assets).
//
//go:embed all:dist
var Dist embed.FS
