// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/session"
)

// PasswordPrompt is a password change: the gateway's forced reset during login,
// or the voluntary reset the settings window offers while online. Both use the
// same fields and rules; only the gateway's message and the skip offer differ.
type PasswordPrompt struct {
	// Forced changes carry the gateway's message, the account, the server-side
	// rule prompt and the two optional fields of the reset form.
	Message     string
	Username    string
	Domain      string
	Prompt      string
	ShowAccount bool
	CanSkip     bool
	// CustomizePwd is a non-local login domain: its policy lives on the server,
	// so the local rule list is hidden unless the gateway sent a prompt.
	CustomizePwd bool

	OldPassword string
	NewPassword string
	Confirm     string
	NewUserName string
	Skip        bool

	// Strategy is the policy the rules panel renders. An empty strategy shows
	// no rule list, which is what a gateway that sends no policy gets.
	Strategy session.PasswordStrategy
}

// Rules lists the policy lines for the panel.
func (p *PasswordPrompt) Rules() []session.PasswordRule {
	return p.Strategy.Rules(p.Username)
}

// CanSubmit reports whether the form is complete enough to send. Validation
// proper runs on submit so a mistake is explained instead of leaving the
// button greyed out with no reason.
func (p *PasswordPrompt) CanSubmit() bool {
	return strings.TrimSpace(p.OldPassword) != "" &&
		strings.TrimSpace(p.NewPassword) != "" &&
		p.Confirm != ""
}

// Validate explains what the user still has to fix. The portal re-checks every
// rule; this local pass only catches what the page can see, so a rule the
// client cannot evaluate (weak-password database, history, keyboard runs) is
// not refused here.
func (p *PasswordPrompt) Validate() error {
	if strings.TrimSpace(p.OldPassword) == "" {
		return errors.New("请输入旧密码")
	}
	if strings.TrimSpace(p.NewPassword) == "" {
		return errors.New("请输入新密码")
	}
	if p.NewPassword != p.Confirm {
		return errors.New("两次输入的新密码不一致")
	}
	if p.NewPassword == p.OldPassword {
		return errors.New("新密码不能与旧密码相同")
	}
	if p.ShowAccount && strings.TrimSpace(p.NewUserName) == "" {
		return errors.New("请输入新账号名")
	}
	if violations := p.Strategy.Violations(p.NewPassword, p.Username); len(violations) > 0 {
		return errors.New(ruleViolationLabel(violations[0]))
	}
	return nil
}

// ruleViolationLabel renders the first unmet rule in the panel's wording. The
// session layer's messages are English; the window shows these.
func ruleViolationLabel(message string) string {
	switch {
	case strings.Contains(message, "at least"):
		return "新密码长度不足"
	case strings.Contains(message, "both an upper-case"):
		return "新密码需同时包含大小写字母"
	case strings.Contains(message, "contain a letter"):
		return "新密码需包含字母"
	case strings.Contains(message, "contain a digit"):
		return "新密码需包含数字"
	case strings.Contains(message, "special character"):
		return "新密码需包含特殊字符"
	case strings.Contains(message, "account name"):
		return "新密码不能包含用户名"
	case strings.Contains(message, "repeat"):
		return "新密码包含连续重复字符"
	default:
		return message
	}
}

// RuleLabel is the panel's wording for one policy line.
func RuleLabel(rule session.PasswordRule) string {
	switch rule.Code {
	case session.RuleLength:
		return fmt.Sprintf("密码长度至少%d位", rule.Value)
	case session.RuleLetters:
		return "密码需包含任意字母"
	case session.RuleUpperAndLower:
		return "密码需同时包含大小写字母"
	case session.RuleNumber:
		return "密码需包含数字"
	case session.RuleSpecial:
		return "密码需包含特殊字符"
	case session.RuleNoUsername:
		return "密码不能包含用户名"
	case session.RuleNoRepeat:
		return fmt.Sprintf("密码不能包含%d位连续重复字符", rule.Value)
	case session.RuleNoSequence:
		return "密码不能包含键盘连续排序字符或常规连续字符"
	case session.RuleNotWeak:
		return "密码不能属于常见弱密码"
	case session.RuleNotInHistory:
		return fmt.Sprintf("新密码不能与历史前%d次密码重复", rule.Value)
	default:
		return string(rule.Code)
	}
}

