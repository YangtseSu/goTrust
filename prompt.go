// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/mythologyli/zju-connect/client/authchallenge"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

// captchaResult is what the window sends back to the login goroutine that is
// waiting inside authchallenge.Handler.
type captchaResult struct {
	response authchallenge.ClickCaptchaResponse
	err      error
}

// codeResult answers a one-time code challenge.
type codeResult struct {
	response authchallenge.CodeResponse
	err      error
}

// externalResult answers a CAS/OAuth2 challenge with the pasted callback.
type externalResult struct {
	response authchallenge.ExternalLoginResponse
	err      error
}

// choiceResult answers the authentication-method chooser.
type choiceResult struct {
	method session.AuthMethod
	err    error
}

// passwordResult answers a forced password change.
type passwordResult struct {
	answer session.PasswordChangeAnswer
	err    error
}

// uiPrompter renders the click captcha in the main window and waits for the
// user. The context is the login attempt's: cancelling the login closes the
// prompt instead of leaving a dialog nobody will answer.
type uiPrompter struct {
	cliPrompter
	app *app
	ctx context.Context
}

func (p *uiPrompter) HandleCodeChallenge(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
	reply := make(chan codeResult, 1)
	p.app.beginCode(&gotrustui.CodePrompt{
		Kind:            challenge.Kind,
		Message:         challenge.Message,
		CanSkip:         challenge.CanSkipSecondaryAuth,
		AllowTOTPSecret: challenge.Kind == authchallenge.CodeTOTP,
	}, reply)
	defer p.app.endCode(reply)

	select {
	case result := <-reply:
		return result.response, result.err
	case <-p.ctx.Done():
		return authchallenge.CodeResponse{}, p.ctx.Err()
	}
}

func (p *uiPrompter) HandleExternalLogin(challenge authchallenge.ExternalLoginChallenge) (authchallenge.ExternalLoginResponse, error) {
	reply := make(chan externalResult, 1)
	p.app.beginExternal(&gotrustui.ExternalLogin{
		Kind:     challenge.Kind,
		LoginURL: challenge.LoginURL,
		Message:  challenge.Message,
	}, reply)
	defer p.app.endExternal(reply)

	// A browser that cannot be opened is not fatal: the window shows the URL
	// and a button that tries again. An empty URL means the gateway expects a
	// callback without a page, so there is nothing to open.
	if challenge.LoginURL != "" {
		if err := p.app.openBrowser(challenge.LoginURL); err != nil {
			p.app.logf("open the login URL: %v", err)
			p.app.setStatusError("无法自动打开浏览器，请手动复制链接: " + err.Error())
		}
	}

	select {
	case result := <-reply:
		return result.response, result.err
	case <-p.ctx.Done():
		return authchallenge.ExternalLoginResponse{}, p.ctx.Err()
	}
}

func (p *uiPrompter) HandleClickCaptcha(challenge authchallenge.ClickCaptchaChallenge) (authchallenge.ClickCaptchaResponse, error) {
	if len(challenge.Image) == 0 {
		return authchallenge.ClickCaptchaResponse{}, errors.New("the gateway sent an empty captcha image")
	}
	bitmap, err := ui.DecodeBitmap(challenge.Image)
	if err != nil {
		return authchallenge.ClickCaptchaResponse{}, fmt.Errorf("decode the captcha image: %w", err)
	}
	width, height := bitmap.Size()
	if width <= 0 || height <= 0 {
		return authchallenge.ClickCaptchaResponse{}, fmt.Errorf("the captcha image is %dx%d", width, height)
	}

	reply := make(chan captchaResult, 1)
	p.app.beginCaptcha(&gotrustui.Captcha{
		Image:  bitmap,
		Width:  width,
		Height: height,
	}, reply)
	defer p.app.endCaptcha(reply)

	select {
	case result := <-reply:
		return result.response, result.err
	case <-p.ctx.Done():
		return authchallenge.ClickCaptchaResponse{}, p.ctx.Err()
	}
}

// beginCaptcha publishes the pending challenge. The login goroutine then
// waits on reply for the window to submit or cancel it.
func (a *app) beginCaptcha(captcha *gotrustui.Captcha, reply chan captchaResult) {
	a.mu.Lock()
	a.view.Captcha = captcha
	a.captcha = reply
	a.mu.Unlock()

	a.setStatus("请在窗口中点击图形验证码", false)
	a.revealWindow()
	a.invalidate()
}

