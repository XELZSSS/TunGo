package app

import (
	"github.com/egoist/mygo/ui"

	"tungo/resources"
)

func loadIcon(name string) *ui.SVG {
	svg, err := resources.Icons.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return ui.MustParseSVG(svg)
}

var (
	iconMinimize = loadIcon("minimize")
	iconMaximize = loadIcon("maximize")
	iconRestore  = loadIcon("restore")
	iconClose    = loadIcon("close")
	iconUp       = loadIcon("up")
	iconDown     = loadIcon("down")
	iconDot      = loadIcon("dot")
)
