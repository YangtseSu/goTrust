// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
)

func TestSettingsApplyBuildsAProfile(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	id := state.Draft.Profiles[0].ID
	state.NameText = " 工作 "
	state.Gateways[0] = " https://vpn.example.com:8443/ "
	state.Draft.Profiles[0].Auth.Username = " alice "
	state.Draft.Profiles[0].Auth.Domain = " Radius "
	state.Draft.Profiles[0].Auth.StorePass = true
	state.Draft.Profiles[0].Auth.Password = "secret"
	state.Draft.Profiles[0].Keep.SetEnabled(false)
	state.Targets[0] = "http://192.0.2.5/\n\nhttp://192.0.2.6/, http://192.0.2.5/"
	state.Draft.Proxy.HTTPBind = ""

	cfg, err := state.Apply()
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	profile := cfg.Active()
	if cfg.ActiveProfile != "工作" || profile.Name != "工作" {
		t.Fatalf("active = %q, profile = %q", cfg.ActiveProfile, profile.Name)
	}
	if profile.ID != id {
		t.Fatalf("profile id = %q, want %q after a rename", profile.ID, id)
	}
	if profile.Server.Address != "vpn.example.com" || profile.Server.Port != 8443 {
		t.Fatalf("server = %+v", profile.Server)
	}
	if profile.Auth.Username != "alice" || profile.Auth.Domain != "Radius" {
		t.Fatalf("auth = %+v", profile.Auth)
	}
	if profile.Keep.IsEnabled() {
		t.Fatal("keep-alive disabled in the form came back enabled")
	}
	if len(profile.Keep.Targets) != 2 ||
		profile.Keep.Targets[0] != "http://192.0.2.5/" ||
		profile.Keep.Targets[1] != "http://192.0.2.6/" {
		t.Fatalf("targets = %v", profile.Keep.Targets)
	}
	if socks5, httpBind := cfg.ProxyAddresses(); httpBind != "" || socks5 == "" {
		t.Fatalf("proxy = %q %q", socks5, httpBind)
	}
	if cfg.Version != config.CurrentVersion {
		t.Fatalf("version = %d", cfg.Version)
	}
}

// The first run must be able to store proxy and startup settings before the
// user knows the gateway; login validates the account.
func TestSettingsApplyAllowsAnIncompleteAccount(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	if _, err := state.Apply(); err != nil {
		t.Fatalf("an empty account was rejected: %v", err)
	}
}

func TestSettingsApplyRejectsBadValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*SettingsState)
	}{
		{"empty name", func(state *SettingsState) { state.NameText = "  " }},
		{"bad port", func(state *SettingsState) { state.Gateways[0] = "vpn.example.com:70000" }},
		{"text port", func(state *SettingsState) { state.Gateways[0] = "vpn.example.com:https" }},
		{"other scheme", func(state *SettingsState) { state.Gateways[0] = "http://vpn.example.com" }},
		{"bad interval", func(state *SettingsState) { state.Draft.Profiles[0].Keep.Interval = "soon" }},
		{"interval below the floor", func(state *SettingsState) { state.Draft.Profiles[0].Keep.Interval = "5s" }},
		{"no listener", func(state *SettingsState) {
			state.Draft.Proxy.SOCKS5Bind = ""
			state.Draft.Proxy.HTTPBind = ""
		}},
		{"non-loopback bind", func(state *SettingsState) {
			state.Draft.Proxy.HTTPBind = "0.0.0.0:1081"
		}},
		{"malformed bind", func(state *SettingsState) {
			state.Draft.Proxy.SOCKS5Bind = "127.0.0.1"
		}},
		{"duplicate name", func(state *SettingsState) {
			state.AddProfile()
			state.NameText = config.DefaultProfileName
			state.Selected = 1
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			state := NewSettingsState(config.Default(), "/tmp/logs")
			testCase.mutate(state)
			if _, err := state.Apply(); err == nil {
				t.Fatal("bad settings were accepted")
			}
		})
	}
}

// The binds are typed by hand now, so the window checks them before saving; the
// same rule runs again before a login.
func TestSettingsApplyTrimsAndChecksProxyBinds(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Draft.Proxy.SOCKS5Bind = " 127.0.0.1:1080 "
	state.Draft.Proxy.HTTPBind = " localhost:1081 "

	cfg, err := state.Apply()
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if socks5, httpBind := cfg.ProxyAddresses(); socks5 != "127.0.0.1:1080" || httpBind != "localhost:1081" {
		t.Fatalf("proxy = %q %q, want the trimmed binds", socks5, httpBind)
	}
}

// The floor is the shortest cadence a probe may have; a value at the floor is
// accepted, so the error is about the rate and not a fixed list of values.
func TestSettingsApplyAcceptsTheMinimumKeepAliveInterval(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Draft.Profiles[0].Keep.Interval = "10s"

	cfg, err := state.Apply()
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := cfg.KeepAliveInterval(); got != config.MinimumKeepAliveInterval {
		t.Fatalf("interval = %v, want %v", got, config.MinimumKeepAliveInterval)
	}
}

