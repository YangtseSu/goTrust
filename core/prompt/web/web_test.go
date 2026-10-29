// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/atrust/client/authchallenge"
	"github.com/YangtseSu/goTrust/core/session"
)

const testToken = "0123456789abcdef0123456789abcdef"

func getPage(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func postForm(mux *http.ServeMux, path string, form url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func answered[T any](p *page[T]) (T, bool) {
	select {
	case answer := <-p.answers:
		return answer, true
	default:
		var zero T
		return zero, false
	}
}

func TestCodePage(t *testing.T) {
	challenge := authchallenge.CodeChallenge{
		Kind:                 authchallenge.CodeSMS,
		Message:              "The code was sent to 138****0000",
		CanSkipSecondaryAuth: true,
	}
	p := newCodePage(challenge, testToken, english)

	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{"The code was sent to 138****0000", `name="code"`, `name="skip"`, "Enter the code"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}
	if code := getPage(p.mux, "/wrong-token/").Code; code != http.StatusNotFound {
		t.Fatalf("wrong token status = %d, want %d", code, http.StatusNotFound)
	}

	recorder := postForm(p.mux, "/"+testToken+"/", url.Values{})
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "Enter the code") {
		t.Fatalf("empty form = %d %q", recorder.Code, recorder.Body.String())
	}
	if _, ok := answered(p); ok {
		t.Fatal("an empty form answered the page")
	}

	recorder = postForm(p.mux, "/"+testToken+"/", url.Values{"code": {" 123456 "}, "skip": {"1"}})
	if recorder.Code != http.StatusOK {
		t.Fatalf("submit = %d %q", recorder.Code, recorder.Body.String())
	}
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"code": {"000000"}}).Code; code != http.StatusConflict {
		t.Fatalf("second submit = %d, want %d", code, http.StatusConflict)
	}
	answer, ok := answered(p)
	if !ok {
		t.Fatal("the page was not answered")
	}
	if answer.Code != "123456" || !answer.SkipSecondaryAuth {
		t.Fatalf("answer = %+v", answer)
	}
}

func TestTextCaptchaPage(t *testing.T) {
	image := []byte("\x89PNG\r\n\x1a\n")
	p := newTextCaptchaPage(authchallenge.TextCaptchaChallenge{
		Image:   image,
		Message: "Enter the displayed characters",
	}, testToken, english)

	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{`src="image"`, "Enter the characters in the image.", `name="code"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}

	recorder := getPage(p.mux, "/"+testToken+"/image")
	if recorder.Code != http.StatusOK || recorder.Body.String() != string(image) {
		t.Fatalf("image = %d %q", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("image content type = %q", contentType)
	}

	if code := postForm(p.mux, "/"+testToken+"/", url.Values{}).Code; code != http.StatusBadRequest {
		t.Fatalf("empty captcha = %d", code)
	}
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"code": {" abcd "}}).Code; code != http.StatusOK {
		t.Fatalf("captcha = %d", code)
	}
	answer, ok := answered(p)
	if !ok || answer.Code != "abcd" {
		t.Fatalf("answer = %+v ok=%v", answer, ok)
	}
}

func TestClickCaptchaPage(t *testing.T) {
	image := []byte("\x89PNG\r\n\x1a\n")
	p := newClickCaptchaPage(authchallenge.ClickCaptchaChallenge{Image: image}, testToken, english)

	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{`id="image"`, `name="points"`, "0 points selected"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}
	if code := getPage(p.mux, "/"+testToken+"/image").Code; code != http.StatusOK {
		t.Fatalf("image status = %d", code)
	}

	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"points": {"[]"}}).Code; code != http.StatusBadRequest {
		t.Fatalf("empty points = %d", code)
	}
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"points": {"not json"}}).Code; code != http.StatusBadRequest {
		t.Fatalf("malformed points = %d", code)
	}
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{
		"points": {"[[10,20],[30,40]]"},
		"width":  {"300"},
		"height": {"150"},
	}).Code; code != http.StatusOK {
		t.Fatalf("submit = %d", code)
	}
	answer, ok := answered(p)
	if !ok || len(answer.Points) != 2 {
		t.Fatalf("answer = %+v ok=%v", answer, ok)
	}
	if answer.Points[0] != (authchallenge.Point{X: 10, Y: 20}) ||
		answer.Points[1] != (authchallenge.Point{X: 30, Y: 40}) {
		t.Fatalf("points = %+v", answer.Points)
	}
	if answer.Width != 300 || answer.Height != 150 {
		t.Fatalf("dimensions = %dx%d", answer.Width, answer.Height)
	}
}

func TestExternalPage(t *testing.T) {
	p := newExternalPage(authchallenge.ExternalLoginChallenge{
		Kind:     authchallenge.ExternalLoginCAS,
		LoginURL: "https://idp.example/login",
	}, testToken, english)

	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{`href="https://idp.example/login"`, `name="callback"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}

	recorder := postForm(p.mux, "/"+testToken+"/", url.Values{"callback": {"https://vpn.example/elsewhere?ticket=ST-1"}})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("wrong callback path = %d %q", recorder.Code, recorder.Body.String())
	}
	if _, ok := answered(p); ok {
		t.Fatal("an invalid callback answered the page")
	}

	callback := "https://vpn.example/passport/v1/auth/cas?ticket=ST-1"
	recorder = postForm(p.mux, "/"+testToken+"/", url.Values{"callback": {callback}})
	if recorder.Code != http.StatusOK {
		t.Fatalf("valid callback = %d %q", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "ST-1") {
		t.Fatalf("the answer page echoed the ticket: %q", recorder.Body.String())
	}
	answer, ok := answered(p)
	if !ok || answer.CallbackURL != callback {
		t.Fatalf("answer = %+v ok=%v", answer, ok)
	}
}

