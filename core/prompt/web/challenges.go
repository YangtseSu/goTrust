// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/YangtseSu/goTrust/core/atrust/client/authchallenge"
	"github.com/YangtseSu/goTrust/core/session"
)

// newCodePage serves a one-time code: SMS, TOTP or RADIUS.
func newCodePage(challenge authchallenge.CodeChallenge, token string, t texts) *page[authchallenge.CodeResponse] {
	message := strings.TrimSpace(challenge.Message)
	if message == "" {
		message = fmt.Sprintf(t.CodeMessage, codeLabel(challenge.Kind, t))
	}
	p := newPage[authchallenge.CodeResponse](token)
	addForm(p, t, func(failure string) []byte {
		return renderPage("code", codeData{
			pageData: pageData{T: t, Title: t.CodeTitle, Failure: failure},
			Message:  message,
			CanSkip:  challenge.CanSkipSecondaryAuth,
		})
	}, func(r *http.Request) (authchallenge.CodeResponse, error) {
		code := strings.TrimSpace(r.FormValue("code"))
		skip := challenge.CanSkipSecondaryAuth && r.FormValue("skip") != ""
		if code == "" && !skip {
			return authchallenge.CodeResponse{}, errors.New(t.CodeEmpty)
		}
		return authchallenge.CodeResponse{Code: code, SkipSecondaryAuth: skip}, nil
	})
	return p
}

// newTextCaptchaPage serves an image and one text field.
func newTextCaptchaPage(challenge authchallenge.TextCaptchaChallenge, token string, t texts) *page[authchallenge.TextCaptchaResponse] {
	p := newPage[authchallenge.TextCaptchaResponse](token)
	addForm(p, t, func(failure string) []byte {
		return renderPage("text-captcha", captchaData{
			pageData: pageData{T: t, Title: t.TextCaptchaTitle, Failure: failure},
		})
	}, func(r *http.Request) (authchallenge.TextCaptchaResponse, error) {
		code := strings.TrimSpace(r.FormValue("code"))
		if code == "" {
			return authchallenge.TextCaptchaResponse{}, errors.New(t.CaptchaCharsEmpty)
		}
		return authchallenge.TextCaptchaResponse{Code: code}, nil
	})
	addImage(p, challenge.Image)
	return p
}

// newClickCaptchaPage serves the image the person clicks. The page reports the
// clicks in the image's own pixels, which is what the portal expects; the
// displayed size only matters for where the marker is drawn.
func newClickCaptchaPage(challenge authchallenge.ClickCaptchaChallenge, token string, t texts) *page[authchallenge.ClickCaptchaResponse] {
	p := newPage[authchallenge.ClickCaptchaResponse](token)
	addForm(p, t, func(failure string) []byte {
		return renderPage("click-captcha", captchaData{
			pageData: pageData{T: t, Title: t.TextCaptchaTitle, Failure: failure},
		})
	}, func(r *http.Request) (authchallenge.ClickCaptchaResponse, error) {
		return parseClickCaptcha(r, t)
	})
	addImage(p, challenge.Image)
	return p
}

func parseClickCaptcha(r *http.Request, t texts) (authchallenge.ClickCaptchaResponse, error) {
	var points [][2]int
	if err := json.Unmarshal([]byte(r.FormValue("points")), &points); err != nil {
		return authchallenge.ClickCaptchaResponse{}, errors.New(t.ClickUnreadable)
	}
	if len(points) == 0 {
		return authchallenge.ClickCaptchaResponse{}, errors.New(t.ClickEmpty)
	}
	response := authchallenge.ClickCaptchaResponse{
		Width:  formInt(r, "width"),
		Height: formInt(r, "height"),
		Points: make([]authchallenge.Point, 0, len(points)),
	}
	for _, pair := range points {
		if pair[0] < 0 || pair[1] < 0 {
			return authchallenge.ClickCaptchaResponse{}, errors.New(t.ClickInvalid)
		}
		response.Points = append(response.Points, authchallenge.Point{X: pair[0], Y: pair[1]})
	}
	// Zero dimensions are not an error: the protocol layer decodes them from
	// the image when they are missing.
	return response, nil
}

// newExternalPage serves the SSO link and the paste box for the callback.
func newExternalPage(challenge authchallenge.ExternalLoginChallenge, token string, t texts) *page[authchallenge.ExternalLoginResponse] {
	message := strings.TrimSpace(challenge.Message)
	if message == "" {
		message = t.ExternalMessage
	}
	p := newPage[authchallenge.ExternalLoginResponse](token)
	addForm(p, t, func(failure string) []byte {
		return renderPage("external", externalData{
			pageData: pageData{T: t, Title: t.ExternalTitle, Failure: failure},
			Message:  message,
			LoginURL: challenge.LoginURL,
		})
	}, func(r *http.Request) (authchallenge.ExternalLoginResponse, error) {
		callback := strings.TrimSpace(r.FormValue("callback"))
		if callback == "" {
			return authchallenge.ExternalLoginResponse{}, errors.New(t.CallbackEmpty)
		}
		if err := session.ValidateCallbackURL(challenge.Kind, callback); err != nil {
			return authchallenge.ExternalLoginResponse{}, err
		}
		return authchallenge.ExternalLoginResponse{CallbackURL: callback}, nil
	})
	return p
}

