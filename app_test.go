// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		name   string
		mutate func(*config.Config)
		want   bool
		reason string
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
			// A session belongs to its connection; stored client data alone
			// is not a reason to connect without a password.
			name:   "stored session is not a credential",
			mutate: func(cfg *config.Config) {},
			reason: "密码",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := ready.Clone()
			testCase.mutate(&cfg)
			decision := decideAutoConnect(cfg)
			if decision.Connect != testCase.want {
				t.Fatalf("connect = %v, want %v (reason %q)", decision.Connect, testCase.want, decision.Reason)
			}
			if testCase.reason != "" && !strings.Contains(decision.Reason, testCase.reason) {
				t.Fatalf("reason = %q, want it to mention %q", decision.Reason, testCase.reason)
			}
		})
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
	application.view.Session = session.State{
		Phase:    session.PhaseOnline,
		ClientIP: "203.0.113.4",
		KeepAlive: session.KeepAliveState{
			Target: "http://192.0.2.10/",
			Status: 200,
			OK:     true,
			At:     time.Now(),
		},
	}

	tooltip := application.tooltip(application.view)
	if !strings.Contains(tooltip, "已连接 203.0.113.4") {
		t.Fatalf("tooltip = %q", tooltip)
	}
	if !strings.Contains(tooltip, "保活: OK 200") {
		t.Fatalf("tooltip = %q", tooltip)
	}

	application.view.Session = session.State{Phase: session.PhaseFailed, Detail: "bad password"}
	if got := application.tooltip(application.view); !strings.Contains(got, "失败: bad password") {
		t.Fatalf("tooltip = %q", got)
	}
}

// The reconnect phase is an active session: 断开 must stop it, and the tray
// must count down rather than claim the user is disconnected.
func TestReconnectIsAnActiveSession(t *testing.T) {
	for _, phase := range []session.Phase{session.PhaseOnline, session.PhaseConnecting, session.PhaseReconnecting} {
		if !sessionActive(phase) {
			t.Fatalf("phase %q is not treated as active", phase)
		}
	}
	for _, phase := range []session.Phase{session.PhaseIdle, session.PhaseFailed} {
		if sessionActive(phase) {
			t.Fatalf("phase %q is treated as active", phase)
		}
	}

	state := session.State{
		Phase: session.PhaseReconnecting,
		Reconnect: session.ReconnectState{
			Attempt: 2,
			Max:     5,
			NextAt:  time.Now().Add(4 * time.Second),
		},
	}
	if got := gotrustui.TrayStatus(state); !strings.Contains(got, "重连中 (2/5") {
		t.Fatalf("tray status = %q, want the ladder position", got)
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
