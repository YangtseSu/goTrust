// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"os"
	"strings"
)

// texts are the words of the pages. Package core cannot reach MyGo's catalogs,
// so the pages carry their own two languages: the tag names the language, and
// every visible word comes from here.
type texts struct {
	HTML string

	SuccessTitle string
	SuccessBody  string

	CodeTitle    string
	CodeLabel    string
	CodeSkip     string
	CodeSkipHint string

	Submit string

	TextCaptchaTitle  string
	TextCaptchaPrompt string
	CaptchaAlt        string
	CaptchaCharsLabel string

	ClickPrompt string
	Undo        string
	ClearAll    string
	// Selected is a printf format: the click page writes the count into it.
	Selected string

	ExternalTitle   string
	ExternalMessage string
	LoginPage       string
	ExternalHint    string
	CallbackLabel   string

	PasswordTitle     string
	AccountLabel      string
	RulesTitle        string
	NewUsername       string
	OldPassword       string
	NewPassword       string
	ConfirmPassword   string
	ChangeButton      string
	SkipButton        string
	ParseFailure      string
	CodeEmpty         string
	CaptchaCharsEmpty string
	ClickUnreadable   string
	ClickEmpty        string
	ClickInvalid      string
	CallbackEmpty     string
	OldPasswordEmpty  string
	NewPasswordEmpty  string
	PasswordMismatch  string
	NewUsernameEmpty  string

	BindTitle     string
	BindMessage   string
	DeviceIDLabel string
	AllowBind     string
	DenyBind      string

	MethodTitle   string
	MethodMessage string
	DecisionEmpty string
	MethodEmpty   string

	CodeSMS     string
	CodeTOTP    string
	CodeRadius  string
	CodeGeneric string
	CodeMessage string

	RuleLength       string
	RuleLetters      string
	RuleUpperLower   string
	RuleNumber       string
	RuleSpecial      string
	RuleNoUsername   string
	RuleNoRepeat     string
	RuleNoSequence   string
	RuleNotWeak      string
	RuleNotInHistory string
}

var chinese = texts{
	HTML: "zh-CN",

	SuccessTitle: "已提交",
	SuccessBody:  "可以关闭此页面，登录会继续。",

	CodeTitle:    "输入验证码",
	CodeLabel:    "验证码",
	CodeSkip:     "跳过二次认证",
	CodeSkipHint: "勾选“跳过二次认证”后可以不填验证码，是否放行由网关决定。",

	Submit: "提交",

	TextCaptchaTitle:  "图形验证码",
	TextCaptchaPrompt: "请输入图片中的字符。",
	CaptchaAlt:        "验证码",
	CaptchaCharsLabel: "图中字符",

	ClickPrompt: "请按图片要求的顺序点击图中的字符。",
	Undo:        "撤销",
	ClearAll:    "全部清除",
	Selected:    "已选 %d 个点",

	ExternalTitle:   "浏览器登录",
	ExternalMessage: "请在浏览器中完成登录。",
	LoginPage:       "登录页面：",
	ExternalHint:    "完成登录后浏览器会停在一个打不开的地址；把地址栏里的完整链接粘贴到下面。",
	CallbackLabel:   "回调地址",

	PasswordTitle:     "修改密码",
	AccountLabel:      "账号：",
	RulesTitle:        "密码要求",
	NewUsername:       "新账号名",
	OldPassword:       "旧密码",
	NewPassword:       "新密码",
	ConfirmPassword:   "确认新密码",
	ChangeButton:      "修改密码",
	SkipButton:        "跳过，稍后再改",
	ParseFailure:      "表单无法解析，请重试。",
	CodeEmpty:         "请输入验证码",
	CaptchaCharsEmpty: "请输入图中的字符",
	ClickUnreadable:   "无法读取点击位置，请重新点击",
	ClickEmpty:        "请先在图片上点击字符",
	ClickInvalid:      "点击位置无效，请重新点击",
	CallbackEmpty:     "请粘贴回调地址",
	OldPasswordEmpty:  "请输入旧密码",
	NewPasswordEmpty:  "请输入新密码",
	PasswordMismatch:  "两次输入的新密码不一致",
	NewUsernameEmpty:  "请输入新账号名",

	BindTitle:     "绑定此设备",
	BindMessage:   "网关要求把这台设备绑定到当前账号。绑定后它出现在终端管理里。",
	DeviceIDLabel: "设备 ID：",
	AllowBind:     "允许绑定",
	DenyBind:      "拒绝",

	MethodTitle:   "选择认证方式",
	MethodMessage: "网关提供多种认证方式，请选择一种。",
	DecisionEmpty: "请选择允许或拒绝",
	MethodEmpty:   "请选择一种认证方式",

	CodeSMS:     "短信验证码",
	CodeTOTP:    "动态口令",
	CodeRadius:  "令牌口令",
	CodeGeneric: "验证码",
	CodeMessage: "网关要求输入一次性的%s。",

	RuleLength:       "密码长度至少 %d 位",
	RuleLetters:      "密码需包含任意字母",
	RuleUpperLower:   "密码需同时包含大小写字母",
	RuleNumber:       "密码需包含数字",
	RuleSpecial:      "密码需包含特殊字符",
	RuleNoUsername:   "密码不能包含用户名",
	RuleNoRepeat:     "密码不能包含 %d 位连续重复字符",
	RuleNoSequence:   "密码不能包含键盘连续排序字符或常规连续字符",
	RuleNotWeak:      "密码不能属于常见弱密码",
	RuleNotInHistory: "新密码不能与历史前 %d 次密码重复",
}

