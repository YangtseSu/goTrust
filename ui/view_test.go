// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/proxy"
	"github.com/YangtseSu/goTrust/core/session"
)

// The view is a pure function of state, so it renders without a window: this
// catches the mistakes that would otherwise only appear when a user opens the
// app, such as a panic in a nil callback or a label that never changes.
func TestViewRendersEveryPhase(t *testing.T) {
	phases := []session.Phase{
		session.PhaseIdle,
		session.PhaseConnecting,
		session.PhaseOnline,
		session.PhaseReconnecting,
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

			tester := ui.NewTester(View(&state, Actions{}), 720, 560)
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
			if !tester.HasText(config.DefaultProfileName) {
				t.Fatal("the active account is missing")
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

	tester := ui.NewTester(View(&state, Actions{}), 720, 560)
	tester.Frame()

	if !tester.HasText("goTrust · 账户 默认") {
		t.Fatalf("the active account is missing from the title bar; texts: %v", tester.Texts())
	}
	if !tester.HasText("vpn.example.com:443") {
		t.Fatalf("the endpoint is missing from the subline; texts: %v", tester.Texts())
	}
	if tester.HasText("hunter2") {
		t.Fatal("a password is painted on the main window")
	}
}

// Clicking 连接 must reach the action, and clicking 断开 (or 取消) in a live
// phase must reach Logout instead. A swapped wiring would lock the user out.
func TestButtonsCallTheRightAction(t *testing.T) {
	cases := []struct {
		phase      session.Phase
		button     string
		wantLogin  bool
		wantLogout bool
	}{
		{phase: session.PhaseIdle, button: "连接", wantLogin: true},
		{phase: session.PhaseFailed, button: "重试", wantLogin: true},
		{phase: session.PhaseOnline, button: "断开", wantLogout: true},
		// 取消 is how a login waiting on the network or a prompt is stopped.
		{phase: session.PhaseConnecting, button: "取消", wantLogout: true},
		{phase: session.PhaseReconnecting, button: "取消", wantLogout: true},
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

			tester := ui.NewTester(View(&state, actions), 720, 560)
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

// The power control is the state: every phase has one label, one glyph and one
// fill, and the two waiting phases turn a spinner instead of a glyph. Offline
// and failed start a login; the other three end it, so hovering warns in red.
func TestPowerControlShowsThePhase(t *testing.T) {
	theme := ui.LightTheme()
	cases := []struct {
		phase  session.Phase
		label  string
		glyph  *ui.SVG
		fill   ui.Color
		ending bool
	}{
		{phase: session.PhaseIdle, label: "连接", glyph: iconPowerPlay, fill: theme.Accent},
		{phase: session.PhaseConnecting, label: "取消", fill: theme.Warning, ending: true},
		{phase: session.PhaseOnline, label: "断开", glyph: iconPowerStop, fill: theme.Success, ending: true},
		{phase: session.PhaseReconnecting, label: "取消", fill: theme.Warning, ending: true},
		{phase: session.PhaseFailed, label: "重试", glyph: iconPowerFail, fill: theme.Danger},
	}
	for _, testCase := range cases {
		look := powerAppearance(theme, testCase.phase)
		if look.label != testCase.label {
			t.Fatalf("phase %s label = %q, want %q", testCase.phase, look.label, testCase.label)
		}
		if look.glyph != testCase.glyph {
			t.Fatalf("phase %s glyph = %v, want %v", testCase.phase, look.glyph, testCase.glyph)
		}
		if look.fill != testCase.fill {
			t.Fatalf("phase %s fill = %v, want %v", testCase.phase, look.fill, testCase.fill)
		}
		if look.ending != testCase.ending {
			t.Fatalf("phase %s ending = %v, want %v", testCase.phase, look.ending, testCase.ending)
		}
	}
}

// Quit and the settings pages belong to the tray and the sidebar; the window
// has no second overflow menu duplicating them, and Ctrl+, is gone.
func TestWindowHasNoOverflowMenuOrSettingsShortcut(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseIdle, Detail: "x"},
		Config:  config.Default(),
	}
	page := PageDashboard
	tester := ui.NewTester(View(&state, Actions{ShowPage: func(chosen Page) { page = chosen }}), 720, 560)
	tester.Frame()
	if tester.HasText("⋯") {
		t.Fatal("the window still has an overflow menu")
	}
	tester.Key(ui.Cmd, ui.KeyComma)
	if page != PageDashboard {
		t.Fatal("Ctrl+, still selects a page")
	}
}

// The window is one surface: one title strip, the navigation, and the page.
func TestShellIsOneSurface(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseIdle},
		Config:  config.Default(),
	}
	tester := ui.NewTester(View(&state, Actions{}), 900, 740)
	tester.Frame()

	titles := 0
	for _, text := range tester.Texts() {
		if strings.HasPrefix(text, "goTrust") {
			titles++
		}
	}
	if titles != 1 {
		t.Fatalf("title strips = %d, want 1; texts: %v", titles, tester.Texts())
	}
	for _, label := range []string{"仪表盘", "账户", "代理", "保活", "启动", "终端管理", "日志", "设置", "多账户"} {
		if !tester.HasText(label) {
			t.Fatalf("the navigation is missing %q", label)
		}
	}
}