func TestSettingsProfileActions(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")

	state.AddProfile()
	if len(state.Draft.Profiles) != 2 || state.Selected != 1 {
		t.Fatalf("profiles = %v selected = %d", state.Draft.ProfileNames(), state.Selected)
	}
	if len(state.Gateways) != 2 || len(state.Targets) != 2 {
		t.Fatalf("buffers: gateways=%d targets=%d", len(state.Gateways), len(state.Targets))
	}

	state.DuplicateProfile()
	if len(state.Draft.Profiles) != 3 || state.Selected != 2 {
		t.Fatalf("profiles = %v selected = %d", state.Draft.ProfileNames(), state.Selected)
	}
	if state.NameText == "" {
		t.Fatal("the copied profile has no name in the editor")
	}

	state.Select(0)
	if state.Selected != 0 || state.NameText != config.DefaultProfileName {
		t.Fatalf("select: %d %q", state.Selected, state.NameText)
	}

	state.DeleteProfile()
	if len(state.Draft.Profiles) != 2 {
		t.Fatalf("profiles after delete = %v", state.Draft.ProfileNames())
	}
	if len(state.Gateways) != 2 || len(state.Targets) != 2 {
		t.Fatalf("buffers did not shrink: gateways=%d targets=%d", len(state.Gateways), len(state.Targets))
	}
}

// A rename or a delete only becomes a file move when the draft is saved. The
// state reports what to move; the caller applies it after writing the settings.
func TestSettingsClientDataChangesFollowTheDraft(t *testing.T) {
	cfg := config.Default()
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	state := NewSettingsState(cfg, "/tmp/logs")

	if _, err := state.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if changes := state.ClientDataChanges(); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none for an unchanged draft", changes)
	}

	state.NameText = "工作二"
	if _, err := state.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	changes := state.ClientDataChanges()
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want the rename", changes)
	}
	if changes[0].From.Name != "工作" || changes[0].To.Name != "工作二" {
		t.Fatalf("rename = %q -> %q", changes[0].From.Name, changes[0].To.Name)
	}

	// Selecting another profile discards the typed name, so the rename is no
	// longer pending.
	state.Select(0)
	if changes := state.ClientDataChanges(); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none after the selection changed", changes)
	}

	state.Select(1)
	state.DeleteProfile()
	changes = state.ClientDataChanges()
	if len(changes) != 1 || changes[0].From.Name != "工作" || changes[0].To.Name != "" {
		t.Fatalf("changes = %+v, want one removal of 工作", changes)
	}

	// A successful save reloads the draft, which clears what was applied.
	state.Reload(cfg)
	if changes := state.ClientDataChanges(); len(changes) != 0 {
		t.Fatalf("changes = %+v after reload", changes)
	}
}

func TestSettingsCannotDeleteTheLastProfile(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.DeleteProfile()
	if len(state.Draft.Profiles) != 1 {
		t.Fatal("the last profile was deleted")
	}
	if state.Status == "" {
		t.Fatal("deleting the last profile reported nothing")
	}
}

// A discovered list belongs to the gateway in the draft. Editing the address or
// the port has to drop it before the picker can pin the old gateway's method.
func TestEditingTheGatewayDropsTheDiscoveredMethods(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "old.example.com"
	cfg.Profiles[0].Server.Port = 443
	state := NewSettingsState(cfg, "/tmp/logs")

	methods := []session.AuthMethod{
		{Type: "auth/psw", Domain: "local"},
		{Type: "auth/cas", Domain: "cas"},
	}
	state.SetMethods(methods, state.targetKey(), "已获取 2 种认证方式", false)

	tester := ui.NewTester(View(&State{Settings: state, Config: state.Draft, Page: PageAccount}, Actions{Settings: SettingsActions{}}), 900, 1200)
	tester.Frame()
	if len(state.Methods) != 2 {
		t.Fatalf("methods = %v, want the discovered list", state.Methods)
	}

	// The same gateway keeps the list across frames.
	tester.Frame()
	if len(state.Methods) != 2 {
		t.Fatalf("methods = %v, want the list to survive an unchanged frame", state.Methods)
	}

	// Another port is another gateway.
	state.Gateways[0] = "old.example.com:8443"
	tester.Frame()
	if len(state.Methods) != 0 || state.MethodsFrom != "" {
		t.Fatalf("methods = %v from %q, want them dropped", state.Methods, state.MethodsFrom)
	}

	// The picker itself refuses a stale entry, even when a frame has not run
	// since the edit: the chosen method must belong to the current gateway.
	state.SetMethods(methods, state.targetKey(), "已获取 2 种认证方式", false)
	state.Gateways[0] = "old.example.com:9443"
	state.SelectAuth(1)
	if got := state.Draft.Profiles[0].Auth.AuthType; got != "" {
		t.Fatalf("auth type = %q, want the stale choice refused", got)
	}
}

