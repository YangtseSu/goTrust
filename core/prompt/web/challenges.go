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
func newCodePage(challenge authchallenge.CodeChallenge, token string) *page[authchallenge.CodeResponse] {
	message := strings.TrimSpace(challenge.Message)
	if message == "" {
		message = "网关要求输入一次性的" + codeLabel(challenge.Kind) + "。"
	}
	p := newPage[authchallenge.CodeResponse](token)
	addForm(p, func(failure string) []byte {
		return renderPage("code", codeData{
			pageData: pageData{Title: "输入验证码", Failure: failure},
			Message:  message,
			CanSkip:  challenge.CanSkipSecondaryAuth,
		})
	}, func(r *http.Request) (authchallenge.CodeResponse, error) {
		code := strings.TrimSpace(r.FormValue("code"))
		skip := challenge.CanSkipSecondaryAuth && r.FormValue("skip") != ""
		if code == "" && !skip {
			return authchallenge.CodeResponse{}, errors.New("请输入验证码")
		}
		return authchallenge.CodeResponse{Code: code, SkipSecondaryAuth: skip}, nil
	})
	return p
}

// newTextCaptchaPage serves an image and one text field.
func newTextCaptchaPage(challenge authchallenge.TextCaptchaChallenge, token string) *page[authchallenge.TextCaptchaResponse] {
	p := newPage[authchallenge.TextCaptchaResponse](token)
	addForm(p, func(failure string) []byte {
		return renderPage("text-captcha", captchaData{
			pageData: pageData{Title: "图形验证码", Failure: failure},
		})
	}, func(r *http.Request) (authchallenge.TextCaptchaResponse, error) {
		code := strings.TrimSpace(r.FormValue("code"))
		if code == "" {
			return authchallenge.TextCaptchaResponse{}, errors.New("请输入图中的字符")
		}
		return authchallenge.TextCaptchaResponse{Code: code}, nil
	})
	addImage(p, challenge.Image)
	return p
}

// newClickCaptchaPage serves the image the person clicks. The page reports the
// clicks in the image's own pixels, which is what the portal expects; the
// displayed size only matters for where the marker is drawn.
func newClickCaptchaPage(challenge authchallenge.ClickCaptchaChallenge, token string) *page[authchallenge.ClickCaptchaResponse] {
	p := newPage[authchallenge.ClickCaptchaResponse](token)
	addForm(p, func(failure string) []byte {
		return renderPage("click-captcha", captchaData{
			pageData: pageData{Title: "图形验证码", Failure: failure},
		})
	}, parseClickCaptcha)
	addImage(p, challenge.Image)
	return p
}

func parseClickCaptcha(r *http.Request) (authchallenge.ClickCaptchaResponse, error) {
	var points [][2]int
	if err := json.Unmarshal([]byte(r.FormValue("points")), &points); err != nil {
		return authchallenge.ClickCaptchaResponse{}, errors.New("无法读取点击位置，请重新点击")
	}
	if len(points) == 0 {
		return authchallenge.ClickCaptchaResponse{}, errors.New("请先在图片上点击字符")
	}
	response := authchallenge.ClickCaptchaResponse{
		Width:  formInt(r, "width"),
		Height: formInt(r, "height"),
		Points: make([]authchallenge.Point, 0, len(points)),
	}
	for _, pair := range points {
		if pair[0] < 0 || pair[1] < 0 {
			return authchallenge.ClickCaptchaResponse{}, errors.New("点击位置无效，请重新点击")
		}
		response.Points = append(response.Points, authchallenge.Point{X: pair[0], Y: pair[1]})
	}
	// Zero dimensions are not an error: the protocol layer decodes them from
	// the image when they are missing.
	return response, nil
}