// endCaptcha clears the challenge the handler owned. It leaves a newer one
// alone, although the upstream client asks one at a time.
func (a *app) endCaptcha(reply chan captchaResult) {
	a.mu.Lock()
	if a.captcha == reply {
		a.captcha = nil
		a.view.Captcha = nil
	}
	a.mu.Unlock()

	a.invalidate()
}

func (a *app) addCaptchaPoint(point gotrustui.Point) {
	a.mu.Lock()
	if captcha := a.view.Captcha; captcha != nil {
		captcha.Points = append(captcha.Points, point)
	}
	a.mu.Unlock()

	a.invalidate()
}

func (a *app) undoCaptchaPoint() {
	a.mu.Lock()
	if captcha := a.view.Captcha; captcha != nil && len(captcha.Points) > 0 {
		captcha.Points = captcha.Points[:len(captcha.Points)-1]
	}
	a.mu.Unlock()

	a.invalidate()
}

func (a *app) clearCaptchaPoints() {
	a.mu.Lock()
	if captcha := a.view.Captcha; captcha != nil {
		captcha.Points = nil
	}
	a.mu.Unlock()

	a.invalidate()
}

func (a *app) submitCaptcha() {
	a.mu.Lock()
	captcha := a.view.Captcha
	reply := a.captcha
	if captcha == nil || reply == nil || len(captcha.Points) == 0 {
		a.mu.Unlock()
		if captcha != nil {
			a.setStatus("请先在验证码图片上点击字符", false)
		}
		return
	}

	response := authchallenge.ClickCaptchaResponse{
		Points: make([]authchallenge.Point, len(captcha.Points)),
		Width:  captcha.Width,
		Height: captcha.Height,
	}
	for i, point := range captcha.Points {
		response.Points[i] = authchallenge.Point{X: point.X, Y: point.Y}
	}
	a.view.Captcha = nil
	a.captcha = nil
	a.mu.Unlock()

	a.invalidate()
	a.setStatus("已提交验证码，正在登录…", true)
	reply <- captchaResult{response: response}
}

// cancelCaptcha makes the waiting login fail instead of leaving the window
// blocked on an answer that will not come.
func (a *app) cancelCaptcha(cause error) {
	a.mu.Lock()
	reply := a.captcha
	a.captcha = nil
	a.view.Captcha = nil
	a.mu.Unlock()

	if reply != nil {
		reply <- captchaResult{err: cause}
	}
	a.invalidate()
}

// beginCode publishes a one-time code challenge.
func (a *app) beginCode(prompt *gotrustui.CodePrompt, reply chan codeResult) {
	a.mu.Lock()
	a.view.Code = prompt
	a.code = reply
	a.mu.Unlock()

	a.setStatus("请在窗口中输入验证码", false)
	a.revealWindow()
	a.invalidate()
}

func (a *app) endCode(reply chan codeResult) {
	a.mu.Lock()
	if a.code == reply {
		a.code = nil
		a.view.Code = nil
	}
	a.mu.Unlock()

	a.invalidate()
}

// submitCode answers the pending code challenge. A TOTP secret the user chose
// to store is validated, used for this attempt if no code was typed, and
// written to the settings file.
func (a *app) submitCode() {
	a.mu.Lock()
	prompt := a.view.Code
	reply := a.code
	if prompt == nil || reply == nil {
		a.mu.Unlock()
		return
	}
	code := strings.TrimSpace(prompt.Code)
	secret := strings.TrimSpace(prompt.TOTPSecret)
	storeSecret := prompt.AllowTOTPSecret && prompt.StoreSecret && secret != ""
	skip := prompt.Skip && prompt.CanSkip
	a.mu.Unlock()

	if code == "" && !storeSecret {
		a.setStatus("请输入验证码", false)
		return
	}
	if storeSecret {
		generated, err := session.GenerateTOTPCode(secret, time.Now())
		if err != nil {
			a.setStatusError(err.Error())
			return
		}
		if code == "" {
			code = generated
		}
	}

	a.mu.Lock()
	if a.code == reply {
		a.code = nil
		a.view.Code = nil
	}
	a.mu.Unlock()

	if storeSecret {
		a.storeTOTPSecret(secret)
	}

	a.invalidate()
	a.setStatus("已提交验证码，正在登录…", true)
	reply <- codeResult{response: authchallenge.CodeResponse{Code: code, SkipSecondaryAuth: skip}}
}

