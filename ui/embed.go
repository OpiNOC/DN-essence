// Package ui exposes the embedded static web assets.
package ui

import "embed"

// Files contains the compiled static assets served by the HTTP server.
//
//go:embed dist/*
var Files embed.FS