// The bottom item names the live account, so every page says which account an
// action applies to. The name is user data and is not translated: the tag is
// the bare name, inside the sidebar and not the page.
func TestSidebarTagsTheLiveAccount(t *testing.T) {
	cfg := config.Default()
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add account: %v", err)
	}
	cfg.ActiveProfile = "工作"
	state := State{
		Session: session.State{Phase: session.PhaseIdle},
		Config:  cfg,
	}
	tester := ui.NewTester(View(&state, Actions{}), 900, 740)
	tester.Frame()

	tag, ok := tester.Find("工作")
	if !ok {
		t.Fatalf("the live account is not tagged; texts: %v", tester.Texts())
	}
	if tag.X+tag.W > sidebarWidth {
		t.Fatalf("the tag at %+v is outside the %v wide sidebar", tag, sidebarWidth)
	}
}

// The measurements are one row of four cells: the address the gateway
// assigned, the two listeners and the last probe. A value the phase has not
// produced is a dash, and the probe's own result stays in the description
// instead of taking a second line.
func TestSessionMetricsShowTheSessionFacts(t *testing.T) {
	keepAlive := session.KeepAliveState{Target: "http://192.0.2.10/", Status: 200, OK: true, At: time.Now()}
	state := State{
		Session: session.State{
			Phase:     session.PhaseOnline,
			Detail:    "vpn.example.com:443",
			ClientIP:  "203.0.113.4",
			KeepAlive: keepAlive,
		},
		Config: config.Default(),
		Listeners: []proxy.Listener{
			{Protocol: proxy.ProtocolSOCKS5, Address: "127.0.0.1:1080"},
			{Protocol: proxy.ProtocolHTTP, Address: "127.0.0.1:1081"},
		},
	}

	tester := ui.NewTester(View(&state, Actions{}), 720, 560)
	tester.Frame()

	for _, want := range []string{
		"已分配地址", "203.0.113.4",
		"SOCKS5", "127.0.0.1:1080",
		"HTTP", "127.0.0.1:1081",
		"保活", "0 秒前",
	} {
		if !tester.HasText(want) {
			t.Fatalf("the metric %q is missing; texts: %v", want, tester.Texts())
		}
	}
	if tester.HasText("OK 200") {
		t.Fatalf("the probe result is painted instead of described; texts: %v", tester.Texts())
	}
	if _, _, description := keepAliveMetric(&state, time.Now()); description != "OK 200 · http://192.0.2.10/" {
		t.Fatalf("keep-alive description = %q", description)
	}

	// Nothing has run yet: every cell says so with a dash rather than a guess.
	idle := State{Session: session.State{Phase: session.PhaseIdle}, Config: config.Default()}
	rendered := ui.NewTester(View(&idle, Actions{}), 720, 560)
	rendered.Frame()
	if !rendered.HasText("—") {
		t.Fatalf("a missing metric is not a dash; texts: %v", rendered.Texts())
	}
}

