// Package providers exposes deployment overlays bundled with the SSG binary.
package providers

import "embed"

// Files contains the available provider directories.
//
//go:embed */*
var Files embed.FS
