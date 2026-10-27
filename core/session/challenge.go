// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/YangtseSu/goTrust/core/atrust/client/authchallenge"
	"github.com/pquerna/otp/totp"
)

// ExternalKind is the SSO flavour a callback URL belongs to. The alias lets the
// UI carry the kind without importing the upstream package.
type ExternalKind = authchallenge.ExternalLoginKind

// CodeKind is the second factor a code challenge asks for. The alias keeps the
// UI off the upstream package the same way.
type CodeKind = authchallenge.CodeKind

const (
	// ExternalCAS is a CAS login.
	ExternalCAS = authchallenge.ExternalLoginCAS
	// ExternalOAuth2 is an OAuth2 login.
	ExternalOAuth2 = authchallenge.ExternalLoginOAuth2
	// CodeSMS is an SMS one-time code.
	CodeSMS = authchallenge.CodeSMS
	// CodeTOTP is a TOTP token.
	CodeTOTP = authchallenge.CodeTOTP
	// CodeRadius is a RADIUS/token-server passcode.
	CodeRadius = authchallenge.CodeRadius
)

// The challenge and response values are aliases, not copies: Prompter is the
// protocol handler interface, so a front-end has to name these exact types.
// Declaring them here lets a front-end implement the interface by importing
// this package alone, without reaching into the protocol layer.
type (
	// CodeChallenge asks for a one-time code.
	CodeChallenge = authchallenge.CodeChallenge
	// CodeResponse answers one.
	CodeResponse = authchallenge.CodeResponse
	// TextCaptchaChallenge asks for the characters in an image.
	TextCaptchaChallenge = authchallenge.TextCaptchaChallenge
	// TextCaptchaResponse answers one.
	TextCaptchaResponse = authchallenge.TextCaptchaResponse
	// Point is one clicked coordinate in a click captcha.
	Point = authchallenge.Point
	// ClickCaptchaChallenge asks for the characters at their positions.
	ClickCaptchaChallenge = authchallenge.ClickCaptchaChallenge
	// ClickCaptchaResponse answers one, in the image's natural pixels.
	ClickCaptchaResponse = authchallenge.ClickCaptchaResponse
	// ExternalLoginChallenge asks for a login in the system browser.
	ExternalLoginChallenge = authchallenge.ExternalLoginChallenge
	// ExternalLoginResponse carries the pasted callback URL.
	ExternalLoginResponse = authchallenge.ExternalLoginResponse
)

// callbackPath is the portal endpoint a callback URL has to point at, and the
// query key it has to carry.
func callbackPath(kind ExternalKind) (path, key string, err error) {
	switch kind {
	case ExternalCAS:
		return "/passport/v1/auth/cas", "ticket", nil
	case ExternalOAuth2:
		return "/passport/v1/auth/httpsOauth2", "code", nil
	default:
		return "", "", fmt.Errorf("unsupported external login kind %q", kind)
	}
}

// ValidateCallbackURL checks the URL a user pasted after logging in with the
// system browser. The gateway only accepts its own callback path with the
// ticket (CAS) or code (OAuth2), so a wrong paste is caught in the dialog
// instead of as a failed login.
//
// The host is not checked: the client sends the callback to the gateway it is
// already talking to, and a gateway behind a proxy may hand out its public name.
func ValidateCallbackURL(kind ExternalKind, raw string) error {
	path, key, err := callbackPath(kind)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid callback URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return errors.New("invalid callback URL: the scheme is not https")
	}
	if parsed.Path != path {
		return fmt.Errorf("invalid callback URL: the path is not %s", path)
	}
	if parsed.Query().Get(key) == "" {
		return fmt.Errorf("invalid callback URL: %s is missing", key)
	}
	return nil
}

// GenerateTOTPCode derives the current one-time code from a TOTP secret, the
// same way the client does when a stored secret lets it skip the prompt.
func GenerateTOTPCode(secret string, at time.Time) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("the TOTP secret is empty")
	}
	code, err := totp.GenerateCode(secret, at)
	if err != nil {
		return "", fmt.Errorf("invalid TOTP secret: %w", err)
	}
	return code, nil
}
