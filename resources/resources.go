package resources

import "embed"

//go:embed TunGo.svg
var SVG []byte

//go:embed icon.png
var PNG []byte

//go:embed icons/*.svg
var Icons embed.FS
