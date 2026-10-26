// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mythologyli/zju-connect/client/atrust"
	"github.com/mythologyli/zju-connect/client/authchallenge"

	"github.com/YangtseSu/goTrust/core/config"
)

// reconnectDelays is the ladder: five attempts, doubling from one second. The
// cap is deliberate. A full login rotates the device token and drops other
// clients of the same account, so an unbounded ladder would keep turning a dead
// session into a login storm.
var reconnectDelays = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
}

// reconnectDelay is the wait before one attempt, clamped to the schedule so a
// bad caller cannot walk past the cap.
func reconnectDelay(attempt int) time.Duration {
	if attempt < 1 {
		return reconnectDelays[0]
	}
	if attempt > len(reconnectDelays) {
		return reconnectDelays[len(reconnectDelays)-1]
	}
	return reconnectDelays[attempt-1]
}

// ReconnectState is the ladder position, shown while a lost session is retried.
// Attempt is zero when no ladder is running.
type ReconnectState struct {
	// Attempt is the 1-based attempt that is about to run or is running.
	Attempt int
	// Max is the number of attempts the ladder will make.
	Max int
	// NextAt is when the next attempt starts, zero while one is in flight.
	NextAt time.Time
}

// KeepAliveState is the last application-traffic probe, reported by the
// upstream loop through its result hook.
type KeepAliveState struct {
	// Target is the URL that was probed, or the remote-DNS fallback name.
	Target string
	// Named reports whether the user configured this target.
	Named bool
	// Status is the HTTP answer; zero for a transport error or a DNS probe.
	Status int
	// OK reports whether the probe counted as application traffic.
	OK bool
	// Err is the transport or lookup error, empty when the probe got an answer.
	Err string
	// Fallback marks the remote-DNS probe, which does not hold the session.
	Fallback bool
	// At is when the probe finished. Zero means no probe has run.
	At time.Time
}

// observeSessionState mirrors what the embedded client learned about the
// session. It is called from the client's goroutine, so it only publishes state
// and starts the ladder; the work itself runs elsewhere.
func (m *Manager) observeSessionState(source *atrust.Client, state atrust.SessionState, reason string) {
	m.mu.RLock()
	live := m.client == source
	phase := m.state.Phase
	m.mu.RUnlock()
	if !live {
		// Setup emits its own transitions before the client is handed over, and
		// a closed client may still report during teardown.
		return
	}

	switch state {
	case atrust.SessionRestoring:
		// The client's own ladder is renewing the session. Show it as
		// reconnecting, but do not start a second ladder on top.
		if m.reconnectRunning() {
			return
		}
		m.setTransition(State{
			Phase:     PhaseReconnecting,
			Detail:    "正在恢复会话",
			ClientIP:  m.tunnelIP(),
			Resources: m.resources(),
			Reconnect: ReconnectState{Max: len(reconnectDelays)},
		})
	case atrust.SessionOnline:
		if phase != PhaseReconnecting || m.reconnectRunning() {
			return
		}
		m.setTransition(State{
			Phase:     PhaseOnline,
			Detail:    m.gatewayAddress(),
			ClientIP:  m.tunnelIP(),
			Resources: m.resources(),
		})
	case atrust.SessionReloginRequired:
		if m.reconnectRunning() {
			return
		}
		m.onSessionLost(reason)
	}
}

// onSessionLost decides what happens when the client says a new login is
// required. The client has already tried its own renewal (and an upstream
// ticket restore where the portal still had the session); goTrust does not
// reuse either: its ladder makes a fresh login.
func (m *Manager) onSessionLost(reason string) {
	m.logf("session lost: %s", reason)

	cfg := m.settings()
	profile := cfg.Active()
	if !cfg.App.AutoReconnect {
		m.expire("会话已过期，需要重新登录")
		return
	}
	if !hasStoredPassword(profile) {
		m.expire("会话已过期，且没有保存的密码，请手动重新登录")
		return
	}
	m.startReconnect(reason)
}

// hasStoredPassword reports whether a reconnect can log in without asking the
// user. A session cannot outlive its connection: stored client data is device
// material, not a reusable session, so only a saved password counts. (Both
// atrust-quadlet and zju-connect tried to revive a session from the stored
// sid ticket and the portal ended it anyway; the attempt only delayed the
// login.)
func hasStoredPassword(profile config.Profile) bool {
	return strings.TrimSpace(profile.Auth.Password) != ""
}