var english = texts{
	HTML: "en",

	SuccessTitle: "Submitted",
	SuccessBody:  "You can close this page; the login continues.",

	CodeTitle:    "Enter the code",
	CodeLabel:    "Code",
	CodeSkip:     "Skip this step",
	CodeSkipHint: "With \"skip this step\" ticked the code may stay empty; the gateway decides whether to accept it.",

	Submit: "Submit",

	TextCaptchaTitle:  "Image captcha",
	TextCaptchaPrompt: "Enter the characters in the image.",
	CaptchaAlt:        "Captcha",
	CaptchaCharsLabel: "Characters",

	ClickPrompt: "Click the characters in the order the image asks.",
	Undo:        "Undo",
	ClearAll:    "Clear all",
	Selected:    "%d points selected",

	ExternalTitle:   "Browser sign-in",
	ExternalMessage: "Finish the login in the browser.",
	LoginPage:       "Login page: ",
	ExternalHint:    "After the login the browser stops on an address that does not load; paste the full link from the address bar below.",
	CallbackLabel:   "Callback address",

	PasswordTitle:     "Change password",
	AccountLabel:      "Account: ",
	RulesTitle:        "Password requirements",
	NewUsername:       "New username",
	OldPassword:       "Current password",
	NewPassword:       "New password",
	ConfirmPassword:   "Confirm new password",
	ChangeButton:      "Change password",
	SkipButton:        "Skip for now",
	ParseFailure:      "The form could not be read; try again.",
	CodeEmpty:         "Enter the code",
	CaptchaCharsEmpty: "Enter the characters in the image",
	ClickUnreadable:   "The click positions could not be read; click again",
	ClickEmpty:        "Click the characters in the image first",
	ClickInvalid:      "A click position is invalid; click again",
	CallbackEmpty:     "Paste the callback address",
	OldPasswordEmpty:  "Enter the current password",
	NewPasswordEmpty:  "Enter the new password",
	PasswordMismatch:  "The new passwords do not match",
	NewUsernameEmpty:  "Enter the new username",

	BindTitle:     "Bind this device",
	BindMessage:   "The gateway asks to bind this device to your account. It then appears in Terminal management.",
	DeviceIDLabel: "Device ID: ",
	AllowBind:     "Allow",
	DenyBind:      "Deny",

	MethodTitle:   "Choose a sign-in method",
	MethodMessage: "The gateway offers several methods; choose one.",
	DecisionEmpty: "Choose allow or deny",
	MethodEmpty:   "Choose a sign-in method",

	CodeSMS:     "SMS code",
	CodeTOTP:    "authenticator code",
	CodeRadius:  "token code",
	CodeGeneric: "code",
	CodeMessage: "The gateway asks for a one-time %s.",

	RuleLength:       "At least %d characters",
	RuleLetters:      "Any letter",
	RuleUpperLower:   "Upper- and lower-case letters",
	RuleNumber:       "A digit",
	RuleSpecial:      "A special character",
	RuleNoUsername:   "Not the username",
	RuleNoRepeat:     "No %d characters repeated in a row",
	RuleNoSequence:   "No keyboard or character runs",
	RuleNotWeak:      "Not a common weak password",
	RuleNotInHistory: "Not one of the last %d passwords",
}

// textsFor returns the words of one language, both of which read their own
// numbers.
func textsFor(language string) texts {
	if language == "en" {
		return english
	}
	return chinese
}

// resolveLanguage reads the language of a tag, or of the POSIX locale
// environment when the tag is empty. The pages speak Chinese and English; any
// other language gets the source language.
func resolveLanguage(tag string) string {
	if strings.TrimSpace(tag) == "" {
		for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
			if value := os.Getenv(name); strings.TrimSpace(value) != "" {
				tag = value
				break
			}
		}
	}
	return languageBase(tag)
}

// languageBase reduces a BCP 47 tag or a POSIX locale to the language the
// pages speak.
func languageBase(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if cut := strings.IndexAny(tag, "_.@-"); cut >= 0 {
		tag = tag[:cut]
	}
	switch tag {
	case "zh":
		return "zh"
	case "en":
		return "en"
	case "c", "posix":
		return "en"
	default:
		return "zh"
	}
}
