// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mythologyli/zju-connect/client/authchallenge"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

func promptTestApp() *app {
	testApp := &app{logger: log.New(io.Discard, "", 0)}
	testApp.logf = testApp.logger.Printf
	return testApp
}

func promptTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		for y := range height {
			img.Set(x, y, color.RGBA{R: 30, G: 60, B: 90, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("encode the test image: %v", err)
	}
	return encoded.Bytes()
}

func waitForPrompt(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("the captcha prompt did not appear")
		}
		time.Sleep(time.Millisecond)
	}
}

// Submitting the window's points must hand the login goroutine the image size
// and the clicks in image pixels: the gateway's captcha payload needs both.
func TestClickCaptchaSubmitsPoints(t *testing.T) {
	testApp := promptTestApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prompter := &uiPrompter{app: testApp, ctx: ctx}
	results := make(chan captchaResult, 1)
	go func() {
		response, err := prompter.HandleClickCaptcha(authchallenge.ClickCaptchaChallenge{
			Image: promptTestPNG(t, 80, 40),
		})
		results <- captchaResult{response: response, err: err}
	}()

	waitForPrompt(t, func() bool {
		testApp.mu.RLock()
		defer testApp.mu.RUnlock()
		return testApp.view.Captcha != nil
	})

	if width, height := testApp.view.Captcha.Width, testApp.view.Captcha.Height; width != 80 || height != 40 {
		t.Fatalf("prompt image = %dx%d, want 80x40", width, height)
	}

	testApp.addCaptchaPoint(gotrustui.Point{X: 1, Y: 2})
	testApp.addCaptchaPoint(gotrustui.Point{X: 70, Y: 30})
	testApp.submitCaptcha()

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("captcha handler: %v", result.err)
		}
		if result.response.Width != 80 || result.response.Height != 40 {
			t.Fatalf("response size = %dx%d, want 80x40", result.response.Width, result.response.Height)
		}
		if len(result.response.Points) != 2 {
			t.Fatalf("response has %d points, want 2", len(result.response.Points))
		}
		if result.response.Points[0] != (authchallenge.Point{X: 1, Y: 2}) ||
			result.response.Points[1] != (authchallenge.Point{X: 70, Y: 30}) {
			t.Fatalf("response points = %+v", result.response.Points)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the captcha handler did not return after submit")
	}

	testApp.mu.RLock()
	pending := testApp.view.Captcha
	testApp.mu.RUnlock()
	if pending != nil {
		t.Fatal("the captcha panel stayed up after submit")
	}
}

