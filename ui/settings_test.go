// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/config"
)

func TestSettingsApplyBuildsAProfile(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
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
		{"no listener", func(state *SettingsState) {
			state.Draft.Proxy.SOCKS5Bind = ""
			state.Draft.Proxy.HTTPBind = ""
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

func TestSettingsViewRendersAndCallsActions(t *testing.T) {
	var saved, closed, created, autostart, verbose int
	state := NewSettingsState(config.Default(), "/var/log/goTrust")
	state.Draft.Profiles[0].Auth.Password = "hunter2"

	actions := SettingsActions{
		Save:       func() { saved++ },
		Close:      func() { closed++ },
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

	for _, wanted := range []string{"设置", "配置", "连接", "代理", "保活", "启动", "高级", "保存"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the settings window", wanted)
		}
	}
	if tester.HasText("hunter2") {
		t.Fatal("the password is painted as readable text")
	}
	if tester.HasText("/var/log/goTrust") == false {
		t.Fatal("the log directory is missing")
	}

	if err := tester.Click("保存"); err != nil {
		t.Fatalf("click save: %v", err)
	}
	if saved != 1 {
		t.Fatalf("save calls = %d", saved)
	}

	if err := tester.Click("新建"); err != nil {
		t.Fatalf("click new: %v", err)
	}
	if created != 1 {
		t.Fatalf("new profile calls = %d", created)
	}

	if err := tester.Click("登录系统时启动"); err != nil {
		t.Fatalf("click autostart: %v", err)
	}
	if !state.Draft.App.Autostart || autostart != 1 {
		t.Fatalf("autostart = %v calls = %d", state.Draft.App.Autostart, autostart)
	}

	if err := tester.Click("详细日志"); err != nil {
		t.Fatalf("click verbose: %v", err)
	}
	if !state.Draft.Logging.Verbose || verbose != 1 {
		t.Fatalf("verbose = %v calls = %d", state.Draft.Logging.Verbose, verbose)
	}

	if err := tester.Click("关闭"); err != nil {
		t.Fatalf("click close: %v", err)
	}
	if closed != 1 {
		t.Fatalf("close calls = %d", closed)
	}
}

func TestSettingsKeepAliveCheckboxUpdatesTheProfile(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	state.Draft.Profiles[0].Keep.SetEnabled(true)

	tester := ui.NewTester(SettingsView(state, SettingsActions{}), 560, 1600)
	tester.Frame()
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