// The granted resources are the point of the window after a login. They are
// grouped by kind with a count each, and the row's menu has to reach the
// browser and the clipboard.
func TestResourceListRendersAndActs(t *testing.T) {
	resources := session.Resources{Entries: []session.ResourceEntry{
		{Kind: session.ResourceApp, Title: "Wiki", Detail: "Portal · web", URL: "https://wiki.example.com", Copy: "https://wiki.example.com"},
		{Kind: session.ResourceIP, Title: "192.0.2.10", Detail: "tcp 443", Copy: "192.0.2.10"},
	}}
	state := State{
		Session: session.State{
			Phase:     session.PhaseOnline,
			Detail:    "vpn.example.com:443",
			ClientIP:  "192.0.2.10",
			Resources: resources,
		},
		Config: config.Default(),
	}

	rendered := ui.NewTester(View(&state, Actions{}), 720, 560)
	rendered.Frame()
	for _, want := range []string{"已获授权的资源", "2 项", "应用 · 1 项", "IP 地址 · 1 项", "Wiki", "Portal · web", "192.0.2.10", "tcp 443"} {
		if !rendered.HasText(want) {
			t.Fatalf("%q is missing from the resource list; texts: %v", want, rendered.Texts())
		}
	}

	var opened string
	openTester := ui.NewTester(View(&state, Actions{OpenResource: func(url string) { opened = url }}), 720, 560)
	if err := openTester.Click("Wiki 的操作"); err != nil {
		t.Fatalf("open the row menu: %v", err)
	}
	if err := openTester.ChooseMenuItem("打开"); err != nil {
		t.Fatalf("choose 打开: %v", err)
	}
	if opened != "https://wiki.example.com" {
		t.Fatalf("opened %q, want the app URL", opened)
	}

	var copied string
	copyTester := ui.NewTester(View(&state, Actions{CopyResource: func(value string) { copied = value }}), 720, 560)
	// The address rule has no suggested URL, so 打开 is disabled there; the copy
	// value is the row's own text.
	if err := copyTester.Click("192.0.2.10 的操作"); err != nil {
		t.Fatalf("open the row menu: %v", err)
	}
	if err := copyTester.ChooseMenuItem("打开"); err == nil {
		t.Fatal("打开 is enabled on a row with no URL")
	}
	if err := copyTester.ChooseMenuItem("复制"); err != nil {
		t.Fatalf("choose 复制: %v", err)
	}
	if copied != "192.0.2.10" {
		t.Fatalf("copied %q, want the row's copy value", copied)
	}
}

// A kind the window does not know still gets a group, so a newer gateway adds
// rows instead of losing them.
func TestResourceListGroupsUnknownKinds(t *testing.T) {
	state := State{
		Session: session.State{
			Phase:  session.PhaseOnline,
			Detail: "vpn.example.com:443",
			Resources: session.Resources{Entries: []session.ResourceEntry{
				{Kind: "tunnel", Title: "db-tunnel", Copy: "db-tunnel"},
				{Kind: session.ResourceDNS, Title: "db.example.com", Detail: "192.0.2.20", Copy: "db.example.com"},
			}},
		},
		Config: config.Default(),
	}

	tester := ui.NewTester(View(&state, Actions{}), 720, 560)
	tester.Frame()
	for _, want := range []string{"其他 · 1 项", "DNS · 1 项", "db-tunnel", "db.example.com"} {
		if !tester.HasText(want) {
			t.Fatalf("%q is missing from the grouped list; texts: %v", want, tester.Texts())
		}
	}
}