// passwordFields renders the inputs both password panels share.
func passwordFields(c *ui.Context, prompt *PasswordPrompt) {
	ui.Column(c).Gap(rowGap).Children(func() {
		if prompt.ShowAccount {
			ui.Field(c, "新账号名", func() {
				ui.TextInput(c, &prompt.NewUserName).Placeholder("新账号名")
			})
		}
		ui.Field(c, "旧密码", func() {
			ui.TextInput(c, &prompt.OldPassword).Password().Placeholder("请输入旧密码")
		})
		ui.Field(c, "新密码", func() {
			ui.TextInput(c, &prompt.NewPassword).Password().Placeholder("请输入新密码")
		})
		ui.Field(c, "确认密码", func() {
			ui.TextInput(c, &prompt.Confirm).Password().Placeholder("请输入新密码")
		})
		if prompt.CanSkip {
			ui.Checkbox(c, &prompt.Skip, "跳过本次修改（网关允许）")
		}
	})
}

// passwordRules renders the policy lines, with a tick on the rules the client
// can check and a neutral mark for the ones only the portal answers.
func passwordRules(c *ui.Context, prompt *PasswordPrompt) {
	rules := prompt.Rules()
	if prompt.CustomizePwd {
		// The reset form shows only the gateway's own prompt for a
		// customized domain: the account may not follow the local policy.
		rules = nil
	}
	if len(rules) == 0 && prompt.Prompt == "" {
		return
	}

	ui.Column(c).Gap(4).Children(func() {
		caption(c, "密码规则")
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			line := ui.Text(c, ruleLabelWithMark(rule)).FontSize(textSize(c, captionScale)).
				TextColor(c.Theme().TextMuted)
			if rule.Satisfied {
				line.TextColor(toneColor(c.Theme(), toneSuccess))
			}
		}
		if prompt.Prompt != "" {
			caption(c, prompt.Prompt)
		}
	})
}

func ruleLabelWithMark(rule session.PasswordRule) string {
	return "● " + RuleLabel(rule)
}

// passwordPanel is the forced change inside the login flow. It replaces the
// connection controls the same way the captcha and code panels do.
func passwordPanel(c *ui.Context, state *State, actions Actions) {
	prompt := state.Password

	// The form is taller than the window's minimum, so it scrolls under a
	// pinned action row: the buttons stay reachable at 440x400 instead of
	// being clipped with the rules.
	ui.Column(c).Grow(1).Gap(sectionGap).Children(func() {
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(sectionGap).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					panelHeading(c, "重置密码")
					caption(c, "网关要求先重置密码才能完成登录。")
					if prompt.Message != "" {
						captionTone(c, prompt.Message, toneWarning)
					}
					if prompt.Username != "" {
						caption(c, "账号："+prompt.Username)
					}
					if prompt.CustomizePwd && prompt.Domain != "" {
						caption(c, "登录域："+prompt.Domain)
					}
				})
				passwordFields(c, prompt)
				passwordRules(c, prompt)
			})
		})
		ui.Row(c).AlignItems(ui.Center).Justify(ui.End).Gap(rowGap).Children(func() {
			label := "重置并登录"
			if prompt.Skip {
				label = "跳过并登录"
			}
			if ui.PrimaryButton(c, label).Disabled(!prompt.CanSubmit() && !prompt.Skip).Clicked() &&
				actions.SubmitPassword != nil {
				actions.SubmitPassword()
			}
			if ui.Button(c, "取消").Clicked() && actions.CancelPassword != nil {
				actions.CancelPassword()
			}
		})
	})
}

// settingsPasswordPanel is the voluntary reset the settings window shows in
// place of the sections, so the fields and the rules have room.
func settingsPasswordPanel(c *ui.Context, state *SettingsState, actions SettingsActions) {
	prompt := state.Password

	ui.Column(c).Gap(sectionGap).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			panelHeading(c, "重置密码")
			caption(c, "密码重置后，其他终端已在线账号需重新登录。")
		})
		passwordFields(c, prompt)
		passwordRules(c, prompt)
		if state.PasswordStatus != "" {
			if state.PasswordError {
				captionTone(c, state.PasswordStatus, toneDanger)
			} else {
				captionTone(c, state.PasswordStatus, toneSuccess)
			}
		}
		ui.Row(c).AlignItems(ui.Center).Justify(ui.End).Gap(rowGap).Children(func() {
			if ui.PrimaryButton(c, "确定").Disabled(state.PasswordBusy || !prompt.CanSubmit()).Clicked() &&
				actions.SubmitPassword != nil {
				actions.SubmitPassword()
			}
			if ui.Button(c, "取消").Disabled(state.PasswordBusy).Clicked() && actions.CancelPassword != nil {
				actions.CancelPassword()
			}
		})
	})
}