func TestExternalPageOAuth2(t *testing.T) {
	p := newExternalPage(authchallenge.ExternalLoginChallenge{
		Kind: authchallenge.ExternalLoginOAuth2,
	}, testToken, english)
	callback := "https://vpn.example/passport/v1/auth/httpsOauth2?code=abc"
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"callback": {callback}}).Code; code != http.StatusOK {
		t.Fatalf("callback = %d", code)
	}
	answer, ok := answered(p)
	if !ok || answer.CallbackURL != callback {
		t.Fatalf("answer = %+v ok=%v", answer, ok)
	}
}

func TestPasswordPage(t *testing.T) {
	challenge := session.PasswordChangeChallenge{
		Message:  "The initial password must be changed",
		Username: "alice",
		Domain:   "example",
		Prompt:   "Set a new password",
		CanSkip:  true,
	}
	p := newPasswordPage(challenge, testToken, english)

	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{"The initial password must be changed", "alice", "Set a new password", `name="old"`, `name="new"`, `name="confirm"`, "Skip for now"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}

	for name, form := range map[string]url.Values{
		"no old password": {"new": {"next"}, "confirm": {"next"}},
		"no new password": {"old": {"now"}},
		"mismatch":        {"old": {"now"}, "new": {"next"}, "confirm": {"other"}},
	} {
		if code := postForm(p.mux, "/"+testToken+"/", form).Code; code != http.StatusBadRequest {
			t.Fatalf("%s = %d", name, code)
		}
	}
	if _, ok := answered(p); ok {
		t.Fatal("an incomplete form answered the page")
	}

	if code := postForm(p.mux, "/"+testToken+"/", url.Values{
		"old":     {"now"},
		"new":     {"next"},
		"confirm": {"next"},
	}).Code; code != http.StatusOK {
		t.Fatalf("valid form = %d", code)
	}
	answer, ok := answered(p)
	if !ok {
		t.Fatal("the page was not answered")
	}
	if answer.OldPassword != "now" || answer.NewPassword != "next" || answer.Skip {
		t.Fatalf("answer = %+v", answer)
	}
}

func TestPasswordPageSkips(t *testing.T) {
	challenge := session.PasswordChangeChallenge{CanSkip: true}
	p := newPasswordPage(challenge, testToken, english)
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"skip": {"1"}}).Code; code != http.StatusOK {
		t.Fatalf("skip = %d", code)
	}
	answer, ok := answered(p)
	if !ok || !answer.Skip {
		t.Fatalf("answer = %+v ok=%v", answer, ok)
	}
}

