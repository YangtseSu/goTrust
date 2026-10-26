// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mythologyli/zju-connect/client/atrust/auth"

	"github.com/YangtseSu/goTrust/core/config"
)

// The password calls go through the live client, so an idle manager has to
// refuse them instead of reaching into a nil client.
func TestPasswordActionsNeedALiveSession(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})

	if err := manager.ResetPassword(context.Background(), "old", "new"); !errors.Is(err, ErrNoLiveSession) {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if _, err := manager.PasswordStrategy(context.Background()); !errors.Is(err, ErrNoLiveSession) {
		t.Fatalf("PasswordStrategy() error = %v", err)
	}
	if _, err := manager.CheckWeakPassword(context.Background(), "123456"); !errors.Is(err, ErrNoLiveSession) {
		t.Fatalf("CheckWeakPassword() error = %v", err)
	}
}

// A password change ends the pending login; the manager retries exactly once
// with the credential the prompt saved, and reports a second demand instead of
// looping.
func TestRetryAfterReauth(t *testing.T) {
	ctx := context.Background()
	sentinel := auth.ErrReauthRequired

	calls := 0
	retried, err := retryAfterReauth(ctx,
		func(context.Context) error { calls++; return sentinel },
		func(context.Context) error { calls++; return nil })
	if !retried || err != nil || calls != 2 {
		t.Fatalf("success retry = retried %v, err %v, calls %d", retried, err, calls)
	}

	calls = 0
	retried, err = retryAfterReauth(ctx,
		func(context.Context) error { calls++; return errors.New("offline") },
		func(context.Context) error { calls++; return nil })
	if retried || err == nil || calls != 1 {
		t.Fatalf("other error = retried %v, err %v, calls %d", retried, err, calls)
	}

	calls = 0
	retried, err = retryAfterReauth(ctx,
		func(context.Context) error { calls++; return sentinel },
		func(context.Context) error { calls++; return sentinel })
	if !retried || !errors.Is(err, sentinel) || calls != 2 {
		t.Fatalf("second demand = retried %v, err %v, calls %d", retried, err, calls)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	calls = 0
	retried, err = retryAfterReauth(cancelled,
		func(context.Context) error { calls++; return sentinel },
		func(context.Context) error { calls++; return nil })
	if retried || !errors.Is(err, sentinel) || calls != 1 {
		t.Fatalf("cancelled = retried %v, err %v, calls %d", retried, err, calls)
	}
}

// A headless login that meets a forced change must say what it needs instead of
// failing with the client's generic message.
func TestPasswordChangeUnavailableExplainsTheWindow(t *testing.T) {
	required := &auth.PasswordChangeRequiredError{Challenge: auth.PasswordChangeChallenge{
		Message: "密码已过期，请修改",
	}}
	err := passwordChangeUnavailable(required)
	if err == nil || !strings.Contains(err.Error(), "密码已过期") || !strings.Contains(err.Error(), "window") {
		t.Fatalf("message = %v", err)
	}

	empty := passwordChangeUnavailable(&auth.PasswordChangeRequiredError{})
	if !strings.Contains(empty.Error(), "password has to be changed") {
		t.Fatalf("empty message = %v", empty)
	}
}
