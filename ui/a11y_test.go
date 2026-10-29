// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
)

// Every phase has one colour meaning, and a failure is the only red one.
func TestPhaseTone(t *testing.T) {
	cases := []struct {
		phase session.Phase
		want  tone
	}{
		{session.PhaseIdle, toneNeutral},
		{session.PhaseConnecting, toneAccent},
		{session.PhaseOnline, toneSuccess},
		{session.PhaseReconnecting, toneWarning},
		{session.PhaseFailed, toneDanger},
	}
	for _, testCase := range cases {
		if got := phaseTone(testCase.phase); got != testCase.want {
			t.Fatalf("phase %s = tone %d, want %d", testCase.phase, got, testCase.want)
		}
	}
}

// A tone is read at caption size, so every tone that carries a mark has to
// clear WCAG AA in both appearances, on the window and on the chip's own
// tint. The chip's label is the theme's text colour on that tint, so it is
// asserted with the same surfaces.
func TestToneInkPassesAA(t *testing.T) {
	appearances := map[string]*ui.Theme{"light": ui.LightTheme(), "dark": ui.DarkTheme()}
	tones := []tone{toneNeutral, toneAccent, toneSuccess, toneWarning, toneDanger}
	for appearance, theme := range appearances {
		for _, tn := range tones {
			ink := toneInk(theme, tn)
			tint := chipSurface(theme, ink)
			if got := contrastRatio(ink, tint); got < 4.5 {
				t.Errorf("%s tone %d: ink on the chip tint = %.2f:1, want >= 4.5", appearance, tn, got)
			}
			if got := contrastRatio(ink, theme.Background); got < 4.5 {
				t.Errorf("%s tone %d: ink on the window = %.2f:1, want >= 4.5", appearance, tn, got)
			}
			if got := contrastRatio(theme.Text, tint); got < 4.5 {
				t.Errorf("%s tone %d: label on the chip tint = %.2f:1, want >= 4.5", appearance, tn, got)
			}
		}
	}
}

// Starting a login twice would race two sessions, so the power control disables
// a start while another action runs. The control is the only place the state
// speaks, so a busy button is simply unavailable.
func TestBusyStateDisablesConnect(t *testing.T) {
	var logins int
	state := State{
		Session: session.State{Phase: session.PhaseIdle, Detail: "未连接"},
		Config:  config.Default(),
		Busy:    true,
	}
	tester := ui.NewTester(View(&state, Actions{Login: func() { logins++ }}), 720, 560)
	tester.Frame()

	if err := tester.Click("连接"); err != nil {
		t.Fatalf("click 连接: %v", err)
	}
	if logins != 0 {
		t.Fatalf("the disabled button started %d logins", logins)
	}
}

// A login in flight is a session to end: the same control shows the spinner
// and stays usable, because Cancel is the only way out of a pending login.
func TestBusyStateKeepsDisconnect(t *testing.T) {
	var logouts int
	state := State{
		Session: session.State{Phase: session.PhaseConnecting, Detail: "正在登录"},
		Config:  config.Default(),
		Busy:    true,
	}
	tester := ui.NewTester(View(&state, Actions{Logout: func() { logouts++ }}), 720, 560)
	if err := tester.Click("取消"); err != nil {
		t.Fatalf("click 取消: %v", err)
	}
	if logouts != 1 {
		t.Fatalf("logout calls = %d, want 1", logouts)
	}
}

// The window's Tab order: the appearance switch, the three window controls,
// the sidebar as one stop (entered at the page on screen), then the
// dashboard's power control. The nav items are tabs for assistive technology,
// so this is also the order they expose.
func TestMainWindowTabOrder(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseIdle, Detail: "未连接"},
		Config:  config.Default(),
	}
	tester := ui.NewTester(View(&state, Actions{}), 720, 560)
	tester.Frame()

	for _, want := range []string{
		locales.ThemeSwitchToDark(),
		locales.WindowMinimize(), locales.WindowMaximize(), locales.WindowClose(),
		"仪表盘",
		"连接",
	} {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if !tester.Focused(want) {
			t.Fatalf("Tab did not reach %q", want)
		}
	}
}

// The sidebar is a vertical tab list: Tab enters it once, at the page on
// screen, Down moves the focus among the rows without choosing one, and Enter
// opens the page under the focus.
func TestSidebarArrowsMoveTheFocusAndEnterOpensThePage(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseIdle, Detail: "未连接"},
		Config:  config.Default(),
	}
	var shown *Page
	tester := ui.NewTester(View(&state, Actions{
		ShowPage: func(page Page) { shown = &page },
	}), 720, 560)

	for range 10 {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if tester.Focused("仪表盘") {
			break
		}
	}
	if !tester.Focused("仪表盘") {
		t.Fatal("Tab never entered the sidebar")
	}

	tester.Key(0, ui.KeyDown)
	tester.Frame()
	if !tester.Focused("账户") {
		t.Fatal("Down did not move the focus to the next tab")
	}
	if shown != nil {
		t.Fatalf("Down opened %q", pageLabel(*shown))
	}

	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if shown == nil || *shown != PageAccount {
		t.Fatalf("Enter opened %v, want 账户", shown)
	}
}