// Cancelling the window or the login must release the waiting handler,
// otherwise a closed panel blocks the login goroutine forever.
func TestClickCaptchaCancelAndContext(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		cancel func(testApp *app, cancelLogin context.CancelFunc) error
	}{
		{
			name: "window",
			cancel: func(testApp *app, _ context.CancelFunc) error {
				testApp.cancelCaptcha(errors.New("captcha cancelled"))
				return errors.New("captcha cancelled")
			},
		},
		{
			name: "login context",
			cancel: func(_ *app, cancelLogin context.CancelFunc) error {
				cancelLogin()
				return context.Canceled
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			testApp := promptTestApp()
			ctx, cancelLogin := context.WithCancel(context.Background())
			defer cancelLogin()

			prompter := &uiPrompter{app: testApp, ctx: ctx}
			results := make(chan captchaResult, 1)
			go func() {
				response, err := prompter.HandleClickCaptcha(authchallenge.ClickCaptchaChallenge{
					Image: promptTestPNG(t, 60, 30),
				})
				results <- captchaResult{response: response, err: err}
			}()

			waitForPrompt(t, func() bool {
				testApp.mu.RLock()
				defer testApp.mu.RUnlock()
				return testApp.view.Captcha != nil
			})

			want := testCase.cancel(testApp, cancelLogin)
			select {
			case result := <-results:
				if result.err == nil {
					t.Fatal("the handler returned without an error")
				}
				if !errors.Is(result.err, want) && result.err.Error() != want.Error() {
					t.Fatalf("handler error = %v, want %v", result.err, want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("the captcha handler ignored the cancellation")
			}

			waitForPrompt(t, func() bool {
				testApp.mu.RLock()
				defer testApp.mu.RUnlock()
				return testApp.view.Captcha == nil
			})
		})
	}
}

// An image the decoder cannot read must fail the login with a reason rather
// than open an empty panel.
func TestClickCaptchaRejectsUndecodableImage(t *testing.T) {
	testApp := promptTestApp()
	prompter := &uiPrompter{app: testApp, ctx: context.Background()}

	if _, err := prompter.HandleClickCaptcha(authchallenge.ClickCaptchaChallenge{}); err == nil {
		t.Fatal("an empty image was accepted")
	}
	if _, err := prompter.HandleClickCaptcha(authchallenge.ClickCaptchaChallenge{Image: []byte("not an image")}); err == nil {
		t.Fatal("an undecodable image was accepted")
	}

	testApp.mu.RLock()
	defer testApp.mu.RUnlock()
	if testApp.view.Captcha != nil {
		t.Fatal("a failed decode left the panel up")
	}
}

// The code dialog has to hand the gateway the typed code and the skip choice,
// and clear itself so the next frame shows the controls again.
func TestCodeChallengeSubmits(t *testing.T) {
	testApp := promptTestApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prompter := &uiPrompter{app: testApp, ctx: ctx}
	results := make(chan codeResult, 1)
	go func() {
		response, err := prompter.HandleCodeChallenge(authchallenge.CodeChallenge{
			Kind:                 authchallenge.CodeSMS,
			Message:              "Please enter the SMS verification code:",
			CanSkipSecondaryAuth: true,
		})
		results <- codeResult{response: response, err: err}
	}()

	waitForPrompt(t, func() bool {
		testApp.mu.RLock()
		defer testApp.mu.RUnlock()
		return testApp.view.Code != nil
	})

	testApp.mu.Lock()
	prompt := testApp.view.Code
	if !prompt.CanSkip || prompt.Kind != authchallenge.CodeSMS {
		testApp.mu.Unlock()
		t.Fatalf("prompt = %+v", prompt)
	}
	prompt.Code = "654321"
	prompt.Skip = true
	testApp.mu.Unlock()

	testApp.submitCode()

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("code handler: %v", result.err)
		}
		if result.response.Code != "654321" || !result.response.SkipSecondaryAuth {
			t.Fatalf("response = %+v", result.response)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the code handler did not return after submit")
	}

	testApp.mu.RLock()
	pending := testApp.view.Code
	testApp.mu.RUnlock()
	if pending != nil {
		t.Fatal("the code panel stayed up after submit")
	}
}

