// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mythologyli/zju-connect/client/atrust/auth"

	"github.com/YangtseSu/goTrust/config"
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
