// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/egoist/mygo/ui"
	"github.com/mythologyli/zju-connect/client/authchallenge"

	gotrustui "github.com/YangtseSu/goTrust/ui"
)

// captchaResult is what the window sends back to the login goroutine that is
// waiting inside authchallenge.Handler.
type captchaResult struct {
	response authchallenge.ClickCaptchaResponse
	err      error
}

// uiPrompter renders the click captcha in the main window and waits for the
// user. The context is the login attempt's: cancelling the login closes the
// prompt instead of leaving a dialog nobody will answer.
type uiPrompter struct {
	cliPrompter
	app *app
	ctx context.Context
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

	// A login started from the tray would otherwise wait on a hidden window.
	if window := a.window.Load(); window != nil {
		window.Show()
		window.Focus()
	}
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

// cliPrompter answers the login prompts a window would show. It is used by
// --no-ui, where refusing with a clear message beats blocking on a window that
// will never open.
type cliPrompter struct {
	logger *log.Logger
}

func (p cliPrompter) HandleCodeChallenge(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
	return authchallenge.CodeResponse{}, fmt.Errorf("this login needs a one-time code (%s), which goTrust does not ask for yet", challenge.Message)
}

func (p cliPrompter) HandleTextCaptcha(authchallenge.TextCaptchaChallenge) (authchallenge.TextCaptchaResponse, error) {
	return authchallenge.TextCaptchaResponse{}, errors.New("this gateway asked for a text captcha, which goTrust does not support")
}

func (p cliPrompter) HandleClickCaptcha(authchallenge.ClickCaptchaChallenge) (authchallenge.ClickCaptchaResponse, error) {
	return authchallenge.ClickCaptchaResponse{}, errors.New("this gateway asked for a click captcha, which needs the window; leave --no-ui off and log in again")
}

func (p cliPrompter) HandleExternalLogin(challenge authchallenge.ExternalLoginChallenge) (authchallenge.ExternalLoginResponse, error) {
	return authchallenge.ExternalLoginResponse{}, fmt.Errorf("this login needs a browser at %s, which goTrust does not open yet", challenge.LoginURL)
}