func TestPasswordPageRequiresTheNewAccountName(t *testing.T) {
	challenge := session.PasswordChangeChallenge{ShowAccount: true, Username: "alice"}
	p := newPasswordPage(challenge, testToken, english)
	form := url.Values{"old": {"now"}, "new": {"next"}, "confirm": {"next"}}
	if code := postForm(p.mux, "/"+testToken+"/", form).Code; code != http.StatusBadRequest {
		t.Fatalf("missing account name = %d", code)
	}
	form.Set("username", "alice2")
	if code := postForm(p.mux, "/"+testToken+"/", form).Code; code != http.StatusOK {
		t.Fatalf("account name = %d", code)
	}
	answer, _ := answered(p)
	if answer.NewUserName != "alice2" {
		t.Fatalf("answer = %+v", answer)
	}
}

func TestBindPage(t *testing.T) {
	challenge := session.DeviceBindChallenge{DeviceID: "device-0001", AuthID: "auth-1"}
	p := newBindPage(challenge, testToken, english)

	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{"device-0001", `value="allow"`, `value="deny"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"decision": {"maybe"}}).Code; code != http.StatusBadRequest {
		t.Fatalf("bad decision = %d", code)
	}
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"decision": {"allow"}}).Code; code != http.StatusOK {
		t.Fatalf("allow = %d", code)
	}
	allowed, ok := answered(p)
	if !ok || !allowed {
		t.Fatalf("answer = %v ok=%v", allowed, ok)
	}

	deny := newBindPage(challenge, testToken, english)
	if code := postForm(deny.mux, "/"+testToken+"/", url.Values{"decision": {"deny"}}).Code; code != http.StatusOK {
		t.Fatalf("deny = %d", code)
	}
	if allowed, ok := answered(deny); !ok || allowed {
		t.Fatalf("answer = %v ok=%v", allowed, ok)
	}
}

func TestMethodPage(t *testing.T) {
	methods := []session.AuthMethod{
		{Name: "Password", Type: "auth/psw", Domain: "local"},
		{Name: "CAS", Type: "auth/cas", Domain: "example", URL: "https://idp.example/login"},
	}
	p := newMethodPage(methods, testToken, english)

	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{"Password", "auth/psw", "CAS", `value="1"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"method": {"9"}}).Code; code != http.StatusBadRequest {
		t.Fatalf("out of range = %d", code)
	}
	if code := postForm(p.mux, "/"+testToken+"/", url.Values{"method": {"1"}}).Code; code != http.StatusOK {
		t.Fatalf("choice = %d", code)
	}
	answer, ok := answered(p)
	if !ok || answer.Type != "auth/cas" {
		t.Fatalf("answer = %+v ok=%v", answer, ok)
	}
}

// askOn starts a prompt in the background and returns the page URL it printed
// together with the target the browser was sent to.
func askOn[T any](t *testing.T, p *Prompter, run func(context.Context) (T, error)) (string, string, <-chan struct {
	answer T
	err    error
}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	links := make(chan string, 1)
	targets := make(chan string, 1)
	p.options.Print = func(link string) { links <- link }
	p.options.Open = func(ctx context.Context, target string) error {
		targets <- target
		return nil
	}
	done := make(chan struct {
		answer T
		err    error
	}, 1)
	go func() {
		answer, err := run(ctx)
		done <- struct {
			answer T
			err    error
		}{answer, err}
	}()

	var link string
	select {
	case link = <-links:
	case <-time.After(5 * time.Second):
		t.Fatal("no page URL was printed")
	}
	var target string
	select {
	case target = <-targets:
	case <-time.After(5 * time.Second):
		t.Fatal("the browser was not opened")
	}
	return link, target, done
}

