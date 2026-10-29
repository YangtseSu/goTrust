// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

// texts are the words of the pages. Package core cannot reach MyGo's catalogs,
// so the pages carry their own English words; HTML names the language.
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
	BindMessage:   "The gateway asks to bind this device to your account. It then appears in Terminals.",
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
