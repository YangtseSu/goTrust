// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

// forcedPasswordChallenge is the gateway's demand as the session layer reports
// it.
func forcedPasswordChallenge() session.PasswordChangeChallenge {
	var strategy session.PasswordStrategy
	strategy.EnablePswLength.Enable = true
	strategy.EnablePswLength.Value = 10
	return session.PasswordChangeChallenge{
		Code:     75500404,
		Message:  "密码已过期",
		Username: "alice",
		Domain:   "local",
		CanSkip:  true,
		Strategy: strategy,
	}
}

func waitForPasswordPanel(t *testing.T, testApp *app) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		testApp.mu.RLock()
		prompt := testApp.view.Password
		testApp.mu.RUnlock()
		if prompt != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the password panel did not appear")
		}
		time.Sleep(time.Millisecond)
	}
}

// A forced change hands the window's answer back to the login goroutine, and a
// skip is only sent when the gateway offered it.
func TestForcedPasswordChangeSubmits(t *testing.T) {
	testApp := promptTestApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan passwordResult, 1)
	go func() {
		answer, err := testApp.changePasswordPrompt(ctx, forcedPasswordChallenge())
		results <- passwordResult{answer: answer, err: err}
	}()

	waitForPasswordPanel(t, testApp)
	testApp.mu.Lock()
	testApp.view.Password.OldPassword = "old-password"
	testApp.view.Password.NewPassword = "new-password-1"
	testApp.view.Password.Confirm = "new-password-1"
	testApp.mu.Unlock()
	testApp.submitPassword()

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("changePasswordPrompt() error = %v", result.err)
		}
		if result.answer.OldPassword != "old-password" || result.answer.NewPassword != "new-password-1" {
			t.Fatalf("answer = %+v", result.answer)
		}
		if result.answer.Skip {
			t.Fatal("a completed form was sent as a skip")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the answer did not reach the login goroutine")
	}

	testApp.mu.RLock()
	cleared := testApp.view.Password == nil
	testApp.mu.RUnlock()
	if !cleared {
		t.Fatal("the panel stayed after the answer")
	}
}

