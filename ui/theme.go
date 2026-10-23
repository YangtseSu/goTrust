// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/session"
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
	return ui.Text(c, text).FontSize(textSize(c, captionScale)).TextColor(toneColor(c.Theme(), t))
}

// statusChip is the phase as a coloured pill next to the window title. The
// colour is the state's, so the eye finds the one changing element first.
func statusChip(c *ui.Context, state *State) {
	label := phaseLabel(state.Session)
	color := toneColor(c.Theme(), phaseTone(state.Session.Phase))
	ui.Row(c).AlignItems(ui.Center).Gap(6).PaddingX(10).PaddingY(4).
		Radius(999).Background(color.Alpha(0.12)).
		Role(ui.RoleStatus).Label("状态：" + label).
		Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(color)
			ui.Text(c, label).FontSize(textSize(c, captionScale)).FontWeight(600).TextColor(color)
		})
}

// busyLine is the visible sign that a login or a logout is running: a spinner
// and a word, so a slow gateway does not look like a frozen window.
func busyLine(c *ui.Context) {
	ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
		ui.Spinner(c).Label("正在处理")
		caption(c, "处理中…")
	})
}