// Tab enters the tab list at the chosen page, not at its first row.
func TestSidebarTabEntersAtTheChosenPage(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseIdle, Detail: "未连接"},
		Config:  config.Default(),
		Page:    PageSettings,
	}
	tester := ui.NewTester(View(&state, Actions{}), 720, 560)

	for range 10 {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if tester.Focused("设置") {
			return
		}
	}
	t.Fatal("Tab did not enter the sidebar at 设置")
}

// The save bar is the last stop of the window: Tab reaches Discard and then
// Save, whatever the page's form contains. Both are absent while the draft
// matches the file, so the draft is edited first.
func TestSettingsTabOrderReachesTheSaveBar(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Gateways[0] = "vpn.example.com:8443"
	_, tester := newShell(t, state, SettingsActions{}, PageAccount)

	found := false
	for range 80 {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if tester.Focused("放弃") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Tab never reached 放弃")
	}

	tester.Key(0, ui.KeyTab)
	tester.Frame()
	if !tester.Focused("保存") {
		t.Fatal("Tab after 放弃 did not reach 保存")
	}
}

// An action's result is reported once, as a toast: the window keeps no line
// for it, so the resources keep their room, and the toast goes away by itself.
func TestActionResultsShowAsToasts(t *testing.T) {
	state := State{
		Session:     session.State{Phase: session.PhaseIdle, Detail: "未连接"},
		Config:      config.Default(),
		Status:      "打开失败: no browser",
		StatusSeq:   1,
		StatusError: true,
	}
	tester := ui.NewTester(View(&state, Actions{}), 720, 560)
	tester.Frame()
	tester.Advance(250 * time.Millisecond)
	tester.Frame()
	if !tester.HasText("打开失败: no browser") {
		t.Fatalf("the failure is missing; texts: %v", tester.Texts())
	}

	tester.Advance(5 * time.Second)
	tester.Frame()
	if tester.HasText("打开失败: no browser") {
		t.Fatal("the toast outlived its timeout")
	}
}

// changedPixels counts the pixels of before and after that differ inside r.
func changedPixels(before, after *image.RGBA, r ui.Rect) int {
	changed := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if !image.Pt(x, y).In(before.Bounds()) || !image.Pt(x, y).In(after.Bounds()) {
				continue
			}
			if before.RGBAAt(x, y) != after.RGBAAt(x, y) {
				changed++
			}
		}
	}
	return changed
}

// A focused icon-only control shows its label as a tooltip above itself.
// Texts() carries accessible labels too, so the tooltip is checked on the
// frame itself: the band above the power control is painted only when the tip
// shows, outside the focus ring.
func TestPowerControlShowsItsTooltipOnFocus(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseIdle, Detail: "未连接"},
		Config:  config.Default(),
	}
	tester := ui.NewTester(View(&state, Actions{}), 720, 560)
	tester.Frame()
	control, ok := tester.Find(locales.Connect())
	if !ok {
		t.Fatal("the power control was not found")
	}
	before := tester.Image()

	for range 10 {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if tester.Focused(locales.Connect()) {
			break
		}
	}
	if !tester.Focused(locales.Connect()) {
		t.Fatal("Tab never reached the power control")
	}
	after := tester.Image()

	// The tip sits 6 DIP above the control; the band ends above the focus ring.
	band := ui.Rect{X: control.X - 40, Y: control.Y - 40, W: control.W + 80, H: 30}
	if changed := changedPixels(before, after, band); changed == 0 {
		t.Fatal("no tooltip was painted above the focused power control")
	}
}

// The row's menu glyph is a menu button: Tab reaches it and Enter opens the
// menu, as it does for a focused menu button.
func TestResourceRowMenuIsReachableFromTheKeyboard(t *testing.T) {
	state := State{
		Session: session.State{
			Phase:    session.PhaseOnline,
			Detail:   "vpn.example.com:443",
			ClientIP: "192.0.2.10",
			Resources: session.Resources{Entries: []session.ResourceEntry{
				{Kind: session.ResourceApp, Title: "Wiki", Detail: "Portal · web", URL: "https://wiki.example.com", Copy: "https://wiki.example.com"},
			}},
		},
		Config: config.Default(),
	}
	label := locales.ResourceMenu("Wiki")
	tester := ui.NewTester(View(&state, Actions{}), 900, 740)

	for range 20 {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if tester.Focused(label) {
			break
		}
	}
	if !tester.Focused(label) {
		t.Fatal("Tab never reached the row menu")
	}

	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := tester.Menu(); len(got) != 2 || got[0] != locales.CommonOpen() || got[1] != locales.CommonCopy() {
		t.Fatalf("menu = %v, want 打开 and 复制", got)
	}
}
