// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/session"
)

// CodePrompt is a one-time code challenge. SMS and RADIUS have no other way to
// answer; TOTP can also take a secret so later logins skip the prompt.
type CodePrompt struct {
	Kind    session.CodeKind
	Message string
	Code    string
	// CanSkip is the gateway's "skip future secondary authentication" offer.
	// Only then does the check box appear, and only then is it sent.
	CanSkip bool
	Skip    bool

	// TOTP only: a secret the user may store to have codes generated for them.
	// It is written only when StoreSecret is ticked.
	AllowTOTPSecret bool
	TOTPSecret      string
	StoreSecret     bool
}

// CanSubmit reports whether the panel has enough to answer the challenge: a
// typed code, or a stored secret a code can be generated from.
func (p *CodePrompt) CanSubmit() bool {
	if strings.TrimSpace(p.Code) != "" {
		return true
	}
	return p.AllowTOTPSecret && p.StoreSecret && strings.TrimSpace(p.TOTPSecret) != ""
}

// CodeTitle is the heading for a challenge kind.
func CodeTitle(kind session.CodeKind) string {
	switch kind {
	case session.CodeSMS:
		return "短信验证码"
	case session.CodeTOTP:
		return "动态验证码"
	case session.CodeRadius:
		return "RADIUS 口令"
	default:
		return "二次验证"
	}
}

// ExternalLogin is a CAS/OAuth2 login that happens in the system browser. The
// gateway waits for the callback URL the browser ends on.
type ExternalLogin struct {
	Kind     session.ExternalKind
	LoginURL string
	Message  string
	Callback string
}

// ExternalTitle is the heading for an SSO flavour.
func ExternalTitle(kind session.ExternalKind) string {
	switch kind {
	case session.ExternalCAS:
		return "统一身份认证（CAS）"
	case session.ExternalOAuth2:
		return "外部登录（OAuth2）"
	default:
		return "外部登录"
	}
}

// AuthChoice asks which of the gateway's methods to log in with. It is only
// shown when several are offered and the profile stores none.
type AuthChoice struct {
	Methods []session.AuthMethod
}

// MethodLabel is how a method is named in the chooser and the settings window.
func MethodLabel(method session.AuthMethod) string {
	return method.String()
}

// codePanel replaces the connection controls while the gateway waits for a
// one-time code.
func codePanel(c *ui.Context, state *State, actions Actions) {
	prompt := state.Code

	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, CodeTitle(prompt.Kind)).FontSize(16).Bold()
		if prompt.Message != "" {
			ui.Text(c, prompt.Message).FontSize(12)
		}
		ui.Field(c, "验证码", func() {
			ui.TextInput(c, &prompt.Code).Placeholder("输入收到的验证码")
		})
		if prompt.CanSkip {
			ui.Checkbox(c, &prompt.Skip, "跳过以后的二次认证")
		}
		if prompt.AllowTOTPSecret {
			ui.Checkbox(c, &prompt.StoreSecret, "保存 TOTP 密钥，以后由 goTrust 自动生成验证码")
			if prompt.StoreSecret {
				ui.Field(c, "TOTP 密钥", func() {
					ui.TextInput(c, &prompt.TOTPSecret).Placeholder("Base32 密钥，例如 JBSWY3DPEHPK3PXP")
				})
				ui.Text(c, "密钥保存在配置文件中（0600），只在你勾选后写入。").FontSize(11)
			}
		}
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "提交").Disabled(!prompt.CanSubmit()).Clicked() && actions.SubmitCode != nil {
				actions.SubmitCode()
			}
			if ui.Button(c, "取消").Clicked() && actions.CancelCode != nil {
				actions.CancelCode()
			}
		})
	})
}

// externalPanel shows where to log in and takes the callback URL back.
func externalPanel(c *ui.Context, state *State, actions Actions) {
	prompt := state.External

	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, ExternalTitle(prompt.Kind)).FontSize(16).Bold()
		if prompt.Message != "" {
			ui.Text(c, prompt.Message).FontSize(12)
		}
		ui.Text(c, "在浏览器中完成登录后，把地址栏里的完整回调地址粘贴到下面。").FontSize(12)
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, prompt.LoginURL).FontSize(11)
			if ui.Button(c, "在浏览器中打开").Clicked() && actions.OpenExternal != nil {
				actions.OpenExternal()
			}
		})
		ui.Field(c, "回调地址", func() {
			ui.TextInput(c, &prompt.Callback).Placeholder("https://…/passport/v1/auth/…?…")
		})
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "提交").Disabled(strings.TrimSpace(prompt.Callback) == "").Clicked() &&
				actions.SubmitExternal != nil {
				actions.SubmitExternal()
			}
			if ui.Button(c, "取消").Clicked() && actions.CancelExternal != nil {
				actions.CancelExternal()
			}
		})
	})
}

// choicePanel asks which offered method to use.
func choicePanel(c *ui.Context, state *State, actions Actions) {
	choice := state.Choice

	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, "选择认证方式").FontSize(16).Bold()
		ui.Text(c, fmt.Sprintf("网关提供 %d 种认证方式，请选择一种登录。", len(choice.Methods))).FontSize(12)
		for index, method := range choice.Methods {
			label := MethodLabel(method)
			if ui.Button(c, label).Clicked() && actions.ChooseAuth != nil {
				actions.ChooseAuth(index)
			}
		}
		if ui.Button(c, "取消").Clicked() && actions.CancelChoice != nil {
			actions.CancelChoice()
		}
	})
}
