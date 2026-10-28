// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
)

// completeProfile is a profile a login can be built from.
func completeProfile() config.Profile {
	profile := config.NewProfile(config.DefaultProfileName)
	profile.Server.Address = "vpn.example.com"
	profile.Server.Port = 443
	profile.Auth.Username = "alice"
	return profile
}

// The window's sentence and the library's validation must agree on when a
// profile is usable: the first is shown, the second is the authority.
func TestProfileProblemMatchesValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*config.Profile)
	}{
		{"complete", func(*config.Profile) {}},
		{"no address", func(p *config.Profile) { p.Server.Address = "" }},
		{"blank address", func(p *config.Profile) { p.Server.Address = "   " }},
		{"no username", func(p *config.Profile) { p.Auth.Username = "" }},
		{"port zero", func(p *config.Profile) { p.Server.Port = 0 }},
		{"port too large", func(p *config.Profile) { p.Server.Port = 70000 }},
	}

	for _, tc := range cases {
		profile := completeProfile()
		tc.mutate(&profile)

		problem := ProfileProblem(profile)
		err := profile.Validate()
		if (problem == "") != (err == nil) {
			t.Fatalf("%s: ProfileProblem = %q, Validate = %v", tc.name, problem, err)
		}
		if problem != "" && !strings.HasPrefix(problem, "请填写") && !strings.HasPrefix(problem, "端口") {
			t.Fatalf("%s: %q is not a sentence the window can show", tc.name, problem)
		}
	}
}

// A profile without a gateway or an account opens on the first-run form, and
// the form cannot be submitted until it has what a login needs.
func TestSetupPanelAppearsWhileTheProfileIsIncomplete(t *testing.T) {
	cfg := config.Default()
	state := State{Config: cfg, Setup: NewSetupDraft(cfg.Active())}

	var saved *SetupDraft
	var connect bool
	actions := Actions{SaveSetup: func(draft SetupDraft, withLogin bool) {
		saved = &draft
		connect = withLogin
	}}

	tester := ui.NewTester(View(&state, actions), 540, 640)
	tester.Frame()

	for _, wanted := range []string{"连接网关", "网关地址", "端口", "用户名", "保存并连接", "更多设置…"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the first-run form", wanted)
		}
	}
	if tester.HasText("已获授权的资源") {
		t.Fatal("the empty resource list is shown behind the first-run form")
	}

	// An empty form cannot be saved.
	if err := tester.Click("保存并连接"); err != nil {
		t.Fatalf("click save and connect: %v", err)
	}
	if saved != nil {
		t.Fatal("an incomplete form was submitted")
	}

	state.Setup.Address = "vpn.example.com"
	state.Setup.Port = "8443"
	state.Setup.Username = "alice"
	tester = ui.NewTester(View(&state, actions), 540, 640)
	tester.Frame()
	if err := tester.Click("保存并连接"); err != nil {
		t.Fatalf("click save and connect: %v", err)
	}
	if saved == nil {
		t.Fatal("save and connect did not reach the callback")
	}
	if !connect {
		t.Fatal("保存并连接 saved without asking for a login")
	}
	if saved.Address != "vpn.example.com" || saved.Port != "8443" || saved.Username != "alice" {
		t.Fatalf("submitted draft = %+v", *saved)
	}

	// 仅保存 keeps the same values and does not log in.
	saved, connect = nil, true
	if err := tester.Click("仅保存"); err != nil {
		t.Fatalf("click save: %v", err)
	}
	if saved == nil || connect {
		t.Fatalf("仅保存 saved = %v connect = %v", saved, connect)
	}
}

// A profile a login can be built from shows the connection controls and never
// the first-run form.
func TestCompleteProfileShowsTheConnectionControls(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0] = completeProfile()

	state := State{Config: cfg}
	tester := ui.NewTester(View(&state, Actions{}), 540, 640)
	tester.Frame()

	if tester.HasText("保存并连接") {
		t.Fatal("the first-run form appeared for a complete profile")
	}
	if !tester.HasText("已获授权的资源") {
		t.Fatal("the connection page is missing")
	}
}

// The form lands on the active profile, and a password reaches the file only
// when the user asked to keep it.
func TestSetupDraftAppliesToTheActiveProfile(t *testing.T) {
	cfg := config.Default()
	draft := NewSetupDraft(cfg.Active())
	draft.Address = "  vpn.example.com "
	draft.Port = "8443"
	draft.Username = " alice "
	draft.Password = "s3cret"
	draft.StorePass = true

	if err := draft.Apply(&cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	profile := cfg.Active()
	if profile.Server.Address != "vpn.example.com" || profile.Server.Port != 8443 ||
		profile.Auth.Username != "alice" {
		t.Fatalf("profile = %+v", profile)
	}
	if !profile.Auth.StorePass || profile.Auth.Password != "s3cret" {
		t.Fatalf("stored password = %q (store %v)", profile.Auth.Password, profile.Auth.StorePass)
	}

	// The same form without the check box leaves no password in the file.
	cfg = config.Default()
	draft = NewSetupDraft(cfg.Active())
	draft.Address = "vpn.example.com"
	draft.Username = "alice"
	draft.Password = "s3cret"
	draft.StorePass = false
	if err := draft.Apply(&cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if cfg.Active().Auth.Password != "" {
		t.Fatal("an unstored password stayed in the configuration")
	}
	if disk := cfg.ForDisk().Active(); disk.Auth.Password != "" {
		t.Fatal("an unstored password reached the settings file")
	}
}

// A form filled for one profile must not write onto another: the user may have
// switched the active profile from the tray while the form was open.
func TestSetupDraftRefusesASwitchedProfile(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles = append(cfg.Profiles, config.NewProfile("工作"))
	cfg.ActiveProfile = "工作"

	draft := NewSetupDraft(cfg.Profiles[0])
	draft.Address = "vpn.example.com"
	draft.Username = "alice"
	if err := draft.Apply(&cfg); err == nil {
		t.Fatal("a draft written for 默认 was applied to 工作")
	}
	if cfg.Active().Server.Address != "" {
		t.Fatal("the form changed a profile it was not filled for")
	}
}
