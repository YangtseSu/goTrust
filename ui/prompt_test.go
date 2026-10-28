// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
)

// A device-binding request must name the device, explain the consequence and
// reach both answers; a gateway that does not ask never shows the panel.
func TestDeviceBindPanelRendersAndAnswers(t *testing.T) {
	var allowed, cancelled int

	state := State{
		Session:    session.State{Phase: session.PhaseConnecting, Detail: "binding"},
		Config:     config.Default(),
		DeviceBind: &DeviceBindPrompt{DeviceID: "0123456789ABCDEF0123456789ABCDEF"},
	}
	actions := Actions{
		AllowDeviceBind:  func() { allowed++ },
		CancelDeviceBind: func() { cancelled++ },
	}

	tester := ui.NewTester(View(&state, actions), 540, 640)
	tester.Frame()
	for _, wanted := range []string{"绑定此设备", "0123456789ABCDEF0123456789ABCDEF", "允许绑定"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the binding panel", wanted)
		}
	}
	if err := tester.Click("允许绑定"); err != nil {
		t.Fatalf("click allow: %v", err)
	}
	if err := tester.Click("取消登录"); err != nil {
		t.Fatalf("click cancel: %v", err)
	}
	if allowed != 1 || cancelled != 1 {
		t.Fatalf("allowed = %d cancelled = %d", allowed, cancelled)
	}
}

