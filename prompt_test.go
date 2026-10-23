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
	"testing"
	"time"

	"github.com/mythologyli/zju-connect/client/authchallenge"

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