func TestAskServesThePageAndOpensIt(t *testing.T) {
	p := New(Options{})
	link, target, done := askOn(t, p, func(ctx context.Context) (authchallenge.CodeResponse, error) {
		return p.Handler(ctx).HandleCodeChallenge(authchallenge.CodeChallenge{Kind: authchallenge.CodeTOTP})
	})
	if target != link {
		t.Fatalf("browser target = %q, want the page %q", target, link)
	}
	if !strings.HasPrefix(link, "http://127.0.0.1:") {
		t.Fatalf("link = %q, want a loopback page", link)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse the link: %v", err)
	}
	token := strings.Trim(parsed.Path, "/")
	if len(token) != 32 {
		t.Fatalf("link path = %q, want a 32-character token", parsed.Path)
	}

	response, err := http.PostForm(link, url.Values{"code": {"162534"}})
	if err != nil {
		t.Fatalf("post the answer: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("submit status = %d", response.StatusCode)
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("handler: %v", result.err)
		}
		if result.answer.Code != "162534" {
			t.Fatalf("answer = %+v", result.answer)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the handler did not return after the answer")
	}
}

func TestAskOpensTheSSOLoginPage(t *testing.T) {
	p := New(Options{})
	link, target, done := askOn(t, p, func(ctx context.Context) (authchallenge.ExternalLoginResponse, error) {
		return p.Handler(ctx).HandleExternalLogin(authchallenge.ExternalLoginChallenge{
			Kind:     authchallenge.ExternalLoginCAS,
			LoginURL: "https://idp.example/login",
		})
	})
	if target != "https://idp.example/login" {
		t.Fatalf("browser target = %q, want the login page", target)
	}
	callback := "https://vpn.example/passport/v1/auth/cas?ticket=ST-9"
	response, err := http.PostForm(link, url.Values{"callback": {callback}})
	if err != nil {
		t.Fatalf("post the callback: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	select {
	case result := <-done:
		if result.err != nil || result.answer.CallbackURL != callback {
			t.Fatalf("answer = %+v err = %v", result.answer, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the handler did not return after the answer")
	}
}

func TestAskStopsWhenTheLoginIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	links := make(chan string, 1)
	p := New(Options{
		Print:   func(link string) { links <- link },
		Timeout: time.Minute,
	})
	done := make(chan error, 1)
	go func() {
		_, err := p.Handler(ctx).HandleClickCaptcha(authchallenge.ClickCaptchaChallenge{Image: []byte("\x89PNG\r\n\x1a\n")})
		done <- err
	}()
	select {
	case <-links:
	case <-time.After(5 * time.Second):
		t.Fatal("no page URL was printed")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled login kept waiting")
	}
}

func TestAskTimesOut(t *testing.T) {
	links := make(chan string, 1)
	p := New(Options{
		Print:   func(link string) { links <- link },
		Timeout: 50 * time.Millisecond,
	})
	done := make(chan error, 1)
	go func() {
		_, err := p.ChangePassword(context.Background(), session.PasswordChangeChallenge{})
		done <- err
	}()
	select {
	case <-links:
	case <-time.After(5 * time.Second):
		t.Fatal("no page URL was printed")
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not answered") {
			t.Fatalf("error = %v, want a timeout", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the page waited past its timeout")
	}
}

// Nothing a user types may reach the URL printer or the log: the URL is the
// only thing a front-end is allowed to repeat.
func TestAnswersAreNeverPrinted(t *testing.T) {
	var mu sync.Mutex
	var output []string
	links := make(chan string, 1)
	p := New(Options{
		Print: func(link string) {
			mu.Lock()
			output = append(output, link)
			mu.Unlock()
			links <- link
		},
		Logf: func(format string, args ...any) {
			mu.Lock()
			output = append(output, fmt.Sprintf(format, args...))
			mu.Unlock()
		},
		Open: func(context.Context, string) error { return errors.New("no browser") },
	})
	done := make(chan error, 1)
	go func() {
		_, err := p.Handler(context.Background()).HandleCodeChallenge(
			authchallenge.CodeChallenge{Kind: authchallenge.CodeSMS})
		done <- err
	}()

	var link string
	select {
	case link = <-links:
	case <-time.After(5 * time.Second):
		t.Fatal("no page URL was printed")
	}
	const code = "827364"
	response, err := http.PostForm(link, url.Values{"code": {code}})
	if err != nil {
		t.Fatalf("post the answer: %v", err)
	}
	_ = response.Body.Close()
	select {
	case result := <-done:
		if result != nil {
			t.Fatalf("handler: %v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the handler did not return")
	}

	time.Sleep(20 * time.Millisecond) // let the browser goroutine report
	mu.Lock()
	defer mu.Unlock()
	for _, line := range output {
		if strings.Contains(line, code) {
			t.Fatalf("output repeated the code: %q", line)
		}
	}
}

func TestLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:41234": true,
		"127.0.0.1":       true,
		"localhost:8080":  true,
		"[::1]:41234":     true,
		"evil.example":    false,
		"192.0.2.1:8080":  false,
		"localhost.evil":  false,
	}
	for host, want := range cases {
		if got := loopbackHost(host); got != want {
			t.Fatalf("loopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestLoopbackHostOnlyRejectsAnotherHost(t *testing.T) {
	handler := loopbackHostOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "evil.example"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestPasswordRuleLabels(t *testing.T) {
	rules := []session.PasswordRule{
		{Code: session.RuleLength, Enabled: true, Value: 8},
		{Code: session.RuleUpperAndLower, Enabled: true},
		{Code: session.RuleNoRepeat, Enabled: true, Value: 3},
		{Code: session.RuleNotInHistory, Enabled: true, Value: 5},
	}
	var labels []string
	for _, rule := range rules {
		labels = append(labels, passwordRuleLabel(rule, english))
	}
	want := []string{"At least 8 characters", "Upper- and lower-case letters", "No 3 characters repeated in a row", "Not one of the last 5 passwords"}
	for index, label := range labels {
		if label != want[index] {
			t.Fatalf("label[%d] = %q, want %q", index, label, want[index])
		}
	}
}

// The pages speak the language the prompter was built with, from the heading
// to the confirmation a submitted form shows.
func TestPagesSpeakEnglish(t *testing.T) {
	p := newCodePage(authchallenge.CodeChallenge{Kind: authchallenge.CodeSMS}, testToken, english)

	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{`lang="en"`, "Enter the code", "The gateway asks for a one-time SMS code.", ">Code\n", ">Submit</button>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}

	rejected := postForm(p.mux, "/"+testToken+"/", url.Values{})
	if rejected.Code != http.StatusBadRequest || !strings.Contains(rejected.Body.String(), "Enter the code") {
		t.Fatalf("empty form = %d %q", rejected.Code, rejected.Body.String())
	}

	accepted := postForm(p.mux, "/"+testToken+"/", url.Values{"code": {"123456"}})
	if !strings.Contains(accepted.Body.String(), "Submitted") {
		t.Fatalf("confirmation = %q, want the English success page", accepted.Body.String())
	}
}

// The pages speak Chinese only when asked; the default and any other value
// stay English, and the environment is never consulted.
func TestTextsFor(t *testing.T) {
	cases := map[string]string{
		"":      "Enter the code",
		"en":    "Enter the code",
		"zh":    "输入验证码",
		"fr":    "Enter the code",
		"zh-CN": "Enter the code",
	}
	for language, want := range cases {
		if got := textsFor(language).CodeTitle; got != want {
			t.Errorf("textsFor(%q).CodeTitle = %q, want %q", language, got, want)
		}
	}
}

// The pages speak Chinese when the front-end asks for it, from the heading to
// the form's own validation.
func TestPagesSpeakChinese(t *testing.T) {
	p := newCodePage(authchallenge.CodeChallenge{Kind: authchallenge.CodeSMS}, testToken, chinese)
	body := getPage(p.mux, "/"+testToken+"/").Body.String()
	for _, want := range []string{`lang="zh-CN"`, "输入验证码", "提交"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page = %q, want %q", body, want)
		}
	}
	recorder := postForm(p.mux, "/"+testToken+"/", url.Values{})
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "请输入验证码") {
		t.Fatalf("empty form = %d %q", recorder.Code, recorder.Body.String())
	}
}