func TestShellRendersPagesAndCallsActions(t *testing.T) {
	var saved, created, discarded, autostart, verbose int
	state := NewSettingsState(config.Default(), "/var/log/gotrust")
	state.Draft.Profiles[0].Auth.Password = "hunter2"

	actions := SettingsActions{
		Save:       func() { saved++ },
		Discard:    func() { discarded++ },
		NewProfile: func() { created++ },
		SetAutostart: func(on bool) {
			if on {
				autostart++
			}
		},
		SetVerbose: func(on bool) {
			if on {
				verbose++
			}
		},
	}

	_, tester := newShell(t, state, actions, PageAccount)

	for _, wanted := range []string{"仪表盘", "账户", "代理", "保活", "启动", "终端管理", "日志", "设置", "多账户", "安全"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the window; texts: %v", wanted, tester.Texts())
		}
	}
	if tester.HasText("hunter2") {
		t.Fatal("the password is painted as readable text")
	}
	// Another page is another form: the fields of the page that is not shown
	// are not in the tree at all.
	if tester.HasText("/var/log/gotrust") {
		t.Fatal("a page that is not shown is drawn")
	}

	openPage(t, tester, "多账户")
	// 新建 makes the draft differ from the file, which is what shows 保存.
	if err := tester.Click("新建"); err != nil {
		t.Fatalf("click new: %v", err)
	}
	if created != 1 {
		t.Fatalf("new profile calls = %d", created)
	}
	tester.Frame()
	if !tester.HasText("未保存的修改") {
		t.Fatalf("the save bar is missing; texts: %v", tester.Texts())
	}
	if err := tester.Click("保存"); err != nil {
		t.Fatalf("click save: %v", err)
	}
	if saved != 1 {
		t.Fatalf("save calls = %d", saved)
	}
	if err := tester.Click("放弃修改"); err != nil {
		t.Fatalf("click discard: %v", err)
	}
	if discarded != 1 {
		t.Fatalf("discard calls = %d", discarded)
	}

	openPage(t, tester, "启动")
	if err := tester.Click("登录系统时启动"); err != nil {
		t.Fatalf("click autostart: %v", err)
	}
	if !state.Draft.App.Autostart || autostart != 1 {
		t.Fatalf("autostart = %v calls = %d", state.Draft.App.Autostart, autostart)
	}

	openPage(t, tester, "设置")
	for _, wanted := range []string{"语言", "跟随系统", "本机数据", "关于", "项目主页", "许可证"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from 设置; texts: %v", wanted, tester.Texts())
		}
	}
	// The log directory belongs to 日志 now, and so does the level switch.
	if tester.HasText("/var/log/gotrust") {
		t.Fatal("the log directory is still on 设置")
	}
	if tester.HasText("详细日志") {
		t.Fatal("详细日志 is still on 设置")
	}
	openPage(t, tester, "日志")
	if !tester.HasText("/var/log/gotrust") {
		t.Fatal("the log directory is missing from 日志")
	}
	if err := tester.Click("详细日志"); err != nil {
		t.Fatalf("click verbose: %v", err)
	}
	if !state.Draft.Logging.Verbose || verbose != 1 {
		t.Fatalf("verbose = %v calls = %d", state.Draft.Logging.Verbose, verbose)
	}

	if tester.HasText("关闭") {
		t.Fatal("the window still offers 关闭")
	}
}

// The heading marks a save that would change the file, and only that: a value
// the file already holds, padding, or the language (mygo stores that choice
// itself) is not an unsaved edit.
func TestSettingsChangedFollowsTheSavedFile(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	state := NewSettingsState(cfg, "/tmp/logs")
	if state.Changed() {
		t.Fatal("a fresh draft is marked unsaved")
	}

	state.Gateways[0] = " vpn.example.com "
	state.NameText = " " + config.DefaultProfileName + " "
	if state.Changed() {
		t.Fatal("values the file already holds are marked unsaved")
	}
	state.LocaleIndex = 2
	if state.Changed() {
		t.Fatal("the language choice is marked as a settings edit")
	}

	state.Gateways[0] = "vpn.example.com:8443"
	if !state.Changed() {
		t.Fatal("an edited gateway is not marked")
	}
	state.Reload(cfg)
	if state.Changed() {
		t.Fatal("the reload kept the edit")
	}

	// Switching the picker is an edit: saving is what makes the switch happen.
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	cfg.ActiveProfile = config.DefaultProfileName
	state = NewSettingsState(cfg, "/tmp/logs")
	state.Select(state.Draft.ProfileIndex("工作"))
	if !state.Changed() {
		t.Fatal("choosing another profile is not marked")
	}

	// So is removing one, even when nothing else changed.
	state = NewSettingsState(cfg, "/tmp/logs")
	state.Select(0)
	state.DeleteProfile()
	if !state.Changed() {
		t.Fatal("a deleted profile is not marked")
	}
}

// The sidebar is the window's navigation: every page replaces the one before
// it, and the page a test asks for is the one the shell draws.
func TestSidebarSwitchesPages(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	view, tester := newShell(t, state, settingsActionsFor(state), PageAccount)

	pages := []struct {
		label string
		page  Page
		want  string
		gone  string
	}{
		{"代理", PageProxy, "SOCKS5 监听", "网关地址"},
		{"保活", PageKeepAlive, "启用保活", "SOCKS5 监听"},
		{"启动", PageStartup, "登录系统时启动", "启用保活"},
		{"终端管理", PageTerminals, "连接后可读取终端列表", "登录系统时启动"},
		{"日志", PageLogs, "/tmp/logs", "连接后可读取终端列表"},
		{"设置", PageSettings, "关于", "/tmp/logs"},
		{"多账户", PageProfiles, "账户名称", "关于"},
	}
	for _, testCase := range pages {
		openPage(t, tester, testCase.label)
		if view.Page != testCase.page {
			t.Fatalf("%s selected page %d, want %d", testCase.label, view.Page, testCase.page)
		}
		if !tester.HasText(testCase.want) {
			t.Fatalf("%q is missing from %s; texts: %v", testCase.want, testCase.label, tester.Texts())
		}
		if tester.HasText(testCase.gone) {
			t.Fatalf("%s kept %q from the page before", testCase.label, testCase.gone)
		}
	}

	openPage(t, tester, "仪表盘")
	if view.Page != PageDashboard {
		t.Fatalf("dashboard selected page %d", view.Page)
	}
	if !tester.HasText("连接后显示网关授予的资源") {
		t.Fatalf("the dashboard is missing; texts: %v", tester.Texts())
	}
}

