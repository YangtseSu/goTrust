// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/YangtseSu/goTrust/core/atrust/client/atrust/auth"
)

// PasswordStrategy is the account's password policy. The alias keeps the UI on
// this package instead of the upstream client.
type PasswordStrategy = auth.PasswordStrategy

// PasswordRule is one line of the policy panel.
type PasswordRule = auth.PasswordRule

// PasswordRuleCode names a rule so the window can render its own wording.
type PasswordRuleCode = auth.PasswordRuleCode

const (
	// RuleLength is the minimum length.
	RuleLength = auth.RuleLength
	// RuleLetters requires a letter.
	RuleLetters = auth.RuleLetters
	// RuleUpperAndLower requires both cases.
	RuleUpperAndLower = auth.RuleUpperAndLower
	// RuleNumber requires a digit.
	RuleNumber = auth.RuleNumber
	// RuleSpecial requires a special character.
	RuleSpecial = auth.RuleSpecial
	// RuleNoUsername forbids the account name inside the password.
	RuleNoUsername = auth.RuleNoUsername
	// RuleNoRepeat forbids a repeated character run.
	RuleNoRepeat = auth.RuleNoRepeat
	// RuleNoSequence forbids keyboard or alphabet runs (portal-checked).
	RuleNoSequence = auth.RuleNoSequence
	// RuleNotWeak forbids a weak-database password (portal-checked).
	RuleNotWeak = auth.RuleNotWeak
	// RuleNotInHistory forbids reusing a recent password (portal-checked).
	RuleNotInHistory = auth.RuleNotInHistory
)

// PasswordChangeChallenge is the gateway's forced password change.
type PasswordChangeChallenge = auth.PasswordChangeChallenge

// PasswordChangeAnswer is what the window returns for a forced change.
type PasswordChangeAnswer = auth.PasswordChangeResponse

// WeakPasswordResult is the portal's answer about a candidate password.
type WeakPasswordResult = auth.WeakPasswordResult

// ErrNoLiveSession reports a password action without a live client.
var ErrNoLiveSession = errors.New("the session is not online")

// passwordChangeUnavailable turns the gateway's typed demand into a failure a
// run without a window can report: there is nobody to ask for the new password.
func passwordChangeUnavailable(required *auth.PasswordChangeRequiredError) error {
	reason := required.Challenge.Message
	if reason == "" {
		reason = "the account's password has to be changed"
	}
	return fmt.Errorf("%s; setting a new password needs the window, so leave --no-ui off and log in again", reason)
}

// LiveProfile returns the profile the live session belongs to, empty when no
// session exists. A credential rewrite uses it instead of the currently active
// profile, which may have changed since the login.
func (m *Manager) LiveProfile() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.liveProfile
}

// ResetPassword resets the live session's password. It is the voluntary flow
// (个人信息 → 密码安全): the portal is expected to end other terminals'
// sessions afterwards.
func (m *Manager) ResetPassword(ctx context.Context, oldPassword, newPassword string) error {
	m.mu.RLock()
	client := m.client
	online := m.state.Phase == PhaseOnline
	m.mu.RUnlock()
	if client == nil || !online {
		return ErrNoLiveSession
	}
	return client.ResetPassword(ctx, oldPassword, newPassword)
}

// PasswordStrategy reads the account's password policy from the live session.
func (m *Manager) PasswordStrategy(ctx context.Context) (PasswordStrategy, error) {
	m.mu.RLock()
	client := m.client
	online := m.state.Phase == PhaseOnline
	m.mu.RUnlock()
	if client == nil || !online {
		return PasswordStrategy{}, ErrNoLiveSession
	}
	return client.PasswordStrategy(ctx)
}

// CheckWeakPassword asks the portal's weak-password database about a candidate
// password. The caller still has to send the change: this only answers whether
// the password is a common one.
func (m *Manager) CheckWeakPassword(ctx context.Context, password string) (WeakPasswordResult, error) {
	m.mu.RLock()
	client := m.client
	online := m.state.Phase == PhaseOnline
	m.mu.RUnlock()
	if client == nil || !online {
		return WeakPasswordResult{}, ErrNoLiveSession
	}
	return client.CheckWeakPassword(ctx, password)
}