// A group longer than the list shows at once gets a filter; typing in it keeps
// the matching rows and drops the rest, and the headings follow the filter.
func TestResourceSearchFiltersLongGroups(t *testing.T) {
	entries := make([]session.ResourceEntry, 0, 10)
	for i := 1; i <= 9; i++ {
		entries = append(entries, session.ResourceEntry{
			Kind:  session.ResourceApp,
			Title: fmt.Sprintf("App %d", i),
			Copy:  fmt.Sprintf("https://app%d.example.com", i),
		})
	}
	entries = append(entries, session.ResourceEntry{Kind: session.ResourceIP, Title: "192.0.2.10", Detail: "tcp 443", Copy: "192.0.2.10"})

	state := State{
		Session: session.State{
			Phase:     session.PhaseOnline,
			Detail:    "vpn.example.com:443",
			Resources: session.Resources{Entries: entries},
		},
		Config: config.Default(),
	}

	tester := ui.NewTester(View(&state, Actions{}), 900, 640)
	tester.Frame()
	if !tester.HasText("应用 · 9 项") {
		t.Fatalf("the app heading is missing; texts: %v", tester.Texts())
	}
	if !tester.HasText("搜索资源") {
		t.Fatal("a group of nine rows has no filter")
	}

	if err := tester.Click("搜索资源"); err != nil {
		t.Fatalf("focus the filter: %v", err)
	}
	tester.Type("app 3")
	tester.Frame()

	if !tester.HasText("应用 · 1 项") || !tester.HasText("App 3") {
		t.Fatalf("the filter kept the wrong rows; texts: %v", tester.Texts())
	}
	if tester.HasText("App 4") || tester.HasText("IP 地址") {
		t.Fatalf("the filter kept rows it should drop; texts: %v", tester.Texts())
	}

	tester.Type("zzz")
	tester.Frame()
	if !tester.HasText("没有匹配的资源") {
		t.Fatalf("an empty match is not explained; texts: %v", tester.Texts())
	}
}

// The resource card offers its filter as soon as it holds a row, as the
// mockup draws it in the header; an empty card has nothing to filter.
func TestResourceSearchAppearsWithEntries(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseOnline, Detail: "vpn.example.com:443"},
		Config:  config.Default(),
	}
	empty := ui.NewTester(View(&state, Actions{}), 900, 640)
	empty.Frame()
	if empty.HasText("搜索资源") {
		t.Fatal("an empty resource card grew a filter")
	}

	state.Session.Resources.Entries = []session.ResourceEntry{
		{Kind: session.ResourceApp, Title: "App 1", Copy: "App 1"},
	}
	filled := ui.NewTester(View(&state, Actions{}), 900, 640)
	filled.Frame()
	if !filled.HasText("搜索资源") {
		t.Fatalf("the resource header lost its filter; texts: %v", filled.Texts())
	}
}

// At the window's minimum the list keeps three rows under the status card:
// the window is for the resources, so it may not collapse to a heading under
// them.
func TestResourceListKeepsThreeRowsAtTheMinimumSize(t *testing.T) {
	entries := make([]session.ResourceEntry, 0, 20)
	for i := 1; i <= 20; i++ {
		title := fmt.Sprintf("App %02d", i)
		entries = append(entries, session.ResourceEntry{Kind: session.ResourceApp, Title: title, Copy: title})
	}
	state := State{
		Session: session.State{Phase: session.PhaseOnline, Detail: "vpn.example.com:443", Resources: session.Resources{Entries: entries}},
		Config:  config.Default(),
	}

	tester := ui.NewTester(View(&state, Actions{}), 720, 560)
	tester.Frame()
	if !tester.HasText("已分配地址") {
		t.Fatalf("the status card left the window; texts: %v", tester.Texts())
	}
	height := float32(tester.Image().Bounds().Max.Y)
	for _, title := range []string{"App 01", "App 02", "App 03"} {
		rect, ok := tester.Find(title)
		if !ok {
			t.Fatalf("%q is not in the list; texts: %v", title, tester.Texts())
		}
		if rect.Y+rect.H > height {
			t.Fatalf("%q is below the window: %+v", title, rect)
		}
	}
}

