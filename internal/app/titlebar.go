package app

import (
	"github.com/egoist/mygo/ui"
)

const titleBarHeight float32 = 36

const (
	captionMinimize = iota
	captionMaximize
	captionClose
)

func (a *app) titleBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(gapBase).AlignItems(ui.Center).Height(titleBarHeight).PaddingX(padX).Background(t.Background).DragWindow().Children(func() {
		ui.Text(c, "TunGo").FontSize(fontM).FontWeight(weightSemibold).TextColor(t.Text).IgnoreInput(true)
		ui.Box(c).Grow(1)
		a.caption(c, t, captionMinimize)
		a.caption(c, t, captionMaximize)
		a.caption(c, t, captionClose)
	})
}

func (a *app) caption(c *ui.Context, t *ui.Theme, kind int) {
	button := ui.Box(c).Size(34, 24).Radius(4).Cursor(ui.CursorPointer).OnClick(func() {
		switch kind {
		case captionMinimize:
			a.win.Minimize()
		case captionMaximize:
			if a.maximized {
				a.win.Unmaximize()
				a.maximized = false
			} else {
				a.win.Maximize()
				a.maximized = true
			}
		case captionClose:
			a.win.Close()
		}
	})

	hovered := button.Hovered()
	bg, ink := ui.Transparent, t.TextMuted
	switch {
	case hovered && kind == captionClose:
		bg, ink = t.Danger, t.InverseText
	case hovered:
		bg, ink = t.SurfaceHover, t.Text
	}
	button.Center().Background(bg).TextColor(ink).Children(func() {
		ui.Icon(c, captionIcon(kind, a.maximized)).Size(iconSize, iconSize)
	})
}

func captionIcon(kind int, maximized bool) *ui.SVG {
	switch kind {
	case captionMaximize:
		if maximized {
			return iconRestore
		}
		return iconMaximize
	case captionClose:
		return iconClose
	}
	return iconMinimize
}
