// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
)

// The view is a pure function of state, so it renders without a window: this
// catches the mistakes that would otherwise only appear when a user opens the
// app, such as a panic in a nil callback or a label that never changes.
func TestViewRendersEveryPhase(t *testing.T) {
	phases := []session.Phase{
		session.PhaseIdle,
		session.PhaseConnecting,
		session.PhaseOnline,
		session.PhaseFailed,
	}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			state := State{
				Session: session.State{
					Phase:    phase,
					Detail:   "detail for " + string(phase),
					ClientIP: "192.0.2.10",
				},
				Config: config.Default(),
			}

			tester := ui.NewTester(View(&state, Actions{}), 520, 560)
			tester.Frame()

			if texts := tester.Texts(); len(texts) == 0 {
				t.Fatal("the view rendered no text")
			}
			if !tester.HasText("goTrust") {
				t.Fatal("the title is missing")
			}
			if !tester.HasText(phaseLabel(state.Session)) {
				t.Fatalf("the phase label %q is missing", phaseLabel(state.Session))
			}
			if !tester.HasText("配置") {
				t.Fatal("the active profile is missing")
			}
		})
	}
}

// The main window names the active account and where it connects, because the
// credentials now live in the settings window.
func TestProfileSummaryShowsTheActiveAccount(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	cfg.Profiles[0].Auth.Username = "alice"

	state := State{
		Session: session.State{Phase: session.PhaseIdle, Detail: "未连接"},
		Config:  cfg,
	}

	tester := ui.NewTester(View(&state, Actions{}), 520, 560)
	tester.Frame()

	if !tester.HasText("配置 默认 ｜ vpn.example.com:443") {
		t.Fatalf("profile summary missing; texts: %v", tester.Texts())
	}
	if tester.HasText("hunter2") {
		t.Fatal("a password is painted on the main window")
	}
}

// Clicking 连接 must reach the action, and clicking 断开 in the online phase
// must reach Logout instead. A swapped wiring would lock the user out.
func TestButtonsCallTheRightAction(t *testing.T) {
	cases := []struct {
		phase      session.Phase
		button     string
		wantLogin  bool
		wantLogout bool
	}{
		{phase: session.PhaseIdle, button: "连接", wantLogin: true},
		{phase: session.PhaseFailed, button: "连接", wantLogin: true},
		{phase: session.PhaseOnline, button: "断开", wantLogout: true},
		{phase: session.PhaseConnecting, button: "断开", wantLogout: true},
	}

	for _, testCase := range cases {
		t.Run(string(testCase.phase), func(t *testing.T) {
			var login, logout int
			state := State{
				Session: session.State{Phase: testCase.phase, Detail: "x"},
				Config:  config.Default(),
			}
			actions := Actions{
				Login:  func() { login++ },
				Logout: func() { logout++ },
			}

			tester := ui.NewTester(View(&state, actions), 520, 560)
			if err := tester.Click(testCase.button); err != nil {
				t.Fatalf("click %q: %v", testCase.button, err)
			}

			if testCase.wantLogin && login != 1 {
				t.Fatalf("login calls = %d, want 1", login)
			}
			if testCase.wantLogout && logout != 1 {
				t.Fatalf("logout calls = %d, want 1", logout)
			}
		})
	}
}

func TestSettingsButtonOpensTheWindow(t *testing.T) {
	opened := 0
	state := State{
		Session: session.State{Phase: session.PhaseIdle, Detail: "x"},
		Config:  config.Default(),
	}
	tester := ui.NewTester(View(&state, Actions{Settings: func() { opened++ }}), 520, 560)
	if err := tester.Click("设置"); err != nil {
		t.Fatalf("click settings: %v", err)
	}
	if opened != 1 {
		t.Fatalf("settings calls = %d, want 1", opened)
	}
}

// The listener addresses are what the user copies into another application, so
// the view has to show every one of them.
func TestAddressesAndKeepAliveAreListed(t *testing.T) {
	state := State{
		Session:   session.State{Phase: session.PhaseOnline, Detail: "vpn.example.com:443"},
		Config:    config.Default(),
		Addresses: []string{"127.0.0.1:1080", "127.0.0.1:1081"},
		KeepAlive: "OK 200 · http://192.0.2.10/",
	}

	tester := ui.NewTester(View(&state, Actions{}), 520, 560)
	tester.Frame()

	for _, address := range state.Addresses {
		if !tester.HasText("代理监听 " + address) {
			t.Fatalf("address %q is missing from the view", address)
		}
	}
	if !tester.HasText("保活 OK 200") {
		t.Fatal("the keep-alive result is missing")
	}
}