// The account rows are the picker: every account shows its name and gateway,
// the live one carries 当前, and a click moves the draft's selection to that
// account, which is what the other pages then edit.
func TestAccountsPageRowsMarkTheLiveAccountAndSelect(t *testing.T) {
	cfg := config.Default()
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add account: %v", err)
	}
	cfg.ActiveProfile = config.DefaultProfileName
	cfg.Profiles[cfg.ProfileIndex("工作")].Server.Address = "vpn.example.com"

	state := NewSettingsState(cfg, "/tmp/logs")
	_, tester := newShell(t, state, settingsActionsFor(state), PageProfiles)

	if !tester.HasText(locales.NameField()) {
		t.Fatalf("多账户 has no account name field; texts: %v", tester.Texts())
	}
	if !tester.HasText("vpn.example.com:443") {
		t.Fatalf("the row does not show its gateway; texts: %v", tester.Texts())
	}
	if !tester.HasText(locales.ProfileCurrent()) {
		t.Fatalf("no row marks the live account; texts: %v", tester.Texts())
	}
	if err := tester.Click("工作"); err != nil {
		t.Fatalf("click the second account: %v", err)
	}
	if state.Selected != state.Draft.ProfileIndex("工作") {
		t.Fatalf("selected = %d, want the 工作 account", state.Selected)
	}
}

// The save bar offers the two actions that move the draft: save it, or throw
// it away. It appears only while the draft differs from the file.
func TestSettingsSaveAndDiscardFollowTheDraft(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	discarded := 0
	_, tester := newShell(t, state, SettingsActions{
		Discard: func() { state.Reload(config.Default()); discarded++ },
	}, PageAccount)

	if tester.HasText("未保存的修改") {
		t.Fatalf("a clean draft shows the save bar; texts: %v", tester.Texts())
	}
	if err := tester.Click("放弃修改"); err == nil {
		t.Fatal("放弃修改 is on screen with nothing to discard")
	}

	state.Gateways[0] = "vpn.example.com:8443"
	tester.Frame()
	if !tester.HasText("未保存的修改") {
		t.Fatalf("an edited gateway does not show the save bar; texts: %v", tester.Texts())
	}
	if err := tester.Click("放弃修改"); err != nil {
		t.Fatalf("click discard: %v", err)
	}
	if discarded != 1 {
		t.Fatalf("discard calls = %d", discarded)
	}
	if state.Changed() || state.Gateways[0] != "" {
		t.Fatalf("the draft survived the discard: %q", state.Gateways[0])
	}
	tester.Frame()
	if tester.HasText("未保存的修改") {
		t.Fatal("the save bar outlived the discard")
	}
}

func TestSettingsKeepAliveSwitchUpdatesTheProfile(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Draft.Profiles[0].Keep.SetEnabled(true)

	_, tester := newShell(t, state, SettingsActions{}, PageKeepAlive)
	if err := tester.Click("启用保活"); err != nil {
		t.Fatalf("click keep-alive: %v", err)
	}
	if state.Draft.Profiles[0].Keep.IsEnabled() {
		t.Fatal("clicking the switch did not disable the keep-alive")
	}
}

func TestAccountPageHintsAtTheProfileSwitch(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.AddProfile()

	_, tester := newShell(t, state, SettingsActions{}, PageAccount)
	if !tester.HasText("保存后将切换到账户") {
		t.Fatalf("no switch hint; texts: %v", tester.Texts())
	}
}

func TestSettingsReloadDiscardsEdits(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.NameText = "unsaved"
	state.Gateways[0] = "vpn.example.com:9999"

	state.Reload(config.Default())
	if state.NameText != config.DefaultProfileName || state.Gateways[0] != "" {
		t.Fatalf("reload kept edits: %q %q", state.NameText, state.Gateways[0])
	}
}

// Choosing a discovered method stores the exact pair the gateway matches;
// choosing 自动 clears it again.
func TestSettingsSelectAuthStoresThePair(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Gateways[0] = "vpn.example.com:443"
	methods := []session.AuthMethod{
		{Name: "密码登录", Type: "auth/psw", Domain: "local"},
		{Name: "统一认证", Type: "auth/cas", Domain: "Radius"},
	}

	state.SetMethods(methods, state.targetKey(), "已获取 2 种认证方式", false)
	if state.Discovering {
		t.Fatal("the state is still marked as discovering")
	}
	state.SelectAuth(2)
	if pair := state.Draft.Profiles[0].Auth; pair.AuthType != "auth/cas" || pair.Domain != "Radius" {
		t.Fatalf("pair = %s · %s", pair.AuthType, pair.Domain)
	}
	if state.Status != "认证方式："+MethodLabel(methods[1]) {
		t.Fatalf("status = %q", state.Status)
	}

	// The saved-but-not-offered entry changes nothing.
	state.SelectAuth(3)
	if got := state.Draft.Profiles[0].Auth.AuthType; got != "auth/cas" {
		t.Fatalf("the trailing entry changed the pair to %q", got)
	}

	state.SelectAuth(0)
	if got := state.Draft.Profiles[0].Auth.AuthType; got != "" {
		t.Fatalf("automatic left %q stored", got)
	}
}