func TestResourceListExplainsAnEmptyGrant(t *testing.T) {
	online := State{Session: session.State{Phase: session.PhaseOnline, Detail: "vpn.example.com:443"}, Config: config.Default()}
	rendered := ui.NewTester(View(&online, Actions{}), 720, 560)
	rendered.Frame()
	if !rendered.HasText("0 项") || !rendered.HasText("网关没有为该账号授予资源") {
		t.Fatalf("empty grant is not explained; texts: %v", rendered.Texts())
	}

	idle := State{Session: session.State{Phase: session.PhaseIdle, Detail: "未连接"}, Config: config.Default()}
	rendered = ui.NewTester(View(&idle, Actions{}), 720, 560)
	rendered.Frame()
	if !rendered.HasText("连接后显示网关授予的资源") {
		t.Fatalf("idle hint is missing; texts: %v", rendered.Texts())
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
	cfg.ActiveProfile = config.DefaultProfileName

	var login, logout int
	var selected string
	state := State{
		Session: session.State{Phase: session.PhaseOnline, Detail: "vpn.example.com:443", ClientIP: "203.0.113.4"},
		Config:  cfg,
	}
	menu := TrayMenu(&state, Actions{
		TrayLogin:     func() { login++ },
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
	// 复制配置 keeps the copy actions; the tray's job is the session, not a
	// second wording for every export.
	for _, item := range items {
		if strings.HasPrefix(item.ID, "copy") {
			t.Fatalf("the tray repeats the window's copy action: %q", item.Label)
		}
	}

	var toggle, accounts *mygo.MenuItem
	for _, item := range items {
		switch item.Label {
		case "断开":
			toggle = item
		case "切换账户":
			accounts = item
		}
	}
	if toggle == nil {
		t.Fatalf("no 断开 toggle in %v", labels(items))
	}
	toggle.Click(toggle, nil)
	if logout != 1 || login != 0 {
		t.Fatalf("toggle did not disconnect: login=%d logout=%d", login, logout)
	}

	if accounts == nil {
		t.Fatalf("no account submenu in %v", labels(items))
	}
	if len(accounts.Submenu) != 2 {
		t.Fatalf("account items = %d, want 2", len(accounts.Submenu))
	}
	if !accounts.Submenu[0].Checked || accounts.Submenu[1].Checked {
		t.Fatal("the active account is not the checked one")
	}
	accounts.Submenu[1].Click(accounts.Submenu[1], nil)
	if selected != "工作" {
		t.Fatalf("account click selected %q", selected)
	}
}

func TestTrayMenuOffersConnectWhenIdle(t *testing.T) {
	var login int
	state := State{
		Session: session.State{Phase: session.PhaseIdle},
		Config:  config.Default(),
	}
	menu := TrayMenu(&state, Actions{TrayLogin: func() { login++ }})
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

	// A login already running disables the same item: the tray must not start
	// a second one while the window's control refuses to.
	state.Busy = true
	menu = TrayMenu(&state, Actions{TrayLogin: func() { login++ }})
	for _, item := range menu.Items() {
		if item.ID == "toggle" && !item.Disabled {
			t.Fatal("the tray offers 连接 while an action is running")
		}
	}
}

// One account has nothing to switch to: the tray spends no line on a submenu
// that could only ever show the account it is already on.
func TestTrayMenuHidesTheAccountSubmenuForOneAccount(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseIdle},
		Config:  config.Default(),
	}
	menu := TrayMenu(&state, Actions{})
	for _, item := range menu.Items() {
		if item.ID == "accounts" {
			t.Fatalf("a single-account tray still offers %q in %v", item.Label, labels(menu.Items()))
		}
	}
}

// 打开默认应用 opens the URL the gateway returned for the first granted
// application, through the same action as the resource row (the desktop's
// default browser).
func TestTrayMenuOpensTheDefaultApp(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseOnline, Resources: session.Resources{
			Entries: []session.ResourceEntry{
				{Kind: session.ResourceDomain, Title: "wiki.example.com", URL: "https://wiki.example.com"},
				{Kind: session.ResourceApp, Title: "Wiki", URL: "https://portal.example.com/wiki"},
			},
		}},
		Config: config.Default(),
	}
	var opened string
	menu := TrayMenu(&state, Actions{OpenResource: func(url string) { opened = url }})

	var item *mygo.MenuItem
	for _, candidate := range menu.Items() {
		if candidate.ID == "open-app" {
			item = candidate
		}
	}
	if item == nil {
		t.Fatalf("no default-app item in %v", labels(menu.Items()))
	}
	if item.Disabled {
		t.Fatal("the default-app item is disabled although the grant has an app URL")
	}
	item.Click(item, nil)
	if opened != "https://portal.example.com/wiki" {
		t.Fatalf("opened %q, want the portal app URL", opened)
	}
}

func TestTrayMenuDefaultAppIsDisabledWithoutAURL(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseIdle},
		Config:  config.Default(),
	}
	menu := TrayMenu(&state, Actions{})
	var found bool
	for _, item := range menu.Items() {
		if item.ID != "open-app" {
			continue
		}
		found = true
		if !item.Disabled {
			t.Fatal("the default-app item is clickable with no grant")
		}
	}
	if !found {
		t.Fatalf("no default-app item in %v", labels(menu.Items()))
	}
}