func TestTrayStatus(t *testing.T) {
	cases := []struct {
		state session.State
		want  string
	}{
		{session.State{Phase: session.PhaseIdle}, "未连接"},
		{session.State{Phase: session.PhaseConnecting}, "连接中"},
		{session.State{Phase: session.PhaseOnline, ClientIP: "203.0.113.4"}, "已连接 203.0.113.4"},
		{session.State{Phase: session.PhaseOnline, Detail: "vpn.example.com:443"}, "已连接 vpn.example.com:443"},
		{session.State{Phase: session.PhaseFailed, Detail: "bad password"}, "失败: bad password"},
		{session.State{Phase: session.PhaseFailed, Detail: strings.Repeat("x", 80)}, "失败: "},
	}
	for _, testCase := range cases {
		got := TrayStatus(testCase.state)
		if testCase.want == "失败: " {
			if len([]rune(got)) > 60 {
				t.Fatalf("a long failure was not truncated: %q", got)
			}
			continue
		}
		if got != testCase.want {
			t.Fatalf("status = %q, want %q", got, testCase.want)
		}
	}
}

// The tray menu is the only control surface when the window is hidden.
func TestTrayMenuStatusProfilesAndToggle(t *testing.T) {
	cfg := config.Default()
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	if err := cfg.SelectProfile(config.DefaultProfileName); err != nil {
		t.Fatalf("select: %v", err)
	}

	var login, logout int
	var selected string
	state := State{
		Session: session.State{Phase: session.PhaseOnline, Detail: "vpn.example.com:443", ClientIP: "203.0.113.4"},
		Config:  cfg,
	}
	menu := TrayMenu(&state, Actions{
		Login:         func() { login++ },
		Logout:        func() { logout++ },
		SelectProfile: func(name string) { selected = name },
	})

	items := menu.Items()
	if len(items) == 0 {
		t.Fatal("empty tray menu")
	}
	if items[0].Label != "已连接 203.0.113.4" {
		t.Fatalf("status line = %q", items[0].Label)
	}
	if !items[0].Disabled {
		t.Fatal("the status line is clickable")
	}

	var toggle, profiles *mygo.MenuItem
	for _, item := range items {
		switch item.Label {
		case "断开":
			toggle = item
		case "配置":
			profiles = item
		}
	}
	if toggle == nil {
		t.Fatalf("no 断开 toggle in %v", labels(items))
	}
	toggle.Click(toggle, nil)
	if logout != 1 || login != 0 {
		t.Fatalf("toggle did not disconnect: login=%d logout=%d", login, logout)
	}

	if profiles == nil {
		t.Fatalf("no profile submenu in %v", labels(items))
	}
	if len(profiles.Submenu) != 2 {
		t.Fatalf("profile items = %d, want 2", len(profiles.Submenu))
	}
	if !profiles.Submenu[0].Checked || profiles.Submenu[1].Checked {
		t.Fatal("the active profile is not the checked one")
	}
	profiles.Submenu[1].Click(profiles.Submenu[1], nil)
	if selected != "工作" {
		t.Fatalf("profile click selected %q", selected)
	}
}

func TestTrayMenuOffersConnectWhenIdle(t *testing.T) {
	var login int
	state := State{
		Session: session.State{Phase: session.PhaseIdle},
		Config:  config.Default(),
	}
	menu := TrayMenu(&state, Actions{Login: func() { login++ }})
	var toggle *mygo.MenuItem
	for _, item := range menu.Items() {
		if item.Label == "连接" {
			toggle = item
		}
	}
	if toggle == nil {
		t.Fatalf("no 连接 toggle in %v", labels(menu.Items()))
	}
	toggle.Click(toggle, nil)
	if login != 1 {
		t.Fatalf("login calls = %d", login)
	}
}

func labels(items []*mygo.MenuItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Label)
	}
	return out
}
