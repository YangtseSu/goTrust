// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

func TestDecideAutoConnect(t *testing.T) {
	ready := config.Default()
	ready.App.AutoConnect = true
	ready.Profiles[0].Server.Address = "vpn.example.com"
	ready.Profiles[0].Auth.Username = "alice"

	cases := []struct {
		name       string
		mutate     func(*config.Config)
		clientData bool
		want       bool
		reason     string
	}{
		{
			name:   "option off",
			mutate: func(cfg *config.Config) { cfg.App.AutoConnect = false },
		},
		{
			name:   "no gateway",
			mutate: func(cfg *config.Config) { cfg.Profiles[0].Server.Address = "" },
			reason: "网关",
		},
		{
			name:   "no username",
			mutate: func(cfg *config.Config) { cfg.Profiles[0].Auth.Username = "" },
			reason: "用户名",
		},
		{
			name:   "no secret",
			mutate: func(cfg *config.Config) {},
			reason: "密码",
		},
		{
			name: "stored password",
			mutate: func(cfg *config.Config) {
				cfg.Profiles[0].Auth.StorePass = true
				cfg.Profiles[0].Auth.Password = "secret"
			},
			want: true,
		},
		{
			name: "unstored password is not stored",
			mutate: func(cfg *config.Config) {
				cfg.Profiles[0].Auth.StorePass = false
				cfg.Profiles[0].Auth.Password = "secret"
			},
			reason: "密码",
		},
		{
			name:       "stored session",
			mutate:     func(cfg *config.Config) {},
			clientData: true,
			want:       true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := ready.Clone()
			testCase.mutate(&cfg)
			decision := decideAutoConnect(cfg, testCase.clientData)
			if decision.Connect != testCase.want {
				t.Fatalf("connect = %v, want %v (reason %q)", decision.Connect, testCase.want, decision.Reason)
			}
			if testCase.reason != "" && !strings.Contains(decision.Reason, testCase.reason) {
				t.Fatalf("reason = %q, want it to mention %q", decision.Reason, testCase.reason)
			}
		})
	}
}

func TestHasClientDataIsPerProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := config.Default()
	application := &app{cfg: cfg}
	if application.hasClientData() {
		t.Fatal("reported a stored session before one exists")
	}

	path, err := cfg.ClientDataPath()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !application.hasClientData() {
		t.Fatal("a stored session was not seen")
	}
}

func TestParseKeepAlive(t *testing.T) {
	cases := []struct {
		line   string
		want   string
		wantOK bool
	}{
		{
			line:   "2026/10/24 01:00:00 KeepAlive: OK status=200 target=http://192.0.2.10/",
			want:   "OK 200 · http://192.0.2.10/",
			wantOK: true,
		},
		{
			line:   "2026/10/24 01:00:00 KeepAlive: target=http://192.0.2.10/ err=dial timeout",
			want:   "失败 · http://192.0.2.10/",
			wantOK: true,
		},
		{
			line:   "2026/10/24 01:00:00 KeepAlive using UDP: OK",
			want:   "DNS OK",
			wantOK: true,
		},
		{
			line:   "2026/10/24 01:00:00 goTrust ready",
			wantOK: false,
		},
	}
	for _, testCase := range cases {
		got, ok := parseKeepAlive(testCase.line)
		if ok != testCase.wantOK {
			t.Fatalf("%q: ok = %v, want %v", testCase.line, ok, testCase.wantOK)
		}
		if ok && got != testCase.want {
			t.Fatalf("%q: result = %q, want %q", testCase.line, got, testCase.want)
		}
	}
}

func TestCfgForDiskClearsUnstoredPasswords(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Auth.StorePass = true
	cfg.Profiles[0].Auth.Password = "stored"
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	cfg.Profiles[1].Auth.StorePass = false
	cfg.Profiles[1].Auth.Password = "memory-only"

	disk := cfgForDisk(cfg)
	if disk.Profiles[0].Auth.Password != "stored" {
		t.Fatal("a stored password was cleared from the file")
	}
	if disk.Profiles[1].Auth.Password != "" {
		t.Fatal("an unstored password reached the file")
	}
	if cfg.Profiles[1].Auth.Password != "memory-only" {
		t.Fatal("the in-memory password was cleared")
	}
}