// A TOTP secret the user opts into is validated, used for this attempt and
// written to the settings file; the next login generates its own codes.
func TestTOTPChallengeStoresTheSecret(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	testApp := promptTestApp()
	testApp.cfg = config.Default()
	testApp.configPath = filepath.Join(t.TempDir(), "config.toml")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prompter := &uiPrompter{app: testApp, ctx: ctx}
	results := make(chan codeResult, 1)
	go func() {
		response, err := prompter.HandleCodeChallenge(authchallenge.CodeChallenge{
			Kind:    authchallenge.CodeTOTP,
			Message: "Please enter the TOTP token:",
		})
		results <- codeResult{response: response, err: err}
	}()

	waitForPrompt(t, func() bool {
		testApp.mu.RLock()
		defer testApp.mu.RUnlock()
		return testApp.view.Code != nil
	})

	testApp.mu.Lock()
	prompt := testApp.view.Code
	if !prompt.AllowTOTPSecret {
		testApp.mu.Unlock()
		t.Fatal("a TOTP challenge did not allow storing the secret")
	}
	prompt.TOTPSecret = secret
	prompt.StoreSecret = true
	testApp.mu.Unlock()

	testApp.submitCode()

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("code handler: %v", result.err)
		}
		if len(result.response.Code) != 6 {
			t.Fatalf("generated code = %q, want six digits", result.response.Code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the code handler did not return after submit")
	}

	onDisk, err := config.LoadFile(testApp.configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := onDisk.Active().Auth.TOTPSecret; got != secret {
		t.Fatalf("stored secret = %q", got)
	}
	if got := testApp.currentConfig().Active().Auth.TOTPSecret; got != secret {
		t.Fatalf("live secret = %q", got)
	}

	// A malformed secret must keep the panel up instead of sending a code the
	// gateway will reject.
	testApp.mu.Lock()
	testApp.view.Code = &gotrustui.CodePrompt{Kind: authchallenge.CodeTOTP, AllowTOTPSecret: true, TOTPSecret: "not base32!", StoreSecret: true}
	testApp.code = results
	testApp.mu.Unlock()

	testApp.submitCode()
	testApp.mu.RLock()
	pending := testApp.view.Code
	testApp.mu.RUnlock()
	if pending == nil {
		t.Fatal("a malformed secret was submitted")
	}
	status := testApp.view.Status
	if !strings.Contains(status, "TOTP") {
		t.Fatalf("status = %q", status)
	}
	testApp.cancelCode(errors.New("done"))
}

// Cancelling the dialog or the login releases the blocked handler.
func TestCodeChallengeCancelAndContext(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		cancel func(testApp *app, cancelLogin context.CancelFunc) error
	}{
		{
			name: "window",
			cancel: func(testApp *app, _ context.CancelFunc) error {
				cause := errors.New("code cancelled")
				testApp.cancelCode(cause)
				return cause
			},
		},
		{
			name: "login context",
			cancel: func(_ *app, cancelLogin context.CancelFunc) error {
				cancelLogin()
				return context.Canceled
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			testApp := promptTestApp()
			ctx, cancelLogin := context.WithCancel(context.Background())
			defer cancelLogin()

			prompter := &uiPrompter{app: testApp, ctx: ctx}
			results := make(chan codeResult, 1)
			go func() {
				response, err := prompter.HandleCodeChallenge(authchallenge.CodeChallenge{
					Kind:    authchallenge.CodeRadius,
					Message: "Please enter the RADIUS token:",
				})
				results <- codeResult{response: response, err: err}
			}()

			waitForPrompt(t, func() bool {
				testApp.mu.RLock()
				defer testApp.mu.RUnlock()
				return testApp.view.Code != nil
			})

			want := testCase.cancel(testApp, cancelLogin)
			select {
			case result := <-results:
				if result.err == nil {
					t.Fatal("the handler returned without an error")
				}
				if !errors.Is(result.err, want) && result.err.Error() != want.Error() {
					t.Fatalf("handler error = %v, want %v", result.err, want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("the code handler ignored the cancellation")
			}

			waitForPrompt(t, func() bool {
				testApp.mu.RLock()
				defer testApp.mu.RUnlock()
				return testApp.view.Code == nil
			})
		})
	}
}

// The SSO dialog opens the login page, keeps a bad paste in the dialog and
// answers a valid callback.
func TestExternalLoginOpensAndValidatesTheCallback(t *testing.T) {
	opened := make(chan string, 2)
	testApp := promptTestApp()
	testApp.browser = func(url string) error {
		opened <- url
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prompter := &uiPrompter{app: testApp, ctx: ctx}
	results := make(chan externalResult, 1)
	go func() {
		response, err := prompter.HandleExternalLogin(authchallenge.ExternalLoginChallenge{
			Kind:     authchallenge.ExternalLoginCAS,
			LoginURL: "https://sso.example.com/cas/login",
			Message:  "Please enter the callback url:",
		})
		results <- externalResult{response: response, err: err}
	}()

	waitForPrompt(t, func() bool {
		testApp.mu.RLock()
		defer testApp.mu.RUnlock()
		return testApp.view.External != nil
	})
	select {
	case url := <-opened:
		if url != "https://sso.example.com/cas/login" {
			t.Fatalf("opened %q", url)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the login page was not opened")
	}

	testApp.mu.Lock()
	testApp.view.External.Callback = "https://vpn.example.com/passport/v1/auth/cas?sfDomain=local"
	testApp.mu.Unlock()
	testApp.submitExternal()

	select {
	case result := <-results:
		t.Fatalf("a callback without a ticket was accepted: %+v", result)
	default:
	}
	testApp.mu.RLock()
	if testApp.view.External == nil {
		testApp.mu.RUnlock()
		t.Fatal("the dialog closed on an invalid callback")
	}
	testApp.mu.RUnlock()

	testApp.mu.Lock()
	testApp.view.External.Callback = "https://vpn.example.com/passport/v1/auth/cas?sfDomain=local&ticket=ST-123"
	testApp.mu.Unlock()
	testApp.submitExternal()

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("external handler: %v", result.err)
		}
		if result.response.CallbackURL != "https://vpn.example.com/passport/v1/auth/cas?sfDomain=local&ticket=ST-123" {
			t.Fatalf("callback = %q", result.response.CallbackURL)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the external handler did not return after submit")
	}
}

// The chooser answers session.Options.ChooseAuth and pins the pick, so the
// next login does not ask again.
func TestChooseAuthPersistsThePick(t *testing.T) {
	testApp := promptTestApp()
	testApp.cfg = config.Default()
	testApp.configPath = filepath.Join(t.TempDir(), "config.toml")

	methods := []session.AuthMethod{
		{Name: "密码登录", Type: "auth/psw", Domain: "local"},
		{Name: "统一认证", Type: "auth/cas", Domain: "local"},
	}

	results := make(chan choiceResult, 1)
	go func() {
		method, err := testApp.chooseAuth(context.Background(), methods)
		results <- choiceResult{method: method, err: err}
	}()

	waitForPrompt(t, func() bool {
		testApp.mu.RLock()
		defer testApp.mu.RUnlock()
		return testApp.view.Choice != nil
	})

	testApp.mu.Lock()
	offered := len(testApp.view.Choice.Methods)
	testApp.mu.Unlock()
	if offered != 2 {
		t.Fatalf("offered methods = %d", offered)
	}

	testApp.chooseAuthIndex(1)

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("choose auth: %v", result.err)
		}
		if result.method != methods[1] {
			t.Fatalf("method = %+v", result.method)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the chooser did not return")
	}

	onDisk, err := config.LoadFile(testApp.configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	profile := onDisk.Active()
	if profile.Auth.AuthType != "auth/cas" || profile.Auth.Domain != "local" {
		t.Fatalf("stored pair = %s · %s", profile.Auth.AuthType, profile.Auth.Domain)
	}
}

// An attempt that ends while the chooser is open must release it.
func TestChooseAuthHonoursTheContext(t *testing.T) {
	testApp := promptTestApp()

	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan choiceResult, 1)
	go func() {
		method, err := testApp.chooseAuth(ctx, []session.AuthMethod{{Type: "auth/psw", Domain: "local"}})
		results <- choiceResult{method: method, err: err}
	}()

	waitForPrompt(t, func() bool {
		testApp.mu.RLock()
		defer testApp.mu.RUnlock()
		return testApp.view.Choice != nil
	})
	cancel()

	select {
	case result := <-results:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the chooser ignored the cancellation")
	}

	waitForPrompt(t, func() bool {
		testApp.mu.RLock()
		defer testApp.mu.RUnlock()
		return testApp.view.Choice == nil
	})
}
