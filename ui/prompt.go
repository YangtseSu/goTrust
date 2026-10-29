// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
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
		return locales.CodeTitleSMS()
	case session.CodeTOTP:
		return locales.CodeTitleTOTP()
	case session.CodeRadius:
		return locales.CodeTitleRadius()
	default:
		return locales.CodeTitleGeneric()
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
		return locales.ExternalTitleCAS()
	case session.ExternalOAuth2:
		return locales.ExternalTitleOAuth2()
	default:
		return locales.ExternalTitleGeneric()
	}
}

// AuthChoice asks which of the gateway's methods to log in with. It is only
// shown when several are offered and the profile stores none.
type AuthChoice struct {
	Methods []session.AuthMethod
}

// DeviceBindPrompt asks whether the portal may bind this installation to the
// account. Binding is a user decision; a gateway without the step never shows
// it.
type DeviceBindPrompt struct {
	DeviceID string
	AuthID   string
	Message  string
}

// MethodLabel is how a method is named in the chooser and on the account page.
func MethodLabel(method session.AuthMethod) string {
	return method.String()
}

// panelActions is the foot of a login panel: 取消登录 on the left, away from the
// answers, and the panel's own controls on the right, its primary action last.
// Abandoning the login is a different intent from answering it, so it does not
// sit beside 提交 where a mis-click ends the flow.
func panelActions(c *ui.Context, cancel func(), answers func()) {
	ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(rowGap).Children(func() {
		if textButton(c, locales.CancelLogin()).Disabled(cancel == nil).Clicked() && cancel != nil {
			cancel()
		}
		if answers == nil {
			return
		}
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(answers)
	})
}

// codePanel replaces the connection controls while the gateway waits for a
// one-time code.
func codePanel(c *ui.Context, state *State, actions Actions) {
	prompt := state.Code

	panelCard(c).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			panelHeading(c, CodeTitle(prompt.Kind))
			if prompt.Message != "" {
				caption(c, prompt.Message)
			}
		})

		formColumn(c, func() {
			formField(c, locales.CodeField(), func() {
				ui.TextInput(c, &prompt.Code).Placeholder(locales.CodePlaceholder()).
					OnSubmit(func() {
						if prompt.CanSubmit() && actions.SubmitCode != nil {
							actions.SubmitCode()
						}
					})
			})
			if prompt.CanSkip {
				ui.Checkbox(c, &prompt.Skip, locales.CodeSkip())
			}
			if prompt.AllowTOTPSecret {
				ui.Checkbox(c, &prompt.StoreSecret, locales.CodeStoreTOTP())
				if prompt.StoreSecret {
					formField(c, locales.CodeTOTPField(), func() {
						ui.TextInput(c, &prompt.TOTPSecret).Placeholder(locales.CodeTOTPPlaceholder())
					})
					caption(c, locales.CodeTOTPHint())
				}
			}
		})

		panelActions(c, actions.CancelCode, func() {
			if ui.PrimaryButton(c, locales.CommonSubmit()).Disabled(!prompt.CanSubmit()).Clicked() && actions.SubmitCode != nil {
				actions.SubmitCode()
			}
		})
	})
}

// externalPanel shows where to log in and takes the callback URL back.
func externalPanel(c *ui.Context, state *State, actions Actions) {
	prompt := state.External

	panelCard(c).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			panelHeading(c, ExternalTitle(prompt.Kind))
			if prompt.Message != "" {
				caption(c, prompt.Message)
			}
			caption(c, locales.ExternalHint())
		})

		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			ui.Text(c, prompt.LoginURL).FontSize(textSize(c, captionScale)).
				TextColor(c.Theme().TextMuted).Grow(1).MinWidth(0).MaxLines(2).
				Ellipsis("…").Label(locales.ExternalURLLabel(prompt.LoginURL))
			if ui.Button(c, locales.ExternalOpen()).Shrink(0).Description(locales.ExternalOpenDesc()).
				Clicked() && actions.OpenExternal != nil {
				actions.OpenExternal()
			}
		})

		formColumn(c, func() {
			formField(c, locales.CallbackField(), func() {
				ui.TextInput(c, &prompt.Callback).Placeholder("https://…/passport/v1/auth/…?…").
					OnSubmit(func() {
						if strings.TrimSpace(prompt.Callback) != "" && actions.SubmitExternal != nil {
							actions.SubmitExternal()
						}
					})
			})
		})

		panelActions(c, actions.CancelExternal, func() {
			if ui.PrimaryButton(c, locales.CommonSubmit()).Disabled(strings.TrimSpace(prompt.Callback) == "").Clicked() &&
				actions.SubmitExternal != nil {
				actions.SubmitExternal()
			}
		})
	})
}

// deviceBindPanel confirms a gateway device-binding request. Binding is what
// puts this installation in 终端管理 as a trusted device, so it is a decision,
// not a silent answer.
func deviceBindPanel(c *ui.Context, state *State, actions Actions) {
	prompt := state.DeviceBind

	panelCard(c).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			panelHeading(c, locales.BindTitle())
			caption(c, locales.BindHint())
			if prompt.Message != "" {
				caption(c, prompt.Message)
			}
		})

		if prompt.DeviceID != "" {
			formColumn(c, func() {
				formField(c, locales.DeviceID(), func() {
					ui.Text(c, prompt.DeviceID).FontSize(textSize(c, captionScale)).
						TextColor(c.Theme().TextMuted)
				})
			})
		}

		panelActions(c, actions.CancelDeviceBind, func() {
			if ui.PrimaryButton(c, locales.BindAllow()).Clicked() && actions.AllowDeviceBind != nil {
				actions.AllowDeviceBind()
			}
		})
	})
}

// choicePanel asks which offered method to use.
func choicePanel(c *ui.Context, state *State, actions Actions) {
	choice := state.Choice

	panelCard(c).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			panelHeading(c, locales.ChoiceTitle())
			caption(c, locales.ChoiceHint(len(choice.Methods)))
		})
		ui.Column(c).Gap(rowGap).Children(func() {
			for index, method := range choice.Methods {
				label := MethodLabel(method)
				if ui.Button(c, label).Clicked() && actions.ChooseAuth != nil {
					actions.ChooseAuth(index)
				}
			}
		})
		panelActions(c, actions.CancelChoice, nil)
	})
}