// A saved pair the gateway no longer lists is still shown, and switching
// profiles drops the list because it belongs to the other gateway.
func TestSettingsMethodOptionsKeepAStalePair(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Draft.Profiles[0].Auth.AuthType = "auth/radius"
	state.Draft.Profiles[0].Auth.Domain = "local"

	options := methodOptions(state.Methods, "auth/radius", "local")
	if len(options) != 2 || options[0] != locales.AuthAutoLabel() {
		t.Fatalf("options = %v", options)
	}
	if !strings.Contains(options[1], "网关未提供") {
		t.Fatalf("options = %v", options)
	}

	state.SetMethods([]session.AuthMethod{{Name: "密码登录", Type: "auth/psw", Domain: "local"}}, "vpn.example.com|443", "ok", false)
	state.AddProfile()
	if state.Methods != nil {
		t.Fatal("the method list survived a profile switch")
	}
}

// Discovery queries the gateway in the draft, so the gateway field has to be
// usable before a save.
func TestSettingsTargetReadsTheDraft(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	if _, err := state.Target(); err == nil {
		t.Fatal("an empty gateway address produced a target")
	}

	state.Gateways[0] = " https://vpn.example.com:8443/ "
	profile, err := state.Target()
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	if profile.ServerAddress() != "vpn.example.com" || profile.Server.Port != 8443 {
		t.Fatalf("target = %+v", profile.Server)
	}

	state.Gateways[0] = "vpn.example.com:0"
	if _, err := state.Target(); err == nil {
		t.Fatal("an out-of-range port produced a target")
	}
}

// The settings window shows the picker, the discover button and the hint.
func TestAccountPageShowsTheAuthPicker(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Gateways[0] = "vpn.example.com:443"
	methods := []session.AuthMethod{
		{Name: "密码登录", Type: "auth/psw", Domain: "local"},
		{Name: "统一认证", Type: "auth/cas", Domain: "local"},
	}
	state.SetMethods(methods, state.targetKey(), "已获取 2 种认证方式", false)
	state.SelectAuth(2)

	discovered := 0
	actions := SettingsActions{
		DiscoverAuth: func() { discovered++ },
	}

	_, tester := newShell(t, state, actions, PageAccount)
	tester.Frame()

	for _, wanted := range []string{"认证方式", "获取认证方式", "网关提供 2 种认证方式", MethodLabel(methods[1])} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the settings window", wanted)
		}
	}

	if err := tester.Click("获取认证方式"); err != nil {
		t.Fatalf("click discover: %v", err)
	}
	if discovered != 1 {
		t.Fatalf("discover calls = %d", discovered)
	}
}

// A save keeps the discovered list when the gateway did not change; editing
// the address drops it, because the pairs belong to the old gateway.
func TestSettingsReloadKeepsMethodsForTheSameGateway(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	state := NewSettingsState(cfg, "/tmp/logs")
	state.SetMethods([]session.AuthMethod{{Type: "auth/psw", Domain: "local"}}, "vpn.example.com|443", "ok", false)

	state.Reload(cfg)
	if len(state.Methods) != 1 {
		t.Fatalf("methods after reload = %+v", state.Methods)
	}

	other := cfg.Clone()
	other.Profiles[0].Server.Address = "other.example.com"
	state.Reload(other)
	if state.Methods != nil {
		t.Fatalf("methods survived a gateway change: %+v", state.Methods)
	}
}

// settingsActionsFor wires the callbacks the way main.go does, so the tests
// drive the real state through the real buttons.
func settingsActionsFor(state *SettingsState) SettingsActions {
	return SettingsActions{
		NewProfile:       func() { state.AddProfile() },
		DuplicateProfile: func() { state.DuplicateProfile() },
		DeleteProfile:    func() { state.DeleteProfile() },
		SelectProfile:    func(index int) { state.Select(index) },
		SelectAuth:       func(index int) { state.SelectAuth(index) },
	}
}

// Deleting the selected profile shrinks the draft from inside the frame. The
// rest of that frame must not index the profile that disappeared: the old
// code read Gateways[1] after 新建 → 删除 and panicked on the GTK main thread.
func TestProfilesPageDeletesWithoutAPanic(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	_, tester := newShell(t, state, settingsActionsFor(state), PageProfiles)

	if err := tester.Click("新建"); err != nil {
		t.Fatalf("click new: %v", err)
	}
	if len(state.Draft.Profiles) != 2 || state.Selected != 1 {
		t.Fatalf("profiles = %v selected = %d", state.Draft.ProfileNames(), state.Selected)
	}
	tester.Frame()

	if err := tester.Click("删除"); err != nil {
		t.Fatalf("click delete: %v", err)
	}
	if names := state.Draft.ProfileNames(); len(names) != 1 || names[0] != config.DefaultProfileName {
		t.Fatalf("profiles = %v", names)
	}
	if state.Selected != 0 {
		t.Fatalf("selected = %d", state.Selected)
	}
	if len(state.Gateways) != 1 || len(state.Targets) != 1 {
		t.Fatalf("buffers after delete: gateways=%d targets=%d", len(state.Gateways), len(state.Targets))
	}

	tester.Frame()
	if !tester.HasText("已删除账户；保存后生效") {
		t.Fatalf("the status line did not survive; texts: %v", tester.Texts())
	}
}

