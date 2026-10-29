// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
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
// and stays usable, because 取消 is the only way out of a pending login.
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

// Tab reaches the navigation first, then the session control. The nav items
// are tabs for assistive technology, so this is also the order they expose.
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
		"仪表盘", "账户", "代理", "保活", "启动", "终端管理", "日志", "设置", "关于", "多账户",
		"连接",
	} {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if !tester.Focused(want) {
			t.Fatalf("Tab did not reach %q", want)
		}
	}
}

// The save bar is the last stop of the window: Tab reaches 放弃修改 and then
// 保存, whatever the page's form contains. Both are absent while the draft
// matches the file, so the draft is edited first.
func TestSettingsTabOrderReachesTheSaveBar(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Gateways[0] = "vpn.example.com:8443"
	_, tester := newShell(t, state, SettingsActions{}, PageAccount)

	found := false
	for range 80 {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if tester.Focused("放弃修改") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Tab never reached 放弃修改")
	}

	tester.Key(0, ui.KeyTab)
	tester.Frame()
	if !tester.Focused("保存") {
		t.Fatal("Tab after 放弃修改 did not reach 保存")
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
