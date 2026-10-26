// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package web serves the interactive steps of a login as pages on the loopback
// interface.
//
// A login can ask for a one-time code, a text or click captcha, an SSO callback
// URL, a new password, a device binding, or an authentication method. None of
// that fits a terminal or a process without a display: a captcha needs an image
// and coordinates, an SSO login a link and a pasted URL. The package serves one
// page per question on 127.0.0.1 under a fresh single-use token, prints the URL,
// and blocks the login goroutine until the page answers or the attempt's
// context ends.
//
// Nothing a user types is logged: only the page URL, which carries the token,
// leaves the package through Print.
package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/YangtseSu/goTrust/core/atrust/client/authchallenge"
	"github.com/YangtseSu/goTrust/core/session"
)

const (
	// DefaultTimeout bounds one unanswered page. A page belongs to a login
	// attempt, so it must not wait forever on a person who walked away.
	DefaultTimeout = 5 * time.Minute

	// maxFormBytes caps a submitted form; the largest is a click captcha's
	// coordinate list, far below this.
	maxFormBytes = 64 << 10
)

// Options configure the pages of one front-end.
type Options struct {
	// Print receives the URL of every page before it opens. A host without a
	// display reads the URL from here. Nil discards it.
	Print func(url string)
	// Open opens a URL in the system's browser. Nil only prints: that is what
	// a run with --print-url wants, because no browser can be started.
	Open func(ctx context.Context, url string) error
	// Logf receives diagnostics such as a browser that would not start. It
	// never receives an answer. Nil discards them.
	Logf func(format string, args ...any)
	// Timeout bounds one page. Zero uses DefaultTimeout.
	Timeout time.Duration
}

// Prompter serves the login's questions. One value serves every attempt of a
// front-end; Handler binds it to one attempt.
type Prompter struct {
	options Options
}

// New returns a prompter that serves the configured pages.
func New(options Options) *Prompter {
	if options.Timeout <= 0 {
		options.Timeout = DefaultTimeout
	}
	if options.Logf == nil {
		options.Logf = func(string, ...any) {}
	}
	return &Prompter{options: options}
}

// Handler returns the prompter for one login attempt. The login goroutine is
// the one that blocks, so the attempt's context is what ends a wait: a
// cancelled login closes its page instead of leaving a question nobody will
// answer.
func (p *Prompter) Handler(ctx context.Context) session.Prompter {
	return &attempt{prompter: p, ctx: ctx}
}

// attempt is the Handler for one login attempt.
type attempt struct {
	prompter *Prompter
	ctx      context.Context
}

// ChooseAuth is session.Options.ChooseAuth: it shows the offered methods and
// returns the click.
func (p *Prompter) ChooseAuth(ctx context.Context, methods []session.AuthMethod) (session.AuthMethod, error) {
	if len(methods) == 0 {
		return session.AuthMethod{}, errors.New("the gateway offered no authentication method")
	}
	return ask(ctx, p, "", func(token string) *page[session.AuthMethod] {
		return newMethodPage(methods, token)
	})
}

// ChangePassword is session.Options.ChangePassword: it serves the gateway's
// forced change. The page only hints at the policy; the portal re-checks every
// rule, and a rule the client cannot evaluate is not refused locally.
func (p *Prompter) ChangePassword(ctx context.Context, challenge session.PasswordChangeChallenge) (session.PasswordChangeAnswer, error) {
	return ask(ctx, p, "", func(token string) *page[session.PasswordChangeAnswer] {
		return newPasswordPage(challenge, token)
	})
}

// ConfirmDeviceBind is session.Options.ConfirmDeviceBind: binding is the
// person's decision, so the login waits for the page.
func (p *Prompter) ConfirmDeviceBind(ctx context.Context, challenge session.DeviceBindChallenge) (bool, error) {
	return ask(ctx, p, "", func(token string) *page[bool] {
		return newBindPage(challenge, token)
	})
}

func (a *attempt) HandleCodeChallenge(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
	return ask(a.ctx, a.prompter, "", func(token string) *page[authchallenge.CodeResponse] {
		return newCodePage(challenge, token)
	})
}

func (a *attempt) HandleTextCaptcha(challenge authchallenge.TextCaptchaChallenge) (authchallenge.TextCaptchaResponse, error) {
	if len(challenge.Image) == 0 {
		return authchallenge.TextCaptchaResponse{}, errors.New("the gateway sent an empty captcha image")
	}
	return ask(a.ctx, a.prompter, "", func(token string) *page[authchallenge.TextCaptchaResponse] {
		return newTextCaptchaPage(challenge, token)
	})
}

func (a *attempt) HandleClickCaptcha(challenge authchallenge.ClickCaptchaChallenge) (authchallenge.ClickCaptchaResponse, error) {
	if len(challenge.Image) == 0 {
		return authchallenge.ClickCaptchaResponse{}, errors.New("the gateway sent an empty captcha image")
	}
	return ask(a.ctx, a.prompter, "", func(token string) *page[authchallenge.ClickCaptchaResponse] {
		return newClickCaptchaPage(challenge, token)
	})
}

func (a *attempt) HandleExternalLogin(challenge authchallenge.ExternalLoginChallenge) (authchallenge.ExternalLoginResponse, error) {
	// The browser is sent to the login page, not to ours: the page's own URL
	// only carries the paste box. A gateway that hands out no login page still
	// gets its paste box opened.
	return ask(a.ctx, a.prompter, challenge.LoginURL, func(token string) *page[authchallenge.ExternalLoginResponse] {
		return newExternalPage(challenge, token)
	})
}

// ask serves one page and waits for its answer.
func ask[T any](ctx context.Context, p *Prompter, open string, build func(token string) *page[T]) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return zero, fmt.Errorf("start the prompt page: %w", err)
	}

	pager := build(newToken())
	server := &http.Server{
		Handler:           loopbackHostOnly(pager.mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer shutdown(server)

	link := "http://" + listener.Addr().String() + pager.path()
	if p.options.Print != nil {
		p.options.Print(link)
	}
	if open == "" {
		open = link
	}
	p.open(ctx, open)

	timer := time.NewTimer(p.options.Timeout)
	defer timer.Stop()

	select {
	case answer := <-pager.answers:
		return answer, nil
	case err := <-served:
		return zero, fmt.Errorf("serve the prompt page: %w", err)
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-timer.C:
		return zero, fmt.Errorf("the prompt at %s was not answered within %s", link, p.options.Timeout)
	}
}

// open starts the browser without holding the login goroutine: a launcher that
// waits for the browser must not delay the page the person is about to use.
func (p *Prompter) open(ctx context.Context, target string) {
	if p.options.Open == nil {
		return
	}
	go func() {
		if err := p.options.Open(ctx, target); err != nil {
			p.options.Logf("open a browser at %s: %v", target, err)
		}
	}()
}

// shutdown closes the listener and lets the answer's response reach the
// browser, so the person sees the confirmation instead of a dead connection.
func shutdown(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

// newToken returns the path segment that authorises one page. It is 128 bits of
// randomness, so another local process cannot reach the question, and it is
// spent when the page answers.
func newToken() string {
	var raw [16]byte
	// crypto/rand.Read fills the slice or panics; on the platforms goTrust
	// builds for it never fails. A guessable fallback would be worse than a
	// crash, because any local process could then answer the login.
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}

// loopbackHostOnly rejects a request that did not address the loopback
// interface by name. A hostile page cannot reach a prompt through DNS
// rebinding then. The port is ignored on purpose: an SSH forward that remaps
// the port still reaches the same loopback address.
func loopbackHostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, "not a loopback request", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loopbackHost(host string) bool {
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}