// newExternalPage serves the SSO link and the paste box for the callback.
func newExternalPage(challenge authchallenge.ExternalLoginChallenge, token string) *page[authchallenge.ExternalLoginResponse] {
	message := strings.TrimSpace(challenge.Message)
	if message == "" {
		message = "请在浏览器中完成登录。"
	}
	p := newPage[authchallenge.ExternalLoginResponse](token)
	addForm(p, func(failure string) []byte {
		return renderPage("external", externalData{
			pageData: pageData{Title: "浏览器登录", Failure: failure},
			Message:  message,
			LoginURL: challenge.LoginURL,
		})
	}, func(r *http.Request) (authchallenge.ExternalLoginResponse, error) {
		callback := strings.TrimSpace(r.FormValue("callback"))
		if callback == "" {
			return authchallenge.ExternalLoginResponse{}, errors.New("请粘贴回调地址")
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
func newPasswordPage(challenge session.PasswordChangeChallenge, token string) *page[session.PasswordChangeAnswer] {
	p := newPage[session.PasswordChangeAnswer](token)
	addForm(p, func(failure string) []byte {
		return renderPage("password", passwordData{
			pageData:    pageData{Title: "修改密码", Failure: failure},
			Message:     strings.TrimSpace(challenge.Message),
			Username:    challenge.Username,
			Domain:      challenge.Domain,
			Prompt:      strings.TrimSpace(challenge.Prompt),
			ShowAccount: challenge.ShowAccount,
			CanSkip:     challenge.CanSkip,
			Rules:       passwordRules(challenge),
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
			return session.PasswordChangeAnswer{}, errors.New("请输入旧密码")
		}
		if strings.TrimSpace(answer.NewPassword) == "" {
			return session.PasswordChangeAnswer{}, errors.New("请输入新密码")
		}
		if answer.NewPassword != r.FormValue("confirm") {
			return session.PasswordChangeAnswer{}, errors.New("两次输入的新密码不一致")
		}
		if challenge.ShowAccount && answer.NewUserName == "" {
			return session.PasswordChangeAnswer{}, errors.New("请输入新账号名")
		}
		return answer, nil
	})
	return p
}

// newBindPage asks whether the portal may bind this installation.
func newBindPage(challenge session.DeviceBindChallenge, token string) *page[bool] {
	p := newPage[bool](token)
	addForm(p, func(failure string) []byte {
		return renderPage("bind", bindData{
			pageData: pageData{Title: "绑定此设备", Failure: failure},
			DeviceID: challenge.DeviceID,
		})
	}, func(r *http.Request) (bool, error) {
		switch r.FormValue("decision") {
		case "allow":
			return true, nil
		case "deny":
			return false, nil
		default:
			return false, errors.New("请选择允许或拒绝")
		}
	})
	return p
}

// newMethodPage lists the methods the gateway offers.
func newMethodPage(methods []session.AuthMethod, token string) *page[session.AuthMethod] {
	p := newPage[session.AuthMethod](token)
	addForm(p, func(failure string) []byte {
		return renderPage("method", methodData{
			pageData: pageData{Title: "选择认证方式", Failure: failure},
			Methods:  methods,
		})
	}, func(r *http.Request) (session.AuthMethod, error) {
		index, err := strconv.Atoi(r.FormValue("method"))
		if err != nil || index < 0 || index >= len(methods) {
			return session.AuthMethod{}, errors.New("请选择一种认证方式")
		}
		return methods[index], nil
	})
	return p
}

// codeLabel names the second factor a code challenge asks for.
func codeLabel(kind authchallenge.CodeKind) string {
	switch kind {
	case authchallenge.CodeSMS:
		return "短信验证码"
	case authchallenge.CodeTOTP:
		return "动态口令"
	case authchallenge.CodeRadius:
		return "令牌口令"
	default:
		return "验证码"
	}
}

// passwordRules lists the policy lines the gateway enabled.
func passwordRules(challenge session.PasswordChangeChallenge) []string {
	var labels []string
	for _, rule := range challenge.Strategy.Rules(challenge.Username) {
		if !rule.Enabled {
			continue
		}
		labels = append(labels, passwordRuleLabel(rule))
	}
	return labels
}

func passwordRuleLabel(rule session.PasswordRule) string {
	switch rule.Code {
	case session.RuleLength:
		return fmt.Sprintf("密码长度至少 %d 位", rule.Value)
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
		return fmt.Sprintf("密码不能包含 %d 位连续重复字符", rule.Value)
	case session.RuleNoSequence:
		return "密码不能包含键盘连续排序字符或常规连续字符"
	case session.RuleNotWeak:
		return "密码不能属于常见弱密码"
	case session.RuleNotInHistory:
		return fmt.Sprintf("新密码不能与历史前 %d 次密码重复", rule.Value)
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