// Gateways and keep-alive targets live in buffers indexed like Draft.Profiles.
// Deleting a middle profile has to drop that profile's buffers, not merely
// truncate the end: 默认/B(b:8443)/C(c:9443) minus B must leave C holding its
// own gateway.
func TestSettingsDeleteKeepsGatewayAndTargetsOnTheirProfile(t *testing.T) {
	state := threeProfileState(t)
	state.Select(state.Draft.ProfileIndex("B"))
	state.Gateways[state.Selected] = "b.example.com:8450"
	state.Targets[state.Selected] = "http://192.0.2.20/"

	tester := ui.NewTester(View(&State{Settings: state, Config: state.Draft, Page: PageProfiles},
		Actions{Settings: settingsActionsFor(state)}), 900, 1200)
	if err := tester.Click("删除"); err != nil {
		t.Fatalf("click delete: %v", err)
	}

	if names := state.Draft.ProfileNames(); len(names) != 2 || names[0] != config.DefaultProfileName || names[1] != "C" {
		t.Fatalf("profiles = %v", names)
	}
	if state.Selected != 1 {
		t.Fatalf("selected = %d", state.Selected)
	}
	if state.Gateways[1] != "c.example.com:9443" {
		t.Fatalf("C's gateway buffer = %q, want c.example.com:9443", state.Gateways[1])
	}
	if state.Targets[1] != "http://192.0.2.31/" {
		t.Fatalf("C's targets buffer = %q", state.Targets[1])
	}

	cfg, err := state.Apply()
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	profile := cfg.Active()
	if profile.Name != "C" || profile.ServerAddress() != "c.example.com" || profile.Server.Port != 9443 {
		t.Fatalf("active = %q server = %+v", profile.Name, profile.Server)
	}
	if len(profile.Keep.Targets) != 1 || profile.Keep.Targets[0] != "http://192.0.2.31/" {
		t.Fatalf("active targets = %v", profile.Keep.Targets)
	}
}

// 复制 copies what the editor shows, including the unsaved gateway and targets,
// and leaves the other profiles' buffers where they were.
func TestSettingsDuplicateCopiesTheEditedBuffers(t *testing.T) {
	state := threeProfileState(t)
	state.Select(state.Draft.ProfileIndex("B"))
	state.Gateways[state.Selected] = "b.example.com:8450"
	state.Targets[state.Selected] = "http://192.0.2.20/"

	state.DuplicateProfile()

	copied := state.Draft.ProfileIndex("B 副本")
	if copied < 0 {
		t.Fatalf("profiles = %v", state.Draft.ProfileNames())
	}
	if state.Selected != copied {
		t.Fatalf("selected = %d, copy = %d", state.Selected, copied)
	}
	if state.Gateways[copied] != "b.example.com:8450" || state.Targets[copied] != "http://192.0.2.20/" {
		t.Fatalf("copy buffers = %q %q", state.Gateways[copied], state.Targets[copied])
	}
	if state.Gateways[2] != "c.example.com:9443" || state.Targets[2] != "http://192.0.2.31/" {
		t.Fatalf("C was disturbed: %q %q", state.Gateways[2], state.Targets[2])
	}
}

// threeProfileState is 默认/B(b:8443)/C(c:9443) with distinct keep-alive
// targets.
func threeProfileState(t *testing.T) *SettingsState {
	t.Helper()
	cfg := config.Default()
	if _, err := cfg.AddProfile("B"); err != nil {
		t.Fatalf("add B: %v", err)
	}
	if _, err := cfg.AddProfile("C"); err != nil {
		t.Fatalf("add C: %v", err)
	}
	b := cfg.ProfileIndex("B")
	cfg.Profiles[b].Server.Address = "b.example.com"
	cfg.Profiles[b].Server.Port = 8443
	cfg.Profiles[b].Keep.Targets = []string{"http://192.0.2.21/"}
	c := cfg.ProfileIndex("C")
	cfg.Profiles[c].Server.Address = "c.example.com"
	cfg.Profiles[c].Server.Port = 9443
	cfg.Profiles[c].Keep.Targets = []string{"http://192.0.2.31/"}
	return NewSettingsState(cfg, "/tmp/logs")
}

// A stored TOTP secret is a credential the form cannot show, so the connection
// section offers a removal action. The removal is a draft edit and takes effect
// with 保存.
func TestSettingsRemoveTheStoredTOTPSecret(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Auth.TOTPSecret = "JBSWY3DPEHPK3PXP"
	state := NewSettingsState(cfg, "/tmp/logs")

	_, tester := newShell(t, state, SettingsActions{}, PageAccount)
	if err := tester.Click("移除"); err != nil {
		t.Fatalf("click 移除: %v", err)
	}
	if got := state.Draft.Profiles[0].Auth.TOTPSecret; got != "" {
		t.Fatalf("draft secret = %q, want it removed", got)
	}
	if !strings.Contains(state.Status, "TOTP") {
		t.Fatalf("status = %q, want the removal confirmed", state.Status)
	}

	saved, err := state.Apply()
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := saved.Active().Auth.TOTPSecret; got != "" {
		t.Fatalf("applied secret = %q, want none", got)
	}

	// With the secret gone the row is gone too, so there is nothing to click.
	_, again := newShell(t, state, SettingsActions{}, PageAccount)
	if err := again.Click("移除"); err == nil {
		t.Fatal("the removal action survived its own click")
	}
}

