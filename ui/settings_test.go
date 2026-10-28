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
	state.Draft.Profiles[0].Server.Address = " vpn.example.com "
	state.Draft.Profiles[0].Auth.Username = " alice "
	state.Draft.Profiles[0].Auth.Domain = " Radius "
	state.Draft.Profiles[0].Auth.StorePass = true
	state.Draft.Profiles[0].Auth.Password = "secret"
	state.Draft.Profiles[0].Keep.SetEnabled(false)
	state.Ports[0] = "8443"
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
		{"bad port", func(state *SettingsState) { state.Ports[0] = "70000" }},
		{"text port", func(state *SettingsState) { state.Ports[0] = "https" }},
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
	if len(state.Ports) != 2 || len(state.Targets) != 2 {
		t.Fatalf("buffers: ports=%d targets=%d", len(state.Ports), len(state.Targets))
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
	if len(state.Ports) != 2 || len(state.Targets) != 2 {
		t.Fatalf("buffers did not shrink: ports=%d targets=%d", len(state.Ports), len(state.Targets))
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

	tester := ui.NewTester(SettingsView(state, SettingsActions{}), 560, 1600)
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
	state.Ports[0] = "8443"
	tester.Frame()
	if len(state.Methods) != 0 || state.MethodsFrom != "" {
		t.Fatalf("methods = %v from %q, want them dropped", state.Methods, state.MethodsFrom)
	}

	// The picker itself refuses a stale entry, even when a frame has not run
	// since the edit: the chosen method must belong to the current gateway.
	state.SetMethods(methods, state.targetKey(), "已获取 2 种认证方式", false)
	state.Ports[0] = "9443"
	state.SelectAuth(1)
	if got := state.Draft.Profiles[0].Auth.AuthType; got != "" {
		t.Fatalf("auth type = %q, want the stale choice refused", got)
	}
}

// openSettingsPage shows another page of the settings window: the tab is
// clicked, then a frame draws what the choice changed.
func openSettingsPage(t *testing.T, tester *ui.Tester, tab string) {
	t.Helper()
	if err := tester.Click(tab); err != nil {
		t.Fatalf("click the %s tab: %v", tab, err)
	}
	tester.Frame()
}

func TestSettingsViewRendersAndCallsActions(t *testing.T) {
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

	tester := ui.NewTester(SettingsView(state, actions), 560, 1600)
	tester.Frame()

	for _, wanted := range []string{"设置", "当前配置", "连接", "代理", "保活", "启动", "高级", "终端", "下次登录生效"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the settings window; texts: %v", wanted, tester.Texts())
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

	// 新建 makes the draft differ from the file, which is what enables 保存.
	if err := tester.Click("新建"); err != nil {
		t.Fatalf("click new: %v", err)
	}
	if created != 1 {
		t.Fatalf("new profile calls = %d", created)
	}
	tester.Frame()
	if !tester.HasText("未保存的修改") {
		t.Fatalf("the unsaved marker is missing; texts: %v", tester.Texts())
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

	openSettingsPage(t, tester, "启动")
	if err := tester.Click("登录系统时启动"); err != nil {
		t.Fatalf("click autostart: %v", err)
	}
	if !state.Draft.App.Autostart || autostart != 1 {
		t.Fatalf("autostart = %v calls = %d", state.Draft.App.Autostart, autostart)
	}

	openSettingsPage(t, tester, "高级")
	if err := tester.Click("详细日志"); err != nil {
		t.Fatalf("click verbose: %v", err)
	}
	if !state.Draft.Logging.Verbose || verbose != 1 {
		t.Fatalf("verbose = %v calls = %d", state.Draft.Logging.Verbose, verbose)
	}
	if !tester.HasText("/var/log/gotrust") {
		t.Fatal("the log directory is missing")
	}

	// The window's own close control hides it; 关闭 stays on the pages that
	// ask a question of their own, such as the password change.
	if tester.HasText("关闭") {
		t.Fatal("the bottom bar still offers 关闭")
	}
}

// The heading marks a save that would change the file, and only that: a value
// the file already holds, padding, or the language (mygo stores that choice
// itself) is not an unsaved edit.
func TestSettingsChangedFollowsTheSavedFile(t *testing.T) {
	cfg := config.Default()
	state := NewSettingsState(cfg, "/tmp/logs")
	if state.Changed() {
		t.Fatal("a fresh draft is marked unsaved")
	}

	state.Ports[0] = " 443 "
	state.NameText = " " + config.DefaultProfileName + " "
	if state.Changed() {
		t.Fatal("values the file already holds are marked unsaved")
	}
	state.LocaleIndex = 2
	if state.Changed() {
		t.Fatal("the language choice is marked as a settings edit")
	}

	state.Ports[0] = "8443"
	if !state.Changed() {
		t.Fatal("an edited port is not marked")
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

// A page with no saved setting of its own still carries the profile bar, so
// 新建 and 删除 work from every tab.
func TestSettingsProfileBarStaysOnEveryPage(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	tester := ui.NewTester(SettingsView(state, settingsActionsFor(state)), 560, 760)

	for _, tab := range []string{"连接", "代理", "保活", "启动", "高级", "终端"} {
		openSettingsPage(t, tester, tab)
		if !tester.HasText("当前配置") {
			t.Fatalf("%s lost the profile bar", tab)
		}
	}
}

// The bottom bar offers the two actions that move the draft: save it, or
// throw it away. Neither means anything while the draft matches the file.
func TestSettingsSaveAndDiscardFollowTheDraft(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	discarded := 0
	tester := ui.NewTester(SettingsView(state, SettingsActions{
		Discard: func() { state.Reload(config.Default()); discarded++ },
	}), 560, 760)
	tester.Frame()

	if err := tester.Click("放弃修改"); err != nil {
		t.Fatalf("click discard: %v", err)
	}
	if discarded != 0 {
		t.Fatal("放弃修改 ran with no changes to discard")
	}
	if tester.HasText("未保存的修改") {
		t.Fatalf("a clean draft is marked unsaved; texts: %v", tester.Texts())
	}

	state.Ports[0] = "8443"
	tester.Frame()
	if !tester.HasText("未保存的修改") {
		t.Fatalf("an edited port is not marked; texts: %v", tester.Texts())
	}
	if err := tester.Click("放弃修改"); err != nil {
		t.Fatalf("click discard: %v", err)
	}
	if discarded != 1 {
		t.Fatalf("discard calls = %d", discarded)
	}
	if state.Changed() || state.Ports[0] != "443" {
		t.Fatalf("the draft survived the discard: %q", state.Ports[0])
	}
	if tester.HasText("未保存的修改") {
		t.Fatal("the unsaved marker outlived the discard")
	}
}

func TestSettingsKeepAliveCheckboxUpdatesTheProfile(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Draft.Profiles[0].Keep.SetEnabled(true)

	tester := ui.NewTester(SettingsView(state, SettingsActions{}), 560, 1600)
	tester.Frame()
	openSettingsPage(t, tester, "保活")
	if err := tester.Click("启用保活"); err != nil {
		t.Fatalf("click keep-alive: %v", err)
	}
	if state.Draft.Profiles[0].Keep.IsEnabled() {
		t.Fatal("clicking the check box did not disable the keep-alive")
	}
}

func TestSettingsViewHintsAtTheProfileSwitch(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.AddProfile()

	tester := ui.NewTester(SettingsView(state, SettingsActions{}), 560, 720)
	tester.Frame()
	if !tester.HasText("保存后将切换到配置") {
		t.Fatalf("no switch hint; texts: %v", tester.Texts())
	}
}

func TestSettingsReloadDiscardsEdits(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.NameText = "unsaved"
	state.Ports[0] = "9999"

	state.Reload(config.Default())
	if state.NameText != config.DefaultProfileName || state.Ports[0] != "443" {
		t.Fatalf("reload kept edits: %q %q", state.NameText, state.Ports[0])
	}
}

// Choosing a discovered method stores the exact pair the gateway matches;
// choosing 自动 clears it again.
func TestSettingsSelectAuthStoresThePair(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Draft.Profiles[0].Server.Address = "vpn.example.com"
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

// Discovery queries the gateway in the draft, so the port text field has to be
// usable before a save.
func TestSettingsTargetReadsTheDraft(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	if _, err := state.Target(); err == nil {
		t.Fatal("an empty gateway address produced a target")
	}

	state.Draft.Profiles[0].Server.Address = " vpn.example.com "
	state.Ports[0] = "8443"
	profile, err := state.Target()
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	if profile.ServerAddress() != "vpn.example.com" || profile.Server.Port != 8443 {
		t.Fatalf("target = %+v", profile.Server)
	}

	state.Ports[0] = "0"
	if _, err := state.Target(); err == nil {
		t.Fatal("an out-of-range port produced a target")
	}
}

// The settings window shows the picker, the discover button and the hint.
func TestSettingsViewShowsTheAuthPicker(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Draft.Profiles[0].Server.Address = "vpn.example.com"
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

	tester := ui.NewTester(SettingsView(state, actions), 560, 1600)
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
// code read Ports[1] after 新建 → 删除 and panicked on the GTK main thread.
func TestSettingsViewDeletesTheSelectedProfileWithoutAPanic(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	tester := ui.NewTester(SettingsView(state, settingsActionsFor(state)), 560, 1600)

	if err := tester.Click("新建"); err != nil {
		t.Fatalf("click new: %v", err)
	}
	if len(state.Draft.Profiles) != 2 || state.Selected != 1 {
		t.Fatalf("profiles = %v selected = %d", state.Draft.ProfileNames(), state.Selected)
	}

	if err := tester.Click("删除"); err != nil {
		t.Fatalf("click delete: %v", err)
	}
	if names := state.Draft.ProfileNames(); len(names) != 1 || names[0] != config.DefaultProfileName {
		t.Fatalf("profiles = %v", names)
	}
	if state.Selected != 0 {
		t.Fatalf("selected = %d", state.Selected)
	}
	if len(state.Ports) != 1 || len(state.Targets) != 1 {
		t.Fatalf("buffers after delete: ports=%d targets=%d", len(state.Ports), len(state.Targets))
	}

	tester.Frame()
	if !tester.HasText("已删除配置；保存后生效") {
		t.Fatalf("the status line did not survive; texts: %v", tester.Texts())
	}
}

// Ports and keep-alive targets live in buffers indexed like Draft.Profiles.
// Deleting a middle profile has to drop that profile's buffers, not merely
// truncate the end: 默认/B(8443)/C(9443) minus B must leave C holding 9443.
func TestSettingsDeleteKeepsPortsAndTargetsOnTheirProfile(t *testing.T) {
	state := threeProfileState(t)
	state.Select(state.Draft.ProfileIndex("B"))
	state.Ports[state.Selected] = "8443"
	state.Targets[state.Selected] = "http://192.0.2.20/"

	tester := ui.NewTester(SettingsView(state, settingsActionsFor(state)), 560, 1600)
	if err := tester.Click("删除"); err != nil {
		t.Fatalf("click delete: %v", err)
	}

	if names := state.Draft.ProfileNames(); len(names) != 2 || names[0] != config.DefaultProfileName || names[1] != "C" {
		t.Fatalf("profiles = %v", names)
	}
	if state.Selected != 1 {
		t.Fatalf("selected = %d", state.Selected)
	}
	if state.Ports[1] != "9443" {
		t.Fatalf("C's port buffer = %q, want 9443", state.Ports[1])
	}
	if state.Targets[1] != "http://192.0.2.31/" {
		t.Fatalf("C's targets buffer = %q", state.Targets[1])
	}

	cfg, err := state.Apply()
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	profile := cfg.Active()
	if profile.Name != "C" || profile.Server.Port != 9443 {
		t.Fatalf("active = %q port = %d", profile.Name, profile.Server.Port)
	}
	if len(profile.Keep.Targets) != 1 || profile.Keep.Targets[0] != "http://192.0.2.31/" {
		t.Fatalf("active targets = %v", profile.Keep.Targets)
	}
}

// 复制 copies what the editor shows, including the unsaved port and targets,
// and leaves the other profiles' buffers where they were.
func TestSettingsDuplicateCopiesTheEditedBuffers(t *testing.T) {
	state := threeProfileState(t)
	state.Select(state.Draft.ProfileIndex("B"))
	state.Ports[state.Selected] = "8450"
	state.Targets[state.Selected] = "http://192.0.2.20/"

	state.DuplicateProfile()

	copied := state.Draft.ProfileIndex("B 副本")
	if copied < 0 {
		t.Fatalf("profiles = %v", state.Draft.ProfileNames())
	}
	if state.Selected != copied {
		t.Fatalf("selected = %d, copy = %d", state.Selected, copied)
	}
	if state.Ports[copied] != "8450" || state.Targets[copied] != "http://192.0.2.20/" {
		t.Fatalf("copy buffers = %q %q", state.Ports[copied], state.Targets[copied])
	}
	if state.Ports[2] != "9443" || state.Targets[2] != "http://192.0.2.31/" {
		t.Fatalf("C was disturbed: %q %q", state.Ports[2], state.Targets[2])
	}
}

// threeProfileState is 默认/B(8443)/C(9443) with distinct keep-alive targets.
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
	cfg.Profiles[b].Server.Port = 8443
	cfg.Profiles[b].Keep.Targets = []string{"http://192.0.2.21/"}
	c := cfg.ProfileIndex("C")
	cfg.Profiles[c].Server.Port = 9443
	cfg.Profiles[c].Keep.Targets = []string{"http://192.0.2.31/"}
	return NewSettingsState(cfg, "/tmp/logs")
}

// The frame checks and the discovery record have to agree on what "the same
// gateway" means, or a padded port drops a list that belongs to it.
func TestTargetKeyNormalisesThePort(t *testing.T) {
	if got, want := targetKey(" vpn.example.com ", " 0443 "), targetKey("vpn.example.com", "443"); got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
}

// A stored TOTP secret is a credential the form cannot show, so the connection
// section offers a removal action. The removal is a draft edit and takes effect
// with 保存.
func TestSettingsRemoveTheStoredTOTPSecret(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Auth.TOTPSecret = "JBSWY3DPEHPK3PXP"
	state := NewSettingsState(cfg, "/tmp/logs")

	tester := ui.NewTester(SettingsView(state, SettingsActions{}), 560, 1600)
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
	if err := ui.NewTester(SettingsView(state, SettingsActions{}), 560, 1600).Click("移除"); err == nil {
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
		SelfID: "self",
		Trusted: []session.Terminal{{
			ID:         "trusted-1",
			DeviceName: "desk-01",
			OS:         "Linux",
			OSVersion:  "7.3.0",
		}},
		Temporary: []session.Terminal{{
			ID:               "self",
			DeviceName:       "desk-02",
			OS:               "Linux",
			OSVersion:        "7.3.0",
			OnlineStatus:     true,
			LastLoginAddress: "Example City",
			LastLoginIP:      "192.0.2.10",
			LastLoginTime:    "2026-10-25 13:17:59",
		}},
	}
	var changed bool
	refreshed := 0
	actions := SettingsActions{
		SetTrusted:       func(trusted bool) { changed = trusted },
		RefreshTerminals: func() { refreshed++ },
	}

	tester := ui.NewTester(SettingsView(state, actions), 560, 1800)
	tester.Frame()
	openSettingsPage(t, tester, "终端")
	for _, wanted := range []string{"授信终端 1 · 临时终端 1", "desk-01", "desk-02", "当前设备"} {
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
	tester = ui.NewTester(SettingsView(state, actions), 560, 1800)
	tester.Frame()
	openSettingsPage(t, tester, "终端")
	if !tester.HasText("连接后可读取终端列表") {
		t.Fatal("the offline hint is missing")
	}
	if err := tester.Click("刷新"); err == nil && refreshed != 0 {
		t.Fatal("the refresh button ran while offline")
	}
}

func TestIdentitySummaryOmitsTheEmptyProcess(t *testing.T) {
	got := identitySummary(config.IdentityConfig{OSFamily: config.IdentityOSLinux, ClientVersion: "2.5.16.41"})
	if got != "Linux · 2.5.16.41" {
		t.Fatalf("summary = %q", got)
	}
	withProcess := identitySummary(config.IdentityConfig{
		OSFamily:      config.IdentityOSWindows,
		ClientVersion: "2.5.16.41",
		ProcessName:   "agent",
	})
	if withProcess != "Windows · 2.5.16.41 · agent" {
		t.Fatalf("summary = %q", withProcess)
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

	tester := ui.NewTester(SettingsView(state, SettingsActions{}), 600, 760)
	tester.Frame()
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