func (a *app) cancelCode(cause error) {
	a.mu.Lock()
	reply := a.code
	a.code = nil
	a.view.Code = nil
	a.mu.Unlock()

	if reply != nil {
		reply <- codeResult{err: cause}
	}
	a.invalidate()
}

// storeChangedPassword rewrites a profile's credential after the gateway
// accepted a change. The next login needs it; a profile that does not store
// passwords keeps it in memory only, exactly like a password typed into the
// settings window.
func (a *app) storeChangedPassword(profileName, newPassword string) {
	cfg := a.mutableConfig()
	index := cfg.ProfileIndex(profileName)
	if index < 0 {
		a.logf("changed password for unknown profile %q; not stored", profileName)
		return
	}
	name := cfg.Profiles[index].Name
	cfg.Profiles[index].Auth.Password = newPassword
	if err := config.SaveFile(a.configPath, cfgForDisk(cfg)); err != nil {
		a.logf("save the changed password: %v", err)
		a.setStatusError("保存新密码失败: " + err.Error())
		return
	}
	a.applyConfig(cfg)

	a.mu.Lock()
	if state := a.settings; state != nil {
		for profile := range state.Draft.Profiles {
			if state.Draft.Profiles[profile].Name == name {
				state.Draft.Profiles[profile].Auth.Password = newPassword
			}
		}
	}
	a.mu.Unlock()
	a.invalidateSettings()
	a.logf("stored the changed password for profile %s", name)
}

// storeTOTPSecret writes the secret into the active profile. The settings file
// already holds the password with owner-only permissions, so the secret goes
// to the same place and only after the user ticked the box. A profile that does
// not store its password stores no secret either: the secret is cleared with
// the password toggle on every save.
func (a *app) storeTOTPSecret(secret string) {
	cfg := a.mutableConfig()
	index := cfg.ActiveIndex()
	if index < 0 {
		return
	}
	if !cfg.Profiles[index].Auth.StorePass {
		a.logf("TOTP secret not stored: the profile does not store its password")
		a.setStatus("未保存 TOTP 密钥：请先在设置中勾选保存密码", false)
		return
	}
	cfg.Profiles[index].Auth.TOTPSecret = strings.TrimSpace(secret)
	if err := config.SaveFile(a.configPath, cfgForDisk(cfg)); err != nil {
		a.logf("save the TOTP secret: %v", err)
		a.setStatusError("保存 TOTP 密钥失败: " + err.Error())
		return
	}
	a.applyConfig(cfg)
	a.logf("stored a TOTP secret for profile %s", cfg.ActiveProfile)
}

// beginExternal publishes an SSO challenge. The browser is opened by the
// handler, so the user can finish the login outside the app.
func (a *app) beginExternal(prompt *gotrustui.ExternalLogin, reply chan externalResult) {
	a.mu.Lock()
	a.view.External = prompt
	a.external = reply
	a.mu.Unlock()

	a.setStatus("请在浏览器中完成登录", false)
	a.revealWindow()
	a.invalidate()
}

func (a *app) endExternal(reply chan externalResult) {
	a.mu.Lock()
	if a.external == reply {
		a.external = nil
		a.view.External = nil
	}
	a.mu.Unlock()

	a.invalidate()
}

// openExternal reopens the login page, for a browser that was closed too early.
func (a *app) openExternal() {
	a.mu.RLock()
	prompt := a.view.External
	a.mu.RUnlock()
	if prompt == nil {
		return
	}
	if err := a.openBrowser(prompt.LoginURL); err != nil {
		a.setStatusError("打开浏览器失败: " + err.Error())
		a.logf("open the login URL: %v", err)
	}
}

func (a *app) submitExternal() {
	a.mu.Lock()
	prompt := a.view.External
	reply := a.external
	if prompt == nil || reply == nil {
		a.mu.Unlock()
		return
	}
	kind := prompt.Kind
	callback := strings.TrimSpace(prompt.Callback)
	a.mu.Unlock()

	if err := session.ValidateCallbackURL(kind, callback); err != nil {
		a.setStatusError(err.Error())
		return
	}

	a.mu.Lock()
	if a.external == reply {
		a.external = nil
		a.view.External = nil
	}
	a.mu.Unlock()

	a.invalidate()
	a.setStatus("已提交回调地址，正在登录…", true)
	reply <- externalResult{response: authchallenge.ExternalLoginResponse{CallbackURL: callback}}
}

