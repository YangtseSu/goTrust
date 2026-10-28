// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
)

// passwordTestStrategy is the smallest policy the panels can render: a length,
// a digit and the portal-only weak-password rule.
func passwordTestStrategy() session.PasswordStrategy {
	var strategy session.PasswordStrategy
	strategy.EnablePswLength.Enable = true
	strategy.EnablePswLength.Value = 10
	strategy.EnablePswCombination.Enable = true
	strategy.EnablePswCombination.Value.EnableNumber = true
	strategy.NotBelongToWeakDb = true
	return strategy
}

// The forced panel shows the gateway's message, the account, the rules and the
// skip box only when the gateway allows it, and its submit reaches the app.
func TestPasswordPanelRendersAndSubmits(t *testing.T) {
	var submitted, cancelled int

	state := State{
		Session: session.State{Phase: session.PhaseConnecting, Detail: "logging in"},
		Config:  config.Default(),
		Password: &PasswordPrompt{
			Message:  "密码已过期，请修改",
			Username: "alice",
			Prompt:   "密码需包含至少两类字符",
			CanSkip:  true,
			Strategy: passwordTestStrategy(),
		},
	}
	actions := Actions{
		SubmitPassword: func() { submitted++ },
		CancelPassword: func() { cancelled++ },
	}

	tester := ui.NewTester(View(&state, actions), 520, 640)
	tester.Frame()

	for _, wanted := range []string{
		"重置密码",
		"密码已过期，请修改",
		"账号：alice",
		"密码规则",
		"密码长度至少10位",
		"密码不能属于常见弱密码",
		"密码需包含至少两类字符",
		"跳过本次修改（网关允许）",
	} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the password panel; texts: %v", wanted, tester.Texts())
		}
	}
	if tester.HasText("新账号名") {
		t.Fatal("the account field appeared without code 75500402")
	}

	if err := tester.Click("重置并登录"); err != nil {
		t.Fatalf("click submit: %v", err)
	}
	if submitted != 0 {
		t.Fatal("an empty form was submitted")
	}
	state.Password.OldPassword = "old"
	state.Password.NewPassword = "new"
	state.Password.Confirm = "new"
	tester = ui.NewTester(View(&state, actions), 520, 640)
	tester.Frame()
	if err := tester.Click("重置并登录"); err != nil {
		t.Fatalf("click submit: %v", err)
	}
	if submitted != 1 {
		t.Fatalf("submit calls = %d", submitted)
	}
	if err := tester.Click("取消登录"); err != nil {
		t.Fatalf("click cancel: %v", err)
	}
	if cancelled != 1 {
		t.Fatalf("cancel calls = %d", cancelled)
	}
}

// The account rename field belongs to code 75500402 (and a skippable change),
// where the portal may change the account name together with the password.
func TestPasswordPanelShowsTheAccountField(t *testing.T) {
	state := State{
		Config: config.Default(),
		Password: &PasswordPrompt{
			ShowAccount: true,
			Strategy:    passwordTestStrategy(),
		},
	}
	tester := ui.NewTester(View(&state, Actions{}), 520, 640)
	tester.Frame()
	if !tester.HasText("新账号名") {
		t.Fatalf("the account field is missing; texts: %v", tester.Texts())
	}
	if tester.HasText("跳过本次修改（网关允许）") {
		t.Fatal("the skip box appeared without the gateway's permission")
	}
}

// Validation catches what the window can see. The portal's own rules (weak
// database, history, keyboard runs) are never refused locally.
func TestPasswordPromptValidation(t *testing.T) {
	base := func() *PasswordPrompt {
		return &PasswordPrompt{
			Username: "alice",
			Strategy: passwordTestStrategy(),
		}
	}

	prompt := base()
	if err := prompt.Validate(); err == nil || !strings.Contains(err.Error(), "旧密码") {
		t.Fatalf("empty form validation = %v", err)
	}

	prompt = base()
	prompt.OldPassword = "old-password"
	prompt.NewPassword = "short1"
	prompt.Confirm = "short1"
	if err := prompt.Validate(); err == nil || !strings.Contains(err.Error(), "长度不足") {
		t.Fatalf("short password validation = %v", err)
	}

	prompt = base()
	prompt.OldPassword = "old-password"
	prompt.NewPassword = "Long-enough1"
	prompt.Confirm = "Long-enough2"
	if err := prompt.Validate(); err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("mismatch validation = %v", err)
	}

	prompt = base()
	prompt.OldPassword = "Long-enough1"
	prompt.NewPassword = "Long-enough1"
	prompt.Confirm = "Long-enough1"
	if err := prompt.Validate(); err == nil || !strings.Contains(err.Error(), "相同") {
		t.Fatalf("unchanged password validation = %v", err)
	}

	prompt = base()
	prompt.OldPassword = "old-password"
	prompt.NewPassword = "Long-enough1"
	prompt.Confirm = "Long-enough1"
	if err := prompt.Validate(); err != nil {
		t.Fatalf("valid form rejected: %v", err)
	}

	// The reset form's account field is required whenever it is shown.
	prompt = base()
	prompt.ShowAccount = true
	prompt.OldPassword = "old-password"
	prompt.NewPassword = "Long-enough1"
	prompt.Confirm = "Long-enough1"
	if err := prompt.Validate(); err == nil || !strings.Contains(err.Error(), "账号名") {
		t.Fatalf("missing account name validation = %v", err)
	}
	prompt.NewUserName = "alice-new"
	if err := prompt.Validate(); err != nil {
		t.Fatalf("form with an account name rejected: %v", err)
	}
}

// The settings window only offers the voluntary change while connected, and
// its panel replaces the sections with a way back.
func TestSettingsPasswordPanel(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Auth.Username = "alice"

	state := NewSettingsState(cfg, "/tmp/logs")
	state.Online = false
	tester := ui.NewTester(SettingsView(state, SettingsActions{}), 600, 760)
	tester.Frame()
	if !tester.HasText("重置密码") {
		t.Fatalf("the password button is missing; texts: %v", tester.Texts())
	}
	if !tester.HasText("连接后才能重置密码") {
		t.Fatal("the offline hint is missing")
	}

	opened := 0
	state.Online = true
	tester = ui.NewTester(SettingsView(state, SettingsActions{OpenPassword: func() { opened++ }}), 600, 760)
	tester.Frame()
	if err := tester.Click("重置密码"); err != nil {
		t.Fatalf("click the password button: %v", err)
	}
	if opened != 1 {
		t.Fatalf("open calls = %d", opened)
	}

	state.Password = &PasswordPrompt{Username: "alice", Strategy: passwordTestStrategy()}
	state.PasswordStatus = "读取密码规则失败: offline"
	state.PasswordError = true
	cancelled := 0
	tester = ui.NewTester(SettingsView(state, SettingsActions{CancelPassword: func() { cancelled++ }}), 600, 760)
	tester.Frame()
	for _, wanted := range []string{"重置密码", "旧密码", "新密码", "确认密码", "读取密码规则失败: offline", "取消"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the settings password panel; texts: %v", wanted, tester.Texts())
		}
	}
	if tester.HasText("保存") {
		t.Fatal("the settings sections are still visible behind the password panel")
	}
	if err := tester.Click("取消"); err != nil {
		t.Fatalf("click back: %v", err)
	}
	if cancelled != 1 {
		t.Fatalf("cancel calls = %d", cancelled)
	}
}