func labels(items []*mygo.MenuItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Label)
	}
	return out
}

// The ladder says which attempt is next and how long the wait is, and the
// phase chip is where it lives; that is the only place a user learns the app
// is retrying instead of hanging.
func TestReconnectCountdownIsRendered(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	state := State{
		Session: session.State{
			Phase:  session.PhaseReconnecting,
			Detail: "会话已过期",
			Reconnect: session.ReconnectState{
				Attempt: 2,
				Max:     5,
				NextAt:  time.Now().Add(4 * time.Second),
			},
			Since: time.Now(),
		},
		Config: cfg,
	}

	tester := ui.NewTester(View(&state, Actions{}), 720, 560)
	tester.Frame()

	if !tester.HasText("重连中 (2/5，4 秒后)") {
		t.Fatalf("the ladder is missing; texts: %v", tester.Texts())
	}
	if !tester.HasText("网关地址 vpn.example.com:443") {
		t.Fatalf("the gateway left the meta line; texts: %v", tester.Texts())
	}
	if got := TrayStatus(state.Session); got != "重连中 (2/5，4 秒后)" {
		t.Fatalf("tray status = %q, want the attempt and the wait", got)
	}
}

// A port is part of an address, not a quantity: the gateway line must not
// group its digits, or a five-digit port reads as "38,265".
func TestGatewayPortIsNotGrouped(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "127.0.0.1"
	cfg.Profiles[0].Server.Port = 38265
	state := State{Session: session.State{Phase: session.PhaseIdle}, Config: cfg}

	tester := ui.NewTester(View(&state, Actions{}), 900, 740)
	tester.Frame()

	if !tester.HasText("网关地址 127.0.0.1:38265") {
		t.Fatalf("the port was reformatted; texts: %v", tester.Texts())
	}
}

// A keep-alive line has to distinguish a real HTTP answer, a transport failure
// and the DNS fallback, because the fallback does not hold the session open.
func TestKeepAliveSummary(t *testing.T) {
	at := time.Now()
	cases := []struct {
		name      string
		keepAlive session.KeepAliveState
		want      string
	}{
		{
			name:      "no probe yet",
			keepAlive: session.KeepAliveState{},
			want:      "",
		},
		{
			name: "a real HTTP answer",
			keepAlive: session.KeepAliveState{
				Target: "http://192.0.2.10/", Status: 200, OK: true, At: at,
			},
			want: "OK 200 · http://192.0.2.10/",
		},
		{
			name: "a transport failure",
			keepAlive: session.KeepAliveState{
				Target: "http://192.0.2.10/", Err: "dial timeout", At: at,
			},
			want: "失败 · http://192.0.2.10/",
		},
		{
			name: "an HTTP error",
			keepAlive: session.KeepAliveState{
				Target: "http://192.0.2.10/", Status: 503, At: at,
			},
			want: "失败 503 · http://192.0.2.10/",
		},
		{
			name: "the DNS fallback",
			keepAlive: session.KeepAliveState{
				Target: "www.baidu.com (UDP)", OK: true, Fallback: true, At: at,
			},
			want: "DNS OK · www.baidu.com (UDP)",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := KeepAliveSummary(testCase.keepAlive); got != testCase.want {
				t.Fatalf("summary = %q, want %q", got, testCase.want)
			}
		})
	}
}

// The tray is ordered by frequency: the state, the two window shortcuts, the
// session toggle, the account submenu and quit. Separators only divide groups
// that exist, so a one-account menu has no empty group and a menu never starts
// or ends with a line.
func TestTrayMenuOrder(t *testing.T) {
	one := State{Session: session.State{Phase: session.PhaseIdle}, Config: config.Default()}
	two := State{Session: session.State{Phase: session.PhaseIdle}, Config: config.Default()}
	if _, err := two.Config.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}

	cases := []struct {
		name  string
		state *State
		want  []string
	}{
		{
			name:  "one account",
			state: &one,
			want:  []string{"未连接", "-", "打开仪表盘", "打开默认应用", "-", "连接", "-", "退出"},
		},
		{
			name:  "two accounts",
			state: &two,
			want:  []string{"未连接", "-", "打开仪表盘", "打开默认应用", "-", "连接", "-", "切换账户", "-", "退出"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := menuLabels(TrayMenu(testCase.state, Actions{}))
			if strings.Join(got, "|") != strings.Join(testCase.want, "|") {
				t.Fatalf("tray order = %v, want %v", got, testCase.want)
			}
		})
	}
}

