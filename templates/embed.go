// Package templates exposes the site templates bundled with the SSG binary.
package templates

import "embed"

// Files contains every directory below templates/ except Go source files,
// which callers ignore when listing template choices.
//
//go:embed */*
var Files embed.FS