// newPasswordPage serves the gateway's forced password change. The policy is
// rendered as hints only: the portal re-checks every rule when the change is
// submitted, including the ones the client cannot evaluate.
func newPasswordPage(challenge session.PasswordChangeChallenge, token string, t texts) *page[session.PasswordChangeAnswer] {
	p := newPage[session.PasswordChangeAnswer](token)
	addForm(p, t, func(failure string) []byte {
		return renderPage("password", passwordData{
			pageData:    pageData{T: t, Title: t.PasswordTitle, Failure: failure},
			Message:     strings.TrimSpace(challenge.Message),
			Username:    challenge.Username,
			Domain:      challenge.Domain,
			Prompt:      strings.TrimSpace(challenge.Prompt),
			ShowAccount: challenge.ShowAccount,
			CanSkip:     challenge.CanSkip,
			Rules:       passwordRules(challenge, t),
		})
	}, func(r *http.Request) (session.PasswordChangeAnswer, error) {
		answer := session.PasswordChangeAnswer{
			OldPassword: r.FormValue("old"),
			NewPassword: r.FormValue("new"),
			NewUserName: strings.TrimSpace(r.FormValue("username")),
			Skip:        challenge.CanSkip && r.FormValue("skip") != "",
		}
		if answer.Skip {
			return answer, nil
		}
		if strings.TrimSpace(answer.OldPassword) == "" {
			return session.PasswordChangeAnswer{}, errors.New(t.OldPasswordEmpty)
		}
		if strings.TrimSpace(answer.NewPassword) == "" {
			return session.PasswordChangeAnswer{}, errors.New(t.NewPasswordEmpty)
		}
		if answer.NewPassword != r.FormValue("confirm") {
			return session.PasswordChangeAnswer{}, errors.New(t.PasswordMismatch)
		}
		if challenge.ShowAccount && answer.NewUserName == "" {
			return session.PasswordChangeAnswer{}, errors.New(t.NewUsernameEmpty)
		}
		return answer, nil
	})
	return p
}

// newBindPage asks whether the portal may bind this installation.
func newBindPage(challenge session.DeviceBindChallenge, token string, t texts) *page[bool] {
	p := newPage[bool](token)
	addForm(p, t, func(failure string) []byte {
		return renderPage("bind", bindData{
			pageData: pageData{T: t, Title: t.BindTitle, Failure: failure},
			DeviceID: challenge.DeviceID,
		})
	}, func(r *http.Request) (bool, error) {
		switch r.FormValue("decision") {
		case "allow":
			return true, nil
		case "deny":
			return false, nil
		default:
			return false, errors.New(t.DecisionEmpty)
		}
	})
	return p
}

// newMethodPage lists the methods the gateway offers.
func newMethodPage(methods []session.AuthMethod, token string, t texts) *page[session.AuthMethod] {
	p := newPage[session.AuthMethod](token)
	addForm(p, t, func(failure string) []byte {
		return renderPage("method", methodData{
			pageData: pageData{T: t, Title: t.MethodTitle, Failure: failure},
			Methods:  methods,
		})
	}, func(r *http.Request) (session.AuthMethod, error) {
		index, err := strconv.Atoi(r.FormValue("method"))
		if err != nil || index < 0 || index >= len(methods) {
			return session.AuthMethod{}, errors.New(t.MethodEmpty)
		}
		return methods[index], nil
	})
	return p
}

// codeLabel names the second factor a code challenge asks for.
func codeLabel(kind authchallenge.CodeKind, t texts) string {
	switch kind {
	case authchallenge.CodeSMS:
		return t.CodeSMS
	case authchallenge.CodeTOTP:
		return t.CodeTOTP
	case authchallenge.CodeRadius:
		return t.CodeRadius
	default:
		return t.CodeGeneric
	}
}

// passwordRules lists the policy lines the gateway enabled.
func passwordRules(challenge session.PasswordChangeChallenge, t texts) []string {
	var labels []string
	for _, rule := range challenge.Strategy.Rules(challenge.Username) {
		if !rule.Enabled {
			continue
		}
		labels = append(labels, passwordRuleLabel(rule, t))
	}
	return labels
}

func passwordRuleLabel(rule session.PasswordRule, t texts) string {
	switch rule.Code {
	case session.RuleLength:
		return fmt.Sprintf(t.RuleLength, rule.Value)
	case session.RuleLetters:
		return t.RuleLetters
	case session.RuleUpperAndLower:
		return t.RuleUpperLower
	case session.RuleNumber:
		return t.RuleNumber
	case session.RuleSpecial:
		return t.RuleSpecial
	case session.RuleNoUsername:
		return t.RuleNoUsername
	case session.RuleNoRepeat:
		return fmt.Sprintf(t.RuleNoRepeat, rule.Value)
	case session.RuleNoSequence:
		return t.RuleNoSequence
	case session.RuleNotWeak:
		return t.RuleNotWeak
	case session.RuleNotInHistory:
		return fmt.Sprintf(t.RuleNotInHistory, rule.Value)
	default:
		return string(rule.Code)
	}
}

// formInt reads a non-negative integer field, zero when it is missing or not a
// number.
func formInt(r *http.Request, name string) int {
	value, err := strconv.Atoi(strings.TrimSpace(r.FormValue(name)))
	if err != nil || value < 0 {
		return 0
	}
	return value
}