// menuLabels is the menu's order in one list: a separator is "-", every other
// item its label.
func menuLabels(menu *mygo.Menu) []string {
	out := make([]string, 0, len(menu.Items()))
	for _, item := range menu.Items() {
		if item.Type == mygo.MenuItemSeparator {
			out = append(out, "-")
			continue
		}
		out = append(out, item.Label)
	}
	return out
}

// 打开仪表盘 is the tray's one page shortcut: the tray icon keeps the page the
// window was reading, so only this item moves it to the session.
func TestTrayMenuOpensTheDashboard(t *testing.T) {
	state := State{Session: session.State{Phase: session.PhaseIdle}, Config: config.Default()}
	var chosen []Page
	menu := TrayMenu(&state, Actions{ShowPage: func(page Page) { chosen = append(chosen, page) }})

	var item *mygo.MenuItem
	for _, candidate := range menu.Items() {
		if candidate.ID == "dashboard" {
			item = candidate
		}
	}
	if item == nil {
		t.Fatalf("no dashboard item in %v", labels(menu.Items()))
	}
	item.Click(item, nil)
	if len(chosen) != 1 || chosen[0] != PageDashboard {
		t.Fatalf("dashboard item chose %v", chosen)
	}
}

// The window answers its own keys: Ctrl+Enter is the state's action, and Esc
// is the 取消登录 of the panel on screen.
func TestWindowShortcuts(t *testing.T) {
	var login, logout int
	actions := Actions{
		Login:  func() { login++ },
		Logout: func() { logout++ },
	}

	idle := State{Session: session.State{Phase: session.PhaseIdle}, Config: config.Default()}
	tester := ui.NewTester(View(&idle, actions), 720, 640)
	tester.Frame()
	tester.Key(ui.Cmd, ui.KeyEnter)
	if login != 1 {
		t.Fatalf("Ctrl+Enter on an idle window called login %d times, want 1", login)
	}

	online := idle
	online.Session.Phase = session.PhaseOnline
	tester = ui.NewTester(View(&online, actions), 720, 640)
	tester.Frame()
	tester.Key(ui.Cmd, ui.KeyEnter)
	if logout != 1 {
		t.Fatalf("Ctrl+Enter on a live session called logout %d times, want 1", logout)
	}

	// A panel owns the login: Esc cancels it, and Ctrl+Enter does not end the
	// session behind it.
	var cancelled int
	panel := State{Config: config.Default(), Code: &CodePrompt{Kind: session.CodeSMS}}
	tester = ui.NewTester(View(&panel, Actions{
		Logout:     func() { logout++ },
		CancelCode: func() { cancelled++ },
	}), 720, 640)
	tester.Frame()
	tester.Key(ui.Cmd, ui.KeyEnter)
	if logout != 1 {
		t.Fatalf("Ctrl+Enter ended a login the panel was asking about: logout=%d", logout)
	}
	tester.Key(0, ui.KeyEscape)
	if cancelled != 1 {
		t.Fatalf("Esc called cancel %d times, want 1", cancelled)
	}
}

// / reaches the resource filter, and once the field has the focus the key
// belongs to the text again.
func TestSlashFocusesTheResourceSearch(t *testing.T) {
	state := State{
		Session:   longGrant(),
		Config:    config.Default(),
		Listeners: renderListeners(),
	}
	tester := ui.NewTester(View(&state, Actions{}), 720, 640)
	tester.Frame()
	if active, _ := tester.TextInputActive(); active {
		t.Fatal("a text input has the focus before /")
	}
	tester.Key(0, ui.KeySlash)
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("/ did not focus the resource filter")
	}

	// A panel replaces the list; when the list returns, the filter must not
	// take the focus back on its own.
	state.Code = &CodePrompt{Kind: session.CodeSMS}
	tester.Frame()
	state.Code = nil
	tester.Frame()
	if active, _ := tester.TextInputActive(); active {
		t.Fatal("the filter took the focus back without a key press")
	}
}