// startReconnect launches the ladder, unless one already runs. The dead client
// is torn down in the ladder goroutine, so the client's own callback returns
// immediately.
func (m *Manager) startReconnect(reason string) {
	m.reconnectMu.Lock()
	if m.reconnectCancel != nil {
		m.reconnectMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.reconnectCancel = cancel
	m.reconnectMu.Unlock()

	m.logf("auto-reconnect: %d attempt(s), waiting %s first", len(reconnectDelays), reconnectDelays[0])
	go m.runReconnect(ctx, reason)
}

func (m *Manager) runReconnect(ctx context.Context, reason string) {
	defer m.endReconnect()

	// A dead client keeps Dial handing out a tunnel that rejects everything.
	m.teardown()

	resources := m.resources()

	attempts, lastErr, stop := reconnectLadder(ctx, len(reconnectDelays),
		func(attempt int) bool {
			return m.waitForReconnect(ctx, attempt, reason)
		},
		func(ctx context.Context, attempt int) error {
			m.logf("auto-reconnect: attempt %d/%d", attempt, len(reconnectDelays))
			m.challenge.Store(false)
			return m.loginWithRetry(ctx)
		},
		m.challenge.Load,
	)
	m.logf("auto-reconnect: stopped after %d attempt(s): %s", attempts, stop)

	switch stop {
	case reconnectChallenge:
		// A retry cannot answer a captcha, a one-time code or an SSO callback.
		// The prompt is on screen; stop and let the person finish.
		m.set(failureState("自动重连已停止：登录需要你完成验证", true, resources))
	case reconnectExhausted:
		m.set(failureState(
			fmt.Sprintf("自动重连 %d 次后仍失败: %v", attempts, lastErr),
			true, resources))
	}
}

// reconnectStop says why the ladder ended. Only the stops that leave the user
// something to do are published; success and a user disconnect are not failures.
type reconnectStop int

const (
	reconnectSucceeded reconnectStop = iota
	reconnectCancelled
	reconnectChallenge
	reconnectExhausted
)

func (s reconnectStop) String() string {
	switch s {
	case reconnectSucceeded:
		return "connected"
	case reconnectCancelled:
		return "cancelled"
	case reconnectChallenge:
		return "a login prompt is waiting"
	case reconnectExhausted:
		return "attempts exhausted"
	default:
		return "unknown"
	}
}

// reconnectLadder runs the attempts and reports the outcome. The wait and the
// attempt are injected so the schedule, the cap and every stop condition are
// tested without a gateway or a 31-second wall clock.
func reconnectLadder(
	ctx context.Context,
	maxAttempts int,
	wait func(attempt int) bool,
	attempt func(ctx context.Context, number int) error,
	challenged func() bool,
) (int, error, reconnectStop) {
	var lastErr error
	done := 0
	for number := 1; number <= maxAttempts; number++ {
		if !wait(number) {
			return done, lastErr, reconnectCancelled
		}
		done = number
		lastErr = attempt(ctx, number)
		if lastErr == nil {
			return done, nil, reconnectSucceeded
		}
		if ctx.Err() != nil {
			return done, lastErr, reconnectCancelled
		}
		if challenged() {
			return done, lastErr, reconnectChallenge
		}
	}
	return done, lastErr, reconnectExhausted
}

// waitForReconnect publishes the countdown and reports whether the attempt may
// start. It emits one state per second while waiting, so the window and the tray
// count down instead of freezing at the first number.
func (m *Manager) waitForReconnect(ctx context.Context, attempt int, reason string) bool {
	deadline := time.Now().Add(reconnectDelay(attempt))
	m.setReconnectWait(attempt, reason, deadline)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if time.Until(deadline) <= 0 {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			m.setReconnectWait(attempt, reason, deadline)
		}
	}
}

func (m *Manager) setReconnectWait(attempt int, reason string, next time.Time) {
	m.setTransition(State{
		Phase:     PhaseReconnecting,
		Detail:    reason,
		ClientIP:  m.tunnelIP(),
		Resources: m.resources(),
		Reconnect: ReconnectState{
			Attempt: attempt,
			Max:     len(reconnectDelays),
			NextAt:  next,
		},
	})
}

// expire stops pretending the tunnel works: the client is closed, Dial refuses,
// and the state says what the user has to do.
func (m *Manager) expire(detail string) {
	resources := m.resources()
	m.teardown()
	m.set(failureState(detail, true, resources))
}

// failureState keeps the resources the account was granted visible: they did not
// change, only the tunnel did.
func failureState(detail string, attention bool, resources Resources) State {
	return State{
		Phase:     PhaseFailed,
		Detail:    detail,
		Attention: attention,
		Resources: resources,
		Since:     time.Now(),
	}
}

// stopReconnect cancels a running ladder. It does not wait: the ladder notices
// its context and returns on its own.
func (m *Manager) stopReconnect() {
	m.reconnectMu.Lock()
	cancel := m.reconnectCancel
	m.reconnectMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) reconnectRunning() bool {
	m.reconnectMu.Lock()
	defer m.reconnectMu.Unlock()
	return m.reconnectCancel != nil
}

func (m *Manager) endReconnect() {
	m.reconnectMu.Lock()
	m.reconnectCancel = nil
	m.reconnectMu.Unlock()
}

// observingPrompter records that the login needs a person. The ladder uses it to
// stop instead of retrying past a prompt nobody will answer twice.
type observingPrompter struct {
	inner  Prompter
	notify func()
}

func (p observingPrompter) HandleCodeChallenge(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
	p.notify()
	return p.inner.HandleCodeChallenge(challenge)
}

func (p observingPrompter) HandleTextCaptcha(challenge authchallenge.TextCaptchaChallenge) (authchallenge.TextCaptchaResponse, error) {
	p.notify()
	return p.inner.HandleTextCaptcha(challenge)
}

func (p observingPrompter) HandleClickCaptcha(challenge authchallenge.ClickCaptchaChallenge) (authchallenge.ClickCaptchaResponse, error) {
	p.notify()
	return p.inner.HandleClickCaptcha(challenge)
}

func (p observingPrompter) HandleExternalLogin(challenge authchallenge.ExternalLoginChallenge) (authchallenge.ExternalLoginResponse, error) {
	p.notify()
	return p.inner.HandleExternalLogin(challenge)
}