func TestForcedPasswordChangeSkips(t *testing.T) {
	testApp := promptTestApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan passwordResult, 1)
	go func() {
		answer, err := testApp.changePasswordPrompt(ctx, forcedPasswordChallenge())
		results <- passwordResult{answer: answer, err: err}
	}()

	waitForPasswordPanel(t, testApp)
	testApp.mu.Lock()
	testApp.view.Password.Skip = true
	testApp.mu.Unlock()
	testApp.submitPassword()

	select {
	case result := <-results:
		if result.err != nil || !result.answer.Skip {
			t.Fatalf("skip answer = %+v, err = %v", result.answer, result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the skip did not reach the login goroutine")
	}
}

// A typo keeps the panel and explains itself instead of burning a login
// attempt. Cancelling then releases the login goroutine.
func TestForcedPasswordChangeValidationAndCancel(t *testing.T) {
	testApp := promptTestApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan passwordResult, 1)
	go func() {
		answer, err := testApp.changePasswordPrompt(ctx, forcedPasswordChallenge())
		results <- passwordResult{answer: answer, err: err}
	}()

	waitForPasswordPanel(t, testApp)
	testApp.mu.Lock()
	testApp.view.Password.OldPassword = "old-password"
	testApp.view.Password.NewPassword = "short"
	testApp.view.Password.Confirm = "short"
	testApp.mu.Unlock()
	testApp.submitPassword()

	testApp.mu.RLock()
	stillOpen := testApp.view.Password != nil
	status := testApp.view.Status
	testApp.mu.RUnlock()
	if !stillOpen {
		t.Fatal("an invalid form closed the panel")
	}
	if !strings.Contains(status, "长度不足") {
		t.Fatalf("validation status = %q", status)
	}

	testApp.cancelPassword(errors.New("cancelled"))
	select {
	case result := <-results:
		if result.err == nil {
			t.Fatal("cancelling the panel answered the login")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the cancellation did not reach the login goroutine")
	}
}

// Ending the login attempt releases a panel nobody will answer.
func TestForcedPasswordChangeHonoursTheContext(t *testing.T) {
	testApp := promptTestApp()
	ctx, cancel := context.WithCancel(context.Background())

	results := make(chan passwordResult, 1)
	go func() {
		answer, err := testApp.changePasswordPrompt(ctx, forcedPasswordChallenge())
		results <- passwordResult{answer: answer, err: err}
	}()

	waitForPasswordPanel(t, testApp)
	cancel()

	select {
	case result := <-results:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the cancelled context did not release the prompt")
	}
}

// The changed credential has to survive the process: the next login would
// otherwise fail with the old password. A profile that does not store
// passwords keeps it in memory only.
func TestStoreChangedPasswordWritesTheCredential(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Default()
	cfg.Profiles[0].Auth.Username = "alice"
	cfg.Profiles[0].Auth.Password = "old-password"
	cfg.Profiles[0].Auth.StorePass = true
	if err := config.SaveFile(path, cfg); err != nil {
		t.Fatalf("save the test config: %v", err)
	}

	testApp := promptTestApp()
	testApp.cfg = cfg
	testApp.configPath = path
	testApp.settings = gotrustui.NewSettingsState(cfg, dir)

	var publishes int
	testApp.runOnUI = func(fn func()) {
		publishes++
		fn()
	}
	testApp.storeChangedPassword(cfg.Profiles[0].Name, "new-password")
	if publishes != 1 {
		t.Fatalf("draft publishes through the UI seam = %d, want 1", publishes)
	}

	testApp.mu.RLock()
	live := testApp.cfg.Active().Auth.Password
	draft := testApp.settings.Draft.Active().Auth.Password
	testApp.mu.RUnlock()
	if live != "new-password" || draft != "new-password" {
		t.Fatalf("live = %q, draft = %q", live, draft)
	}

	stored, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("reload the config: %v", err)
	}
	if stored.Active().Auth.Password != "new-password" {
		t.Fatalf("stored password = %q", stored.Active().Auth.Password)
	}

	// Without 保存密码 the file must not carry the new credential, but the
	// running configuration still needs it for the next login.
	cfg.Profiles[0].Auth.StorePass = false
	if err := config.SaveFile(path, cfg.ForDisk()); err != nil {
		t.Fatalf("save the test config: %v", err)
	}
	testApp.cfg = cfg
	testApp.storeChangedPassword(cfg.Profiles[0].Name, "second-password")

	stored, err = config.LoadFile(path)
	if err != nil {
		t.Fatalf("reload the config: %v", err)
	}
	if stored.Active().Auth.Password != "" {
		t.Fatalf("an unstored password reached the file: %q", stored.Active().Auth.Password)
	}
	testApp.mu.RLock()
	live = testApp.cfg.Active().Auth.Password
	testApp.mu.RUnlock()
	if live != "second-password" {
		t.Fatalf("live password = %q", live)
	}
}

// The voluntary flow needs the live session the portal authenticates with; an
// idle manager has to say so instead of pretending to change anything.
func TestVoluntaryPasswordChangeNeedsASession(t *testing.T) {
	cfg := config.Default()
	testApp := promptTestApp()
	testApp.manager = session.New(session.Options{Config: func() config.Config { return cfg }})

	if _, err := testApp.manager.PasswordStrategy(context.Background()); !errors.Is(err, session.ErrNoLiveSession) {
		t.Fatalf("PasswordStrategy() error = %v, want ErrNoLiveSession", err)
	}
	if err := testApp.changePasswordOnline(context.Background(), "old", "new-password-1"); !errors.Is(err, session.ErrNoLiveSession) {
		t.Fatalf("changePasswordOnline() error = %v, want ErrNoLiveSession", err)
	}
}
