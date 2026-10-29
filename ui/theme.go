// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
)

// The interface has four text sizes and one spacing grid, so every window looks
// like the same application. Sizes are derived from the theme's own font size:
// a desktop that asks for larger text gets larger headings and captions too,
// and the ratio between them stays fixed.
const (
	titleScale   = 1.5  // the window title
	panelScale   = 1.15 // the heading of a pre-login panel
	captionScale = 0.85 // secondary lines, hints and status details

	windowPadding = 24 // the frame around a window's content
	sectionGap    = 16 // between the groups of a window
	rowGap        = 8  // between the parts of one group
)

// palette is the colour set the mockup draws the window with. mygo's Theme
// carries the control colours; the extra surfaces (the window, the sidebar,
// the inputs' fill and the log pane) are the app's own, so they live here.
type palette struct {
	content ui.Color
	surface ui.Color
	sidebar ui.Color
	hover   ui.Color
	border  ui.Color
	text    ui.Color
	muted   ui.Color

	accent     ui.Color
	accentWeak ui.Color

	success ui.Color
	warning ui.Color
	danger  ui.Color

	switchOff ui.Color
	logBG     ui.Color

	powerOn   ui.Color
	powerWait ui.Color
	powerFail ui.Color
}

func paletteFor(dark bool) palette {
	if dark {
		return palette{
			content: ui.Hex("#1e2025"),
			surface: ui.Hex("#23262c"),
			sidebar: ui.Hex("#1a1c21"),
			hover:   ui.Hex("#2b2f37"),
			border:  ui.Hex("#32363f"),
			text:    ui.Hex("#e7e9ee"),
			muted:   ui.Hex("#9aa1ad"),

			accent:     ui.Hex("#77a8fb"),
			accentWeak: ui.Hex("#1f2b41"),

			success: ui.Hex("#5cc98a"),
			warning: ui.Hex("#e5a54b"),
			danger:  ui.Hex("#f0736a"),

			switchOff: ui.Hex("#3a3f49"),
			logBG:     ui.Hex("#15171b"),

			powerOn:   ui.Hex("#16a34a"),
			powerWait: ui.Hex("#d97706"),
			powerFail: ui.Hex("#dc2626"),
		}
	}
	return palette{
		content: ui.Hex("#fbfbfc"),
		surface: ui.Hex("#ffffff"),
		sidebar: ui.Hex("#f1f3f6"),
		hover:   ui.Hex("#eceef2"),
		border:  ui.Hex("#e2e5ea"),
		text:    ui.Hex("#1f2328"),
		muted:   ui.Hex("#6b7280"),

		accent:     ui.Hex("#2563eb"),
		accentWeak: ui.Hex("#e8effe"),

		success: ui.Hex("#15803d"),
		warning: ui.Hex("#b45309"),
		danger:  ui.Hex("#b42318"),

		switchOff: ui.Hex("#d3d7de"),
		logBG:     ui.Hex("#f7f8fa"),

		powerOn:   ui.Hex("#16a34a"),
		powerWait: ui.Hex("#d97706"),
		powerFail: ui.Hex("#dc2626"),
	}
}

// palette is the mockup's colours for the appearance the frame uses.
func paletteOf(c *ui.Context) palette { return paletteFor(c.Theme().Dark) }

// themeFor is the mygo theme the frame draws with: the mockup's colours on
// the toolkit's metrics, so mygo's own widgets (inputs, switches, selects,
// scrollbars) sit in the same palette as the views.
func themeFor(dark bool) *ui.Theme {
	theme := ui.LightTheme()
	if dark {
		theme = ui.DarkTheme()
	}
	p := paletteFor(dark)
	theme.Background = p.content
	theme.Surface = p.surface
	theme.SurfaceHover = p.hover
	theme.SurfacePressed = p.hover.Mix(p.text, 0.08)
	theme.Border = p.border
	theme.Text = p.text
	theme.TextMuted = p.muted
	theme.Accent = p.accent
	theme.AccentHover = p.accent.Mix(p.text, 0.12)
	theme.AccentPressed = p.accent.Mix(p.text, 0.2)
	theme.AccentText = ui.RGB(255, 255, 255)
	theme.Danger = p.danger
	theme.Warning = p.warning
	theme.Success = p.success
	theme.Selection = p.accent.Alpha(0.25)
	theme.Focus = p.accent.Alpha(0.55)
	theme.Inverse = p.text
	theme.InverseText = p.content
	return theme
}

// tone is the meaning of a colour, not the colour itself: the theme decides
// what success and failure look like in the light and in the dark.
type tone uint8

const (
	toneNeutral tone = iota
	toneAccent
	toneSuccess
	toneWarning
	toneDanger
)

// phaseTone maps a session phase to the colour that reports it. Connecting is
// work in progress, reconnecting is a warning because the session is gone, and
// a failure is the only red state.
func phaseTone(phase session.Phase) tone {
	switch phase {
	case session.PhaseOnline:
		return toneSuccess
	case session.PhaseConnecting:
		return toneAccent
	case session.PhaseReconnecting:
		return toneWarning
	case session.PhaseFailed:
		return toneDanger
	default:
		return toneNeutral
	}
}

// toneColor is the theme's own colour for a tone. These are Meter fills, so
// they are surfaces and the source toneInk walks away from; the readable mark
// is toneInk.
func toneColor(theme *ui.Theme, t tone) ui.Color {
	switch t {
	case toneAccent:
		return theme.Accent
	case toneSuccess:
		return theme.Success
	case toneWarning:
		return theme.Warning
	case toneDanger:
		return theme.Danger
	default:
		return theme.TextMuted
	}
}