// The terminal controls read the portal through the live session and make the
// state legible before the action; a disconnected window must not offer it.
func TestSettingsShowsAndChangesTrust(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Online = true
	state.TrustKnown = true
	state.Trusted = true
	state.TerminalsKnown = true
	state.Terminals = &session.Terminals{
		SelfID:         "self",
		MaxDeviceCount: 3,
		Trusted: []session.Terminal{{
			ID:               "trusted-1",
			DevDBID:          "6f1c9a2e4b7d",
			DeviceName:       "desk-01",
			DeviceType:       "client",
			OS:               "Linux",
			OSVersion:        "7.3.0",
			LastLoginAddress: "LAN",
			LastLoginIP:      "192.0.2.10",
			LastLoginTime:    "2026-10-25 13:17:59",
			NetworkZoneList:  []string{"校园网", "无线"},
		}},
		Temporary: []session.Terminal{{
			ID:               "self",
			DevDBID:          "2a9d51c0f8e3",
			DeviceName:       "desk-02",
			DeviceType:       "browser",
			OS:               "Linux",
			OSVersion:        "7.3.0",
			Browser:          "Firefox 128",
			OnlineStatus:     true,
			LastLoginAddress: "Example City",
			LastLoginIP:      "192.0.2.11",
			LastLoginTime:    "2026-10-25 13:17:59",
		}},
	}
	var changed bool
	refreshed := 0
	actions := SettingsActions{
		SetTrusted:       func(trusted bool) { changed = trusted },
		RefreshTerminals: func() { refreshed++ },
	}

	_, tester := newShell(t, state, actions, PageTerminals)
	for _, wanted := range []string{
		"授信终端 1/3 · 临时终端 1", "本机已信任", "desk-01", "desk-02", "当前设备",
		"类型", "客户端", "Linux 7.3.0", "Firefox 128", "最近登录", "登录地址",
		"内网（192.0.2.10）", "网络区域", "校园网 / 无线", "设备 ID", "6f1c9a2e4b7d",
	} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from 终端管理", wanted)
		}
	}
	if err := tester.Click("取消信任"); err != nil {
		t.Fatalf("click untrust: %v", err)
	}
	if changed {
		t.Fatal("取消信任 asked to trust the device")
	}

	state.Online = false
	_, tester = newShell(t, state, actions, PageTerminals)
	if !tester.HasText("连接后可读取终端列表") {
		t.Fatal("the offline hint is missing")
	}
	if err := tester.Click("刷新"); err == nil && refreshed != 0 {
		t.Fatal("the refresh button ran while offline")
	}
}

// 代理 owns the two copy actions the dashboard used to hide in its menu: the
// PAC address and one of the three Clash/Mihomo renderings.
func TestProxyPageCopiesTheProxyConfig(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.ClashFormat = ClashScript
	view := &State{
		Settings: state,
		Config:   state.Draft,
		Page:     PageProxy,
		PACURL:   "http://127.0.0.1:1082/pac",
	}
	var pac, clash int
	var format ClashFormat
	tester := ui.NewTester(View(view, Actions{
		CopyPAC:   func() { pac++ },
		CopyClash: func(chosen ClashFormat) { clash++; format = chosen },
	}), 900, 1400)
	tester.Frame()

	if !tester.HasText("http://127.0.0.1:1082/pac") {
		t.Fatalf("the PAC address is missing; texts: %v", tester.Texts())
	}
	if !tester.HasText(locales.ClashScript()) {
		t.Fatalf("the picker does not show %q; texts: %v", locales.ClashScript(), tester.Texts())
	}
	if err := tester.Click(locales.CopyPAC()); err != nil {
		t.Fatalf("click the PAC copy: %v", err)
	}
	if pac != 1 {
		t.Fatalf("PAC copy calls = %d", pac)
	}
	if err := tester.Click(locales.CopyClashDesc()); err != nil {
		t.Fatalf("click the Clash copy: %v", err)
	}
	if clash != 1 || format != ClashScript {
		t.Fatalf("Clash copy calls = %d format = %q", clash, format)
	}

	// Without a PAC URL there is nothing to copy, so the button is disabled.
	view.PACURL = ""
	tester.Frame()
	if !tester.HasText(locales.PacOff()) {
		t.Fatal("the disabled PAC row does not explain itself")
	}
	if err := tester.Click(locales.CopyPAC()); err == nil && pac != 1 {
		t.Fatal("the PAC copy ran without an address")
	}
}

