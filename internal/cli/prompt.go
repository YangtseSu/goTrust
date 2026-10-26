// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"

	"github.com/YangtseSu/goTrust/core/atrust/client/authchallenge"
	"github.com/YangtseSu/goTrust/core/session"
)

// refusingPrompter answers the login prompts a terminal cannot show. Refusing
// with the reason beats waiting for a dialog that will never open.
type refusingPrompter struct{}

var _ session.Prompter = refusingPrompter{}

func (refusingPrompter) HandleCodeChallenge(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
	return authchallenge.CodeResponse{}, fmt.Errorf(
		"this login needs a %s one-time code, which %s cannot ask for", challenge.Kind, program)
}

func (refusingPrompter) HandleTextCaptcha(authchallenge.TextCaptchaChallenge) (authchallenge.TextCaptchaResponse, error) {
	return authchallenge.TextCaptchaResponse{}, errors.New(
		"this gateway asked for a text captcha, which goTrust does not support")
}

func (refusingPrompter) HandleClickCaptcha(authchallenge.ClickCaptchaChallenge) (authchallenge.ClickCaptchaResponse, error) {
	return authchallenge.ClickCaptchaResponse{}, errors.New(
		"this login needs a click captcha, which " + program + " cannot show")
}

func (refusingPrompter) HandleExternalLogin(challenge authchallenge.ExternalLoginChallenge) (authchallenge.ExternalLoginResponse, error) {
	return authchallenge.ExternalLoginResponse{}, fmt.Errorf(
		"this login needs a browser at %s and a pasted callback URL, which %s cannot ask for",
		challenge.LoginURL, program)
}