func TestActiveProfileFeedsTheSession(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "first.example.com"
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	cfg.Profiles[1].Server.Address = "second.example.com"

	application := &app{cfg: cfg}
	if got := application.currentConfig().Active().ServerAddress(); got != "second.example.com" {
		t.Fatalf("active server = %q", got)
	}

	if err := cfg.SelectProfile(config.DefaultProfileName); err != nil {
		t.Fatalf("select: %v", err)
	}
	application.applyConfig(cfg)
	if got := application.currentConfig().Active().ServerAddress(); got != "first.example.com" {
		t.Fatalf("active server after switch = %q", got)
	}
}

func TestTooltipMirrorsPhaseAndKeepAlive(t *testing.T) {
	application := &app{}
	application.view.Session = session.State{Phase: session.PhaseOnline, ClientIP: "203.0.113.4"}
	application.view.KeepAlive = "OK 200 · http://192.0.2.10/"

	tooltip := application.tooltip(application.view)
	if !strings.Contains(tooltip, "已连接 203.0.113.4") {
		t.Fatalf("tooltip = %q", tooltip)
	}
	if !strings.Contains(tooltip, "保活: OK 200") {
		t.Fatalf("tooltip = %q", tooltip)
	}

	application.view.Session = session.State{Phase: session.PhaseFailed, Detail: "bad password"}
	application.view.KeepAlive = ""
	if got := application.tooltip(application.view); !strings.Contains(got, "失败: bad password") {
		t.Fatalf("tooltip = %q", got)
	}
}

func TestObserveLogLineUpdatesTheView(t *testing.T) {
	application := &app{}
	application.observeLogLine("2026/10/24 01:00:00 KeepAlive: OK status=200 target=http://192.0.2.10/")
	if !strings.HasPrefix(application.view.KeepAlive, "OK 200") {
		t.Fatalf("keep-alive = %q", application.view.KeepAlive)
	}

	application.observeLogLine("2026/10/24 01:00:30 goTrust ready")
	if gotrustui.TrayStatus(application.view.Session) != "未连接" {
		t.Fatalf("an ordinary line changed the state")
	}
}

// Saving the settings window must write TOML, make the selected profile active
// and leave the in-memory password usable for this run.
func TestSaveSettingsWritesAndApplies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Default()
	application := &app{cfg: cfg, configPath: path}
	application.view.Config = cfg

	state := gotrustui.NewSettingsState(cfg, "/tmp/logs")
	state.NameText = "工作"
	state.Draft.Profiles[0].Server.Address = "vpn.example.com"
	state.Draft.Profiles[0].Auth.Username = "alice"
	state.Draft.Profiles[0].Auth.Password = "memory-only"
	state.Ports[0] = "8443"
	application.settings = state

	application.saveSettings()

	if state.Status != "设置已保存" {
		t.Fatalf("status = %q", state.Status)
	}
	if application.currentConfig().ActiveProfile != "工作" {
		t.Fatalf("active = %q", application.currentConfig().ActiveProfile)
	}
	if got := application.currentConfig().Active().Auth.Password; got != "memory-only" {
		t.Fatalf("in-memory password = %q", got)
	}

	onDisk, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	profile := onDisk.Active()
	if profile.Name != "工作" || profile.Server.Address != "vpn.example.com" ||
		profile.Server.Port != 8443 || profile.Auth.Username != "alice" {
		t.Fatalf("on-disk profile = %+v", profile)
	}
	if profile.Auth.Password != "" {
		t.Fatal("an unstored password reached the settings file")
	}
}

// A save that fails validation must not touch the file or the live settings.
func TestSaveSettingsRejectsBadValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Default()
	application := &app{cfg: cfg, configPath: path}
	application.view.Config = cfg

	state := gotrustui.NewSettingsState(cfg, "/tmp/logs")
	state.Ports[0] = "0"
	application.settings = state

	application.saveSettings()

	if state.Status == "" || state.Status == "设置已保存" {
		t.Fatalf("status = %q", state.Status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a rejected save wrote the file: %v", err)
	}
	if application.currentConfig().Active().Server.Port != 443 {
		t.Fatal("a rejected save changed the live settings")
	}
}