func (a *app) cancelExternal(cause error) {
	a.mu.Lock()
	reply := a.external
	a.external = nil
	a.view.External = nil
	a.mu.Unlock()

	if reply != nil {
		reply <- externalResult{err: cause}
	}
	a.invalidate()
}

// chooseAuth is session.Options.ChooseAuth: it shows the offered methods and
// blocks the login goroutine until the window answers or the attempt ends.
func (a *app) chooseAuth(ctx context.Context, methods []session.AuthMethod) (session.AuthMethod, error) {
	reply := make(chan choiceResult, 1)
	a.beginChoice(&gotrustui.AuthChoice{Methods: methods}, reply)
	defer a.endChoice(reply)

	select {
	case result := <-reply:
		if result.err != nil {
			return session.AuthMethod{}, result.err
		}
		// Remember the answer: auto mode asks only once per gateway, and the
		// store method is what the settings window shows next time.
		a.storeAuthChoice(result.method)
		return result.method, nil
	case <-ctx.Done():
		return session.AuthMethod{}, ctx.Err()
	}
}

func (a *app) beginChoice(choice *gotrustui.AuthChoice, reply chan choiceResult) {
	a.mu.Lock()
	a.view.Choice = choice
	a.choice = reply
	a.mu.Unlock()

	a.setStatus("请选择认证方式", false)
	a.revealWindow()
	a.invalidate()
}

func (a *app) endChoice(reply chan choiceResult) {
	a.mu.Lock()
	if a.choice == reply {
		a.choice = nil
		a.view.Choice = nil
	}
	a.mu.Unlock()

	a.invalidate()
}

// chooseAuthIndex answers the chooser with the method at index.
func (a *app) chooseAuthIndex(index int) {
	a.mu.Lock()
	choice := a.view.Choice
	reply := a.choice
	if choice == nil || reply == nil || index < 0 || index >= len(choice.Methods) {
		a.mu.Unlock()
		return
	}
	method := choice.Methods[index]
	a.choice = nil
	a.view.Choice = nil
	a.mu.Unlock()

	a.invalidate()
	a.setStatus("认证方式："+gotrustui.MethodLabel(method), true)
	reply <- choiceResult{method: method}
}

// changePasswordPrompt is session.Options.ChangePassword: it shows the
// gateway's forced change and blocks the login goroutine until the window
// answers or the attempt ends.
func (a *app) changePasswordPrompt(ctx context.Context, challenge session.PasswordChangeChallenge) (session.PasswordChangeAnswer, error) {
	reply := make(chan passwordResult, 1)
	a.beginPassword(&gotrustui.PasswordPrompt{
		Message:      challenge.Message,
		Username:     challenge.Username,
		Domain:       challenge.Domain,
		Prompt:       challenge.Prompt,
		ShowAccount:  challenge.ShowAccount,
		CanSkip:      challenge.CanSkip,
		CustomizePwd: challenge.CustomizePwd,
		Strategy:     challenge.Strategy,
	}, reply)
	defer a.endPassword(reply)

	select {
	case result := <-reply:
		return result.answer, result.err
	case <-ctx.Done():
		return session.PasswordChangeAnswer{}, ctx.Err()
	}
}

func (a *app) beginPassword(prompt *gotrustui.PasswordPrompt, reply chan passwordResult) {
	a.mu.Lock()
	a.view.Password = prompt
	a.password = reply
	a.mu.Unlock()

	a.setStatus("网关要求重置密码", false)
	a.revealWindow()
	a.invalidate()
}

func (a *app) endPassword(reply chan passwordResult) {
	a.mu.Lock()
	if a.password == reply {
		a.password = nil
		a.view.Password = nil
	}
	a.mu.Unlock()

	a.invalidate()
}