// The code panel has to carry the gateway's message, offer the skip choice only
// when the gateway allows it, and reach the submit callback.
func TestCodePanelRendersAndSubmits(t *testing.T) {
	var submitted, cancelled int

	state := State{
		Session: session.State{Phase: session.PhaseConnecting, Detail: "logging in"},
		Config:  config.Default(),
		Code: &CodePrompt{
			Kind:    session.CodeSMS,
			Message: "Please enter the SMS verification code:",
			CanSkip: true,
		},
	}
	actions := Actions{
		SubmitCode: func() { submitted++ },
		CancelCode: func() { cancelled++ },
	}

	tester := ui.NewTester(View(&state, actions), 520, 560)
	tester.Frame()

	for _, wanted := range []string{"短信验证码", "Please enter the SMS verification code:", "跳过以后的二次认证"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the code panel", wanted)
		}
	}
	if tester.HasText("TOTP 密钥") {
		t.Fatal("a non-TOTP challenge offered a TOTP secret")
	}

	// An empty code cannot be submitted; a typed one can.
	if err := tester.Click("提交"); err != nil {
		t.Fatalf("click submit: %v", err)
	}
	if submitted != 0 {
		t.Fatal("an empty code was submitted")
	}
	state.Code.Code = "123456"
	tester = ui.NewTester(View(&state, actions), 520, 560)
	tester.Frame()
	if err := tester.Click("提交"); err != nil {
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

// A gateway that does not allow skipping must not show the check box, and a
// TOTP challenge is the only one that offers a secret to store.
func TestCodePanelSkipAndTOTPSecret(t *testing.T) {
	state := State{
		Config: config.Default(),
		Code: &CodePrompt{
			Kind:            session.CodeTOTP,
			Message:         "Please enter the TOTP token:",
			AllowTOTPSecret: true,
		},
	}

	tester := ui.NewTester(View(&state, Actions{}), 520, 560)
	tester.Frame()
	if tester.HasText("跳过以后的二次认证") {
		t.Fatal("the skip box appeared without the gateway's permission")
	}
	if !tester.HasText("保存 TOTP 密钥，以后由 goTrust 自动生成验证码") {
		t.Fatal("a TOTP challenge did not offer to store the secret")
	}
	if _, ok := tester.Find("TOTP 密钥"); ok {
		t.Fatal("the secret field is shown before the user opts in")
	}

	state.Code.StoreSecret = true
	tester = ui.NewTester(View(&state, Actions{}), 520, 560)
	tester.Frame()
	if _, ok := tester.Find("TOTP 密钥"); !ok {
		t.Fatal("opting in did not reveal the secret field")
	}
}

// The SSO panel shows the URL, retries the browser and submits the callback.
func TestExternalPanelRendersAndActs(t *testing.T) {
	var opened, submitted, cancelled int

	state := State{
		Config: config.Default(),
		External: &ExternalLogin{
			Kind:     session.ExternalCAS,
			LoginURL: "https://sso.example.com/cas/login?service=aTrust",
			Message:  "Please enter the callback url:",
			Callback: "https://vpn.example.com/passport/v1/auth/cas?ticket=ST-1",
		},
	}
	actions := Actions{
		OpenExternal:   func() { opened++ },
		SubmitExternal: func() { submitted++ },
		CancelExternal: func() { cancelled++ },
	}

	tester := ui.NewTester(View(&state, actions), 520, 560)
	tester.Frame()

	for _, wanted := range []string{"统一身份认证（CAS）", "https://sso.example.com/cas/login?service=aTrust"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing from the external login panel", wanted)
		}
	}

	if err := tester.Click("在浏览器中打开"); err != nil {
		t.Fatalf("click open: %v", err)
	}
	if opened != 1 {
		t.Fatalf("open calls = %d", opened)
	}
	if err := tester.Click("提交"); err != nil {
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

// A login question keeps the session's facts: the header stays, and the count
// of what the gateway already granted is still on screen under the panel.
func TestLoginPanelKeepsHeaderAndGrantedCount(t *testing.T) {
	state := State{
		Session: session.State{
			Phase:  session.PhaseConnecting,
			Detail: "需要二次验证",
			Resources: session.Resources{Entries: []session.ResourceEntry{
				{Kind: session.ResourceApp, Title: "知识库", Copy: "https://wiki.example.com"},
				{Kind: session.ResourceDomain, Title: "build.example.com", Copy: "build.example.com"},
				{Kind: session.ResourceIP, Title: "192.0.2.10", Copy: "192.0.2.10"},
			}},
		},
		Config: config.Default(),
		Code:   &CodePrompt{Kind: session.CodeSMS},
	}

	tester := ui.NewTester(View(&state, Actions{}), 540, 640)
	tester.Frame()
	for _, wanted := range []string{"连接中", "已获授权 3 项资源"} {
		if !tester.HasText(wanted) {
			t.Fatalf("%q is missing while a panel is up: %v", wanted, tester.Texts())
		}
	}
}

// 取消登录 is not one more answer: it sits on the left, 提交 on the right, and
// the secondary answers between them stay plain text.
func TestPanelActionsPutCancelAwayFromSubmit(t *testing.T) {
	state := State{
		Config: config.Default(),
		Code:   &CodePrompt{Kind: session.CodeSMS, Code: "123456"},
	}
	actions := Actions{CancelCode: func() {}, CancelCaptcha: func() {}}

	tester := ui.NewTester(View(&state, actions), 540, 640)
	tester.Frame()
	cancel, ok := tester.Find("取消登录")
	if !ok {
		t.Fatal("the panel has no 取消登录")
	}
	submit, ok := tester.Find("提交")
	if !ok {
		t.Fatal("the panel has no 提交")
	}
	if cancel.X >= submit.X {
		t.Fatalf("取消登录 at x=%v is not left of 提交 at x=%v", cancel.X, submit.X)
	}

	state.Captcha = &Captcha{Image: testBitmap(t, 100, 50), Width: 100, Height: 50, Points: []Point{{X: 10, Y: 10}}}
	state.Code = nil
	tester = ui.NewTester(View(&state, actions), 540, 640)
	tester.Frame()
	cancel, _ = tester.Find("取消登录")
	undo, ok := tester.Find("撤销")
	if !ok {
		t.Fatal("the captcha panel has no 撤销")
	}
	submit, _ = tester.Find("提交")
	if !(cancel.X < undo.X && undo.X < submit.X) {
		t.Fatalf("captcha actions run in the wrong order: 取消登录 x=%v 撤销 x=%v 提交 x=%v", cancel.X, undo.X, submit.X)
	}
}

// The chooser lists every offered method and reports the picked index.
func TestChoicePanelCallsBackWithTheIndex(t *testing.T) {
	var picked []int
	var cancelled int

	methods := []session.AuthMethod{
		{Name: "密码登录", Type: "auth/psw", Domain: "local"},
		{Name: "统一认证", Type: "auth/cas", Domain: "local"},
	}
	state := State{
		Config: config.Default(),
		Choice: &AuthChoice{Methods: methods},
	}
	actions := Actions{
		ChooseAuth:   func(index int) { picked = append(picked, index) },
		CancelChoice: func() { cancelled++ },
	}

	tester := ui.NewTester(View(&state, actions), 520, 560)
	tester.Frame()

	if !tester.HasText("选择认证方式") || !tester.HasText("网关提供 2 种认证方式") {
		t.Fatalf("the chooser heading is missing: %v", tester.Texts())
	}
	for _, method := range methods {
		if !tester.HasText(MethodLabel(method)) {
			t.Fatalf("method %q is missing", MethodLabel(method))
		}
	}

	if err := tester.Click(MethodLabel(methods[1])); err != nil {
		t.Fatalf("click the second method: %v", err)
	}
	if len(picked) != 1 || picked[0] != 1 {
		t.Fatalf("picked = %v", picked)
	}
	if err := tester.Click("取消登录"); err != nil {
		t.Fatalf("click cancel: %v", err)
	}
	if cancelled != 1 {
		t.Fatalf("cancel calls = %d", cancelled)
	}
}
