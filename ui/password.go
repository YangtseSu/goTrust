// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
)

// PasswordPrompt is a password change: the gateway's forced reset during login,
// or the voluntary reset the account page offers while online. Both use the
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
		return errors.New(locales.ErrOldPassword())
	}
	if strings.TrimSpace(p.NewPassword) == "" {
		return errors.New(locales.ErrNewPassword())
	}
	if p.NewPassword != p.Confirm {
		return errors.New(locales.ErrPasswordMismatch())
	}
	if p.NewPassword == p.OldPassword {
		return errors.New(locales.ErrPasswordSame())
	}
	if p.ShowAccount && strings.TrimSpace(p.NewUserName) == "" {
		return errors.New(locales.ErrNewUsername())
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
		return locales.ViolationLength()
	case strings.Contains(message, "both an upper-case"):
		return locales.ViolationCase()
	case strings.Contains(message, "contain a letter"):
		return locales.ViolationLetter()
	case strings.Contains(message, "contain a digit"):
		return locales.ViolationDigit()
	case strings.Contains(message, "special character"):
		return locales.ViolationSpecial()
	case strings.Contains(message, "account name"):
		return locales.ViolationUsername()
	case strings.Contains(message, "repeat"):
		return locales.ViolationRepeat()
	default:
		return message
	}
}

// RuleLabel is the panel's wording for one policy line.
func RuleLabel(rule session.PasswordRule) string {
	switch rule.Code {
	case session.RuleLength:
		return locales.RuleLength(rule.Value)
	case session.RuleLetters:
		return locales.RuleLetters()
	case session.RuleUpperAndLower:
		return locales.RuleUpperLower()
	case session.RuleNumber:
		return locales.RuleNumber()
	case session.RuleSpecial:
		return locales.RuleSpecial()
	case session.RuleNoUsername:
		return locales.RuleNoUsername()
	case session.RuleNoRepeat:
		return locales.RuleNoRepeat(rule.Value)
	case session.RuleNoSequence:
		return locales.RuleNoSequence()
	case session.RuleNotWeak:
		return locales.RuleNotWeak()
	case session.RuleNotInHistory:
		return locales.RuleHistory(rule.Value)
	default:
		return string(rule.Code)
	}
}

// passwordFields renders the inputs both password panels share.
func passwordFields(c *ui.Context, prompt *PasswordPrompt) {
	ui.Column(c).Gap(rowGap).Children(func() {
		if prompt.ShowAccount {
			ui.Field(c, locales.FieldNewUsername(), func() {
				ui.TextInput(c, &prompt.NewUserName).Placeholder(locales.FieldNewUsername())
			})
		}
		ui.Field(c, locales.FieldOldPassword(), func() {
			ui.TextInput(c, &prompt.OldPassword).Password().Placeholder(locales.PlaceholderOldPassword())
		})
		ui.Field(c, locales.FieldNewPassword(), func() {
			ui.TextInput(c, &prompt.NewPassword).Password().Placeholder(locales.PlaceholderNewPassword())
		})
		ui.Field(c, locales.FieldConfirmPassword(), func() {
			ui.TextInput(c, &prompt.Confirm).Password().Placeholder(locales.PlaceholderNewPassword())
		})
		if prompt.CanSkip {
			ui.Checkbox(c, &prompt.Skip, locales.PasswordSkipChange())
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
		caption(c, locales.PasswordRules())
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			line := ui.Text(c, ruleLabelWithMark(rule)).FontSize(textSize(c, captionScale)).
				TextColor(c.Theme().TextMuted)
			if rule.Satisfied {
				line.TextColor(toneInk(c.Theme(), toneSuccess))
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
	ui.Column(c).Grow(1).Shrink(1).Gap(sectionGap).Children(func() {
		ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
			ui.Column(c).Gap(sectionGap).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					panelHeading(c, locales.PasswordTitle())
					caption(c, locales.PasswordForcedHint())
					if prompt.Message != "" {
						captionTone(c, prompt.Message, toneWarning)
					}
					if prompt.Username != "" {
						caption(c, locales.PasswordAccount(prompt.Username))
					}
					if prompt.CustomizePwd && prompt.Domain != "" {
						caption(c, locales.PasswordDomain(prompt.Domain))
					}
				})
				passwordFields(c, prompt)
				passwordRules(c, prompt)
			})
		})
		panelActions(c, actions.CancelPassword, func() {
			label := locales.PasswordResetLogin()
			if prompt.Skip {
				label = locales.PasswordSkipLogin()
			}
			if ui.PrimaryButton(c, label).Disabled(!prompt.CanSubmit() && !prompt.Skip).Clicked() &&
				actions.SubmitPassword != nil {
				actions.SubmitPassword()
			}
		})
	})
}

// settingsPasswordPanel is the voluntary reset the account page shows in
// place of the sections, so the fields and the rules have room.
func settingsPasswordPanel(c *ui.Context, state *SettingsState, actions SettingsActions) {
	prompt := state.Password

	ui.Column(c).Gap(sectionGap).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			panelHeading(c, locales.PasswordTitle())
			caption(c, locales.PasswordVoluntaryHint())
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
			if ui.PrimaryButton(c, locales.PasswordResetOK()).Disabled(state.PasswordBusy || !prompt.CanSubmit()).Clicked() &&
				actions.SubmitPassword != nil {
				actions.SubmitPassword()
			}
			if ui.Button(c, locales.CommonCancel()).Disabled(state.PasswordBusy).Clicked() && actions.CancelPassword != nil {
				actions.CancelPassword()
			}
		})
	})
}