// submitPassword answers the forced change. A skip is only sent when the
// gateway offered it; otherwise the fields are validated before they leave the
// window, so a typo does not cost a full login attempt.
func (a *app) submitPassword() {
	a.mu.Lock()
	prompt := a.view.Password
	reply := a.password
	if prompt == nil || reply == nil {
		a.mu.Unlock()
		return
	}
	answer := session.PasswordChangeAnswer{
		OldPassword: prompt.OldPassword,
		NewPassword: prompt.NewPassword,
		NewUserName: prompt.NewUserName,
		Skip:        prompt.Skip && prompt.CanSkip,
	}
	copied := *prompt
	a.mu.Unlock()

	if !answer.Skip {
		if err := copied.Validate(); err != nil {
			a.setStatusError(err.Error())
			return
		}
	}

	a.mu.Lock()
	if a.password == reply {
		a.password = nil
		a.view.Password = nil
	}
	a.mu.Unlock()

	a.invalidate()
	if answer.Skip {
		a.setStatus("已跳过密码重置，正在登录…", true)
		reply <- passwordResult{answer: answer}
		return
	}
	a.setStatus("正在重置密码，随后完成登录…", true)
	reply <- passwordResult{answer: answer}
}

func (a *app) cancelPassword(cause error) {
	a.mu.Lock()
	reply := a.password
	a.password = nil
	a.view.Password = nil
	a.mu.Unlock()

	if reply != nil {
		reply <- passwordResult{err: cause}
	}
	a.invalidate()
}

func (a *app) cancelChoice(cause error) {
	a.mu.Lock()
	reply := a.choice
	a.choice = nil
	a.view.Choice = nil
	a.mu.Unlock()

	if reply != nil {
		reply <- choiceResult{err: cause}
	}
	a.invalidate()
}

// storeAuthChoice pins the chosen pair in the active profile, so a later login
// does not ask again.
func (a *app) storeAuthChoice(method session.AuthMethod) {
	cfg := a.mutableConfig()
	index := cfg.ActiveIndex()
	if index < 0 {
		return
	}
	cfg.Profiles[index].Auth.AuthType = method.Type
	cfg.Profiles[index].Auth.Domain = method.Domain
	if err := config.SaveFile(a.configPath, cfgForDisk(cfg)); err != nil {
		a.logf("save the authentication method: %v", err)
		return
	}
	a.applyConfig(cfg)
	a.logf("stored authentication method %s · %s", method.Type, method.Domain)
}

// cancelPrompts releases every pending question, so a disconnect cannot leave
// a login goroutine waiting on a panel that is gone.
func (a *app) cancelPrompts(cause error) {
	a.cancelCaptcha(cause)
	a.cancelCode(cause)
	a.cancelExternal(cause)
	a.cancelChoice(cause)
	a.cancelPassword(cause)
}

// revealWindow brings the main window forward: a login started from the tray
// would otherwise wait on a hidden window.
func (a *app) revealWindow() {
	if window := a.window.Load(); window != nil {
		window.Show()
		window.Focus()
	}
}

// openBrowser opens a URL with the desktop's default browser.
func (a *app) openBrowser(url string) error {
	if a.browser != nil {
		return a.browser(url)
	}
	return mygo.Shell.OpenExternal(url)
}

// cliPrompter answers the login prompts a window would show. It is used by
// --no-ui, where refusing with a clear message beats blocking on a window that
// will never open.
type cliPrompter struct{}

func (p cliPrompter) HandleCodeChallenge(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
	return authchallenge.CodeResponse{}, fmt.Errorf(
		"this login needs a %s one-time code, which requires the window; run again without --no-ui",
		challenge.Kind)
}

func (p cliPrompter) HandleTextCaptcha(authchallenge.TextCaptchaChallenge) (authchallenge.TextCaptchaResponse, error) {
	return authchallenge.TextCaptchaResponse{}, errors.New("this gateway asked for a text captcha, which goTrust does not support")
}

func (p cliPrompter) HandleClickCaptcha(authchallenge.ClickCaptchaChallenge) (authchallenge.ClickCaptchaResponse, error) {
	return authchallenge.ClickCaptchaResponse{}, errors.New("this gateway asked for a click captcha, which needs the window; leave --no-ui off and log in again")
}

func (p cliPrompter) HandleExternalLogin(challenge authchallenge.ExternalLoginChallenge) (authchallenge.ExternalLoginResponse, error) {
	return authchallenge.ExternalLoginResponse{}, fmt.Errorf(
		"this login needs a browser at %s and a pasted callback URL, which requires the window; run again without --no-ui",
		challenge.LoginURL)
}
