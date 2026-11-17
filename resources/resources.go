// Package resources holds the images the app draws and the packagers copy.
package resources

import "embed"

//go:embed TunGo.svg
var SVG []byte

//go:embed icon.png
var PNG []byte

//go:embed icons/*.svg
var Icons embed.FS
