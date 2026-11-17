package app

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/i18n"
)

var (
	fontUI      = fontStack("Segoe UI Variable Text, Segoe UI Variable, Segoe UI, Microsoft YaHei UI, sans-serif", "SF Pro Text, PingFang SC, Helvetica Neue, sans-serif", "Noto Sans CJK SC, Noto Sans, DejaVu Sans, sans-serif")
	fontDisplay = fontStack("Segoe UI Variable Display, Segoe UI Variable, Segoe UI, Microsoft YaHei UI, sans-serif", "SF Pro Display, PingFang SC, Helvetica Neue, sans-serif", "Noto Sans CJK SC, Noto Sans, DejaVu Sans, sans-serif")
	fontMono    = "Cascadia Mono, Cascadia Code, Consolas, SF Mono, Menlo, DejaVu Sans Mono, monospace"
)

func fontStack(windows, darwin, linux string) string {
	switch runtime.GOOS {
	case "windows":
		return windows
	case "darwin":
		return darwin
	}
	return linux
}

const weightSemibold = 600

const (
	gapTight float32 = 4
	gapBase  float32 = 8
	gapLoose float32 = 12

	padX      float32 = 16
	padY      float32 = 8
	padYDense float32 = 4
)

const (
	fontS float32 = 12
	fontM float32 = 13
	fontL float32 = 20
)

const tableRowHeight float32 = 32

const (
	controlWide  float32 = 160
	controlWider float32 = 190
	buttonBase   float32 = 72
	iconSize     float32 = 16
	markSize     float32 = 12
)

var (
	lightTheme = &ui.Theme{
		Background:      ui.Hex("#ffffff"),
		Surface:         ui.Hex("#fafafa"),
		SurfaceHover:    ui.Hex("#f4f4f5"),
		SurfacePressed:  ui.Hex("#e4e4e7"),
		Border:          ui.Hex("#e2e2e6"),
		Text:            ui.Hex("#18181b"),
		TextMuted:       ui.Hex("#71717a"),
		Accent:          ui.Hex("#18181b"),
		AccentHover:     ui.Hex("#27272a"),
		AccentPressed:   ui.Hex("#3f3f46"),
		AccentText:      ui.Hex("#ffffff"),
		Danger:          ui.Hex("#dc2626"),
		Warning:         ui.Hex("#b45309"),
		Success:         ui.Hex("#15803d"),
		Selection:       ui.RGBA(24, 24, 27, 0.07),
		Focus:           ui.RGBA(24, 24, 27, 0.45),
		FocusRingWidth:  1,
		FocusRingOffset: 1,
		Inverse:         ui.Hex("#18181b"),
		InverseText:     ui.Hex("#ffffff"),
		Scrollbar:       ui.RGBA(24, 24, 27, 0.18),
		ScrollbarWidth:  6,
		Radius:          6,
		Spacing:         4,
		Font:            fontUI,
		FontSize:        13,
	}
	darkTheme = &ui.Theme{
		Dark:            true,
		Background:      ui.Hex("#0a0a0a"),
		Surface:         ui.Hex("#121214"),
		SurfaceHover:    ui.Hex("#1f1f22"),
		SurfacePressed:  ui.Hex("#2e2e33"),
		Border:          ui.Hex("#2a2a2f"),
		Text:            ui.Hex("#fafafa"),
		TextMuted:       ui.Hex("#a1a1aa"),
		Accent:          ui.Hex("#fafafa"),
		AccentHover:     ui.Hex("#e4e4e7"),
		AccentPressed:   ui.Hex("#d4d4d8"),
		AccentText:      ui.Hex("#0a0a0a"),
		Danger:          ui.Hex("#f87171"),
		Warning:         ui.Hex("#fbbf24"),
		Success:         ui.Hex("#4ade80"),
		Selection:       ui.RGBA(250, 250, 250, 0.10),
		Focus:           ui.RGBA(250, 250, 250, 0.45),
		FocusRingWidth:  1,
		FocusRingOffset: 1,
		Inverse:         ui.Hex("#fafafa"),
		InverseText:     ui.Hex("#0a0a0a"),
		Scrollbar:       ui.RGBA(250, 250, 250, 0.18),
		ScrollbarWidth:  6,
		Radius:          6,
		Spacing:         4,
		Font:            fontUI,
		FontSize:        13,
	}
)

var listThemes = map[*ui.Theme]*ui.Theme{
	lightTheme: listOf(lightTheme),
	darkTheme:  listOf(darkTheme),
}

func listTheme(t *ui.Theme) *ui.Theme {
	if one, ok := listThemes[t]; ok {
		return one
	}
	return listOf(t)
}

func listOf(t *ui.Theme) *ui.Theme {
	one := *t
	one.Accent = t.SurfacePressed
	one.AccentHover = t.SurfacePressed
	one.AccentPressed = t.SurfacePressed
	one.AccentText = t.Text
	return &one
}

var Themes = []string{"light", "dark", "system"}

func themeFor(name string, dark bool) *ui.Theme {
	if name == "dark" || (name == "system" && dark) {
		return darkTheme
	}
	return lightTheme
}

func systemDark() bool { return mygo.Theme.IsDark() }

func themeLabel(name string) string {
	switch name {
	case "dark":
		return i18n.T("深色")
	case "system":
		return i18n.T("跟随系统")
	}
	return i18n.T("浅色")
}