// 日志 is a live tail of the file: the filter narrows it, 复制 copies what is
// shown, and 详细日志 raises the level through the same saved setting.
func TestLogPageFiltersCopiesAndToggles(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.LogPath = "/tmp/logs/gotrust.log"
	state.LogLines = []string{"09:00 ready", "09:01 error: timeout", "09:02 ready"}
	state.LogFilter = "READY"
	var copied string
	var verbose, opened int
	actions := SettingsActions{
		CopyLogs:   func(text string) { copied = text },
		SetVerbose: func(on bool) { verbose++ },
		OpenLogs:   func() { opened++ },
	}

	_, tester := newShell(t, state, actions, PageLogs)
	if !tester.HasText("09:00 ready") || !tester.HasText("09:02 ready") {
		t.Fatalf("the filtered lines are missing; texts: %v", tester.Texts())
	}
	if tester.HasText("09:01 error") {
		t.Fatal("the filter let a non-matching line through")
	}
	if !tester.HasText("/tmp/logs/gotrust.log") {
		t.Fatal("the page does not name the file it tails")
	}

	if err := tester.Click(locales.CopyLogsDesc()); err != nil {
		t.Fatalf("click the log copy: %v", err)
	}
	if want := "09:00 ready\n09:02 ready"; copied != want {
		t.Fatalf("copied %q, want %q", copied, want)
	}
	if err := tester.Click("详细日志"); err != nil {
		t.Fatalf("click verbose: %v", err)
	}
	if verbose != 1 || !state.Draft.Logging.Verbose {
		t.Fatalf("verbose calls = %d state = %v", verbose, state.Draft.Logging.Verbose)
	}
	if err := tester.Click("打开目录"); err != nil {
		t.Fatalf("open the log directory: %v", err)
	}
	if opened != 1 {
		t.Fatalf("open calls = %d", opened)
	}

	state.LogFilter = ""
	tester.Frame()
	if !tester.HasText("09:01 error: timeout") {
		t.Fatal("clearing the filter did not restore the line")
	}
}

// 设置 is the app's own data and its release: the language, the client data
// file and identity, and 关于 with the pages the project lives on.
func TestSettingsPageShowsLocalDataAndAbout(t *testing.T) {
	cfg := config.Default()
	cfg.Identity.DeviceID = "6f1c9a2e4b7d"
	state := NewSettingsState(cfg, "/tmp/logs")
	var opened []string
	actions := SettingsActions{
		OpenURL:  func(url string) { opened = append(opened, url) },
		OpenLogs: func() {},
	}

	_, tester := newShell(t, state, actions, PageSettings)
	for _, wanted := range []string{
		"语言", "跟随系统", "本机数据", "数据文件", "6f1c9a2e4b7d", "打开配置文件",
		"关于", "版本", releaseVersion(), "项目主页", projectHost, "许可证", licenseLine,
	} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from 设置; texts: %v", wanted, tester.Texts())
		}
	}

	if err := tester.Click(locales.ProjectHome()); err != nil {
		t.Fatalf("open the project: %v", err)
	}
	if err := tester.Click(locales.LicenseLabel()); err != nil {
		t.Fatalf("open the license: %v", err)
	}
	if want := []string{projectURL, licenseURL}; len(opened) != len(want) ||
		opened[0] != want[0] || opened[1] != want[1] {
		t.Fatalf("opened = %v, want %v", opened, want)
	}

	// The language moved off 启动: the page is the four toggles only.
	_, startup := newShell(t, state, actions, PageStartup)
	if startup.HasText("跟随系统") {
		t.Fatal("the language switch is still on 启动")
	}
	for _, wanted := range []string{"登录系统时启动", "启动后自动连接", "启动时隐藏窗口", "断线后自动重连"} {
		if !startup.HasText(wanted) {
			t.Fatalf("%q is missing from 启动", wanted)
		}
	}
}

// The data file field shows a home-relative path, as the documentation writes
// it, while the settings keep the absolute one.
func TestShortPathKeepsTheHomeShort(t *testing.T) {
	t.Setenv("HOME", "/home/user")
	if got := shortPath("/home/user/.config/gotrust/client_data.json"); got != "~/.config/gotrust/client_data.json" {
		t.Fatalf("shortPath(home) = %q", got)
	}
	if got := shortPath("/var/lib/gotrust/client_data.json"); got != "/var/lib/gotrust/client_data.json" {
		t.Fatalf("shortPath(other) = %q", got)
	}
	if got := shortPath("/home/userland/client_data.json"); got != "/home/userland/client_data.json" {
		t.Fatalf("shortPath(a lookalike) = %q", got)
	}
}

// A save's result is a toast: it shows once and goes, instead of holding a line
// under the form for the rest of the window's life. The sequence makes the
// same text a second result.
func TestSettingsStatusShowsAsAToast(t *testing.T) {
	state := NewSettingsState(config.Default(), "/home/user/.local/state/gotrust")
	state.SetStatus(locales.SettingsSaved(), false)
	first := state.StatusSeq
	state.SetStatus(locales.SettingsSaved(), false)
	if state.StatusSeq <= first {
		t.Fatal("repeating the same result did not advance the sequence")
	}

	_, tester := newShell(t, state, SettingsActions{}, PageAccount)
	tester.Advance(250 * time.Millisecond)
	tester.Frame()
	if !tester.HasText(locales.SettingsSaved()) {
		t.Fatalf("the result is not on screen: %v", tester.Texts())
	}
	tester.Advance(6 * time.Second)
	tester.Frame()
	if tester.HasText(locales.SettingsSaved()) {
		t.Fatal("the result stayed on screen instead of fading as a toast")
	}
}
