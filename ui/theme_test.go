// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// themeFor starts from the frame's theme, which has already followed the
// desktop's preferences, so the text scale survives the mockup's palette.
func TestThemeForKeepsTheDesktopTextScale(t *testing.T) {
	var got float32
	tester := ui.NewTester(func(c *ui.Context) {
		got = themeFor(c).FontSize
	}, 400, 300)
	tester.SetPreferences(ui.Preferences{TextScale: 1.5})

	if want := ui.LightTheme().FontSize * 1.5; got != want {
		t.Fatalf("FontSize = %v, want %v", got, want)
	}
}

// The palette replaces the colours Theme.follow mixed, so themeFor has to
// apply the high-contrast adjustment to its own values.
func TestThemeForReappliesHighContrast(t *testing.T) {
	p := paletteFor(false)

	var got ui.Theme
	tester := ui.NewTester(func(c *ui.Context) {
		got = *themeFor(c)
	}, 400, 300)
	tester.SetPreferences(ui.Preferences{TextScale: 1, HighContrast: true})

	if want := p.border.Mix(p.text, 0.45); got.Border != want {
		t.Fatalf("Border = %+v, want %+v", got.Border, want)
	}
	if want := p.muted.Mix(p.text, 0.4); got.TextMuted != want {
		t.Fatalf("TextMuted = %+v, want %+v", got.TextMuted, want)
	}
	if got.Focus.A != 255 {
		t.Fatalf("Focus alpha = %d, want 255", got.Focus.A)
	}
}

// Without high contrast the palette is used as drawn.
func TestThemeForUsesThePaletteByDefault(t *testing.T) {
	p := paletteFor(false)

	var got ui.Theme
	ui.NewTester(func(c *ui.Context) {
		got = *themeFor(c)
	}, 400, 300)

	if got.Border != p.border || got.TextMuted != p.muted {
		t.Fatalf("Border/TextMuted = %+v/%+v, want the palette's %+v/%+v",
			got.Border, got.TextMuted, p.border, p.muted)
	}
}