// contrastFloor is WCAG AA for body text (4.5:1) with a margin: the ink is
// mixed in 5% steps, so the first step that clears the threshold would
// otherwise sit on it.
const contrastFloor = 4.8

// toneInk is a tone's colour when it has to be read, as text or as the chip's
// dot on the chip's own tint. The semantic fills measure 2.81-4.37:1 at
// caption size, so the ink is the fill moved toward the far end of the
// appearance until it clears contrastFloor. Every use of a tone as a mark
// goes through this; the raw fill stays a surface.
func toneInk(theme *ui.Theme, t tone) ui.Color {
	fill := toneColor(theme, t)
	away := ui.RGB(0, 0, 0)
	if theme.Dark {
		away = ui.RGB(255, 255, 255)
	}
	for step := range 21 {
		ink := fill
		if step > 0 {
			ink = fill.Mix(away, float32(step)*0.05)
		}
		if contrastRatio(ink, chipSurface(theme, ink)) >= contrastFloor {
			return ink
		}
	}
	// The theme's own text colour always reads on its background; a fill that
	// cannot be read at any mix has no business carrying meaning.
	return theme.Text
}

// chipSurface is the pill behind a status label: the ink at 12% over the
// window.
func chipSurface(theme *ui.Theme, ink ui.Color) ui.Color {
	return ink.Alpha(0.12).Over(theme.Background)
}

// contrastRatio is WCAG 2.1's ratio between two colours composited over an
// opaque background.
func contrastRatio(a, b ui.Color) float64 {
	first, second := luminance(a), luminance(b)
	if first < second {
		first, second = second, first
	}
	return (first + 0.05) / (second + 0.05)
}

// luminance is WCAG 2.1's relative luminance of an sRGB colour.
func luminance(c ui.Color) float64 {
	linear := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

// textSize returns a size on the scale, rounded to half a point so the
// rasterizer does not have to guess at fractional pixels.
func textSize(c *ui.Context, scale float32) float32 {
	base := c.Theme().FontSize
	if base <= 0 {
		base = 14
	}
	return float32(math.Round(float64(base*scale)*2)) / 2
}

// windowHeading is the one line at the top of a window.
func windowHeading(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(textSize(c, titleScale)).Bold().Role(ui.RoleHeading).Level(1)
}

// panelHeading names the question a pre-login panel asks.
func panelHeading(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(textSize(c, panelScale)).Bold().Role(ui.RoleHeading).Level(2)
}

// formColumn lays a card's fields out top to bottom with the mockup's 14 DIP
// between them.
func formColumn(c *ui.Context, children func()) {
	ui.Column(c).Gap(c.Theme().Space(3.5)).Children(children)
}

// formField is one field of a card: its label above the control, muted and at
// caption size, as the mockup draws it.
func formField(c *ui.Context, label string, children func()) {
	ui.Column(c).Gap(c.Theme().Space(1.5)).Children(func() {
		if label != "" {
			ui.Text(c, label).FontSize(textSize(c, captionScale)).TextColor(c.Theme().TextMuted)
		}
		children()
	})
}

// sectionHeading names a group inside a window.
func sectionHeading(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(c.Theme().FontSize).FontWeight(600).Role(ui.RoleHeading).Level(2)
}

// caption renders a secondary line: a hint, an address, a timestamp. It is
// quieter than the thing it explains, never the other way round.
func caption(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(textSize(c, captionScale)).TextColor(c.Theme().TextMuted)
}

// captionTone is a secondary line that also reports a state, such as a failed
// keep-alive probe.
func captionTone(c *ui.Context, text string, t tone) ui.Element {
	return ui.Text(c, text).FontSize(textSize(c, captionScale)).TextColor(toneInk(c.Theme(), t))
}

// textButton is a button that keeps only its label: a secondary action in a
// panel (撤销, 清空) stays quieter than the filled primary button, and the
// pointer tints the padding so the target is still visible. It keeps the
// vertical room of a filled button, so a row that mixes the two stays on one
// line.
func textButton(c *ui.Context, label string) ui.Element {
	theme := c.Theme()
	button := ui.ButtonBase(c).Padding(theme.Space(1.5), theme.Space(2)).Radius(theme.Radius)
	if button.Hovered() {
		button.Background(theme.SurfaceHover)
	}
	return button.Children(func() {
		ui.Text(c, label).FontSize(theme.FontSize).TextColor(theme.Text)
	})
}

// card is a group of related content on its own face: the dashboard shows the
// session and the resources as two cards, so each reads as one object instead
// of every row sharing the page's background. The fill is the window's, so a
// control inside the card keeps the theme's own button face; the border is
// what draws the card.
func card(c *ui.Context) ui.Element {
	theme := c.Theme()
	p := paletteOf(c)
	return ui.Column(c).Padding(theme.Space(4)).Radius(theme.Radius*2).
		Border(1, p.border).Background(p.surface)
}

// busyLine is the visible sign that a save is running: a spinner and a word, so
// a slow disk does not look like a frozen window. The dashboard's control says
// the same for a login through its own spinner.
func busyLine(c *ui.Context) {
	ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
		ui.Spinner(c).Label(locales.BusyLabel())
		caption(c, locales.BusyText())
	})
}
