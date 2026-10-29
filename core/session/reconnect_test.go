// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	clientpkg "github.com/YangtseSu/goTrust/core/atrust/client"
	"github.com/YangtseSu/goTrust/core/atrust/client/atrust"
	"github.com/YangtseSu/goTrust/core/atrust/service"

	"github.com/YangtseSu/goTrust/core/config"
)

// The ladder is a fixed promise: 1/2/4/8/16 s, five attempts. A change here
// changes how the account looks to the gateway, so pin it.
func TestReconnectScheduleIsTheDoublingLadder(t *testing.T) {
	want := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}
	if !slices.Equal(reconnectDelays, want) {
		t.Fatalf("schedule = %v, want %v", reconnectDelays, want)
	}
	if len(reconnectDelays) != 5 {
		t.Fatalf("attempt cap = %d, want 5", len(reconnectDelays))
	}
	for attempt := 1; attempt <= len(want); attempt++ {
		if got := reconnectDelay(attempt); got != want[attempt-1] {
			t.Fatalf("delay(%d) = %s, want %s", attempt, got, want[attempt-1])
		}
	}
	if got := reconnectDelay(0); got != want[0] {
		t.Fatalf("delay(0) = %s, want the first step", got)
	}
	if got := reconnectDelay(99); got != want[len(want)-1] {
		t.Fatalf("delay(99) = %s, want the last step", got)
	}
}

// The ladder stops at the cap instead of retrying forever: every full login
// rotates the device token and drops other clients of the account.
func TestReconnectLadderStopsAtTheCap(t *testing.T) {
	failure := errors.New("gateway unreachable")
	var attempts int
	attemptsMade, lastErr, stop := reconnectLadder(
		context.Background(), len(reconnectDelays),
		func(int) bool { return true },
		func(context.Context, int) error {
			attempts++
			return failure
		},
		func() bool { return false },
	)

	if stop != reconnectExhausted {
		t.Fatalf("stop = %s, want exhausted", stop)
	}
	if attemptsMade != len(reconnectDelays) || attempts != len(reconnectDelays) {
		t.Fatalf("attempts = (%d, %d), want %d", attemptsMade, attempts, len(reconnectDelays))
	}
	if !errors.Is(lastErr, failure) {
		t.Fatalf("last error = %v, want the attempt failure", lastErr)
	}
}

// A captcha, a one-time code or an SSO callback needs a person. Retrying past
// the prompt would only repeat the same question, so the ladder stops.
func TestReconnectLadderStopsOnAChallenge(t *testing.T) {
	var attempts int
	challenge := false
	attemptsMade, _, stop := reconnectLadder(
		context.Background(), len(reconnectDelays),
		func(int) bool { return true },
		func(context.Context, int) error {
			attempts++
			if attempts == 2 {
				challenge = true
			}
			return errors.New("login cancelled by the user")
		},
		func() bool { return challenge },
	)

	if stop != reconnectChallenge {
		t.Fatalf("stop = %s, want challenge", stop)
	}
	if attemptsMade != 2 || attempts != 2 {
		t.Fatalf("attempts = (%d, %d), want 2", attemptsMade, attempts)
	}
}

// A user who presses Disconnect cancels the wait, not just the attempt in flight.
func TestReconnectLadderStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attemptsMade, _, stop := reconnectLadder(
		ctx, len(reconnectDelays),
		func(int) bool { return false },
		func(context.Context, int) error {
			t.Fatal("an attempt ran after the ladder was cancelled")
			return nil
		},
		func() bool { return false },
	)

	if stop != reconnectCancelled || attemptsMade != 0 {
		t.Fatalf("stop = %d after %d attempts, want cancelled before the first", stop, attemptsMade)
	}
}

// A lost session with auto-reconnect off must say what the user has to do,
// instead of leaving an online-looking window over a dead tunnel.
func TestSessionLossWithoutAutoReconnectExpires(t *testing.T) {
	var states []State
	manager := New(Options{
		Config:  func() config.Config { return config.Default() },
		OnState: func(state State) { states = append(states, state) },
	})
	client := atrust.NewClient(atrust.ClientOptions{})
	t.Cleanup(client.Close)
	manager.mu.Lock()
	manager.client = client
	manager.mu.Unlock()

	manager.observeSessionState(client, atrust.SessionReloginRequired, "session refresh")

	state := manager.State()
	if state.Phase != PhaseFailed {
		t.Fatalf("phase = %q, want %q", state.Phase, PhaseFailed)
	}
	if !state.Attention {
		t.Fatal("an expiring session did not ask for attention")
	}
	if len(states) == 0 || !states[len(states)-1].Attention {
		t.Fatalf("observed states = %+v, want a final state with attention", states)
	}

	// The dead client is gone: a dial answers ErrNotOnline instead of using a
	// tunnel that rejects everything.
	if _, err := manager.Dial(context.Background(), "tcp", "192.0.2.10:80"); !errors.Is(err, ErrNotOnline) {
		t.Fatalf("dial error = %v, want ErrNotOnline", err)
	}
}

// Auto-reconnect without a saved password has nothing to try, and must say so
// rather than starting a ladder of empty logins. A stored session is not a
// credential: it ends with its connection.
func TestSessionLossWithoutAPasswordExpires(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := config.Default()
	cfg.App.AutoReconnect = true

	var states []State
	manager := New(Options{
		Config:  func() config.Config { return cfg },
		OnState: func(state State) { states = append(states, state) },
	})
	client := atrust.NewClient(atrust.ClientOptions{})
	t.Cleanup(client.Close)
	manager.mu.Lock()
	manager.client = client
	manager.mu.Unlock()

	manager.observeSessionState(client, atrust.SessionReloginRequired, "session refresh")

	state := manager.State()
	if state.Phase != PhaseFailed || !state.Attention {
		t.Fatalf("state = %+v, want a failed state with attention", state)
	}
	if manager.reconnectRunning() {
		t.Fatal("a ladder started without a saved password")
	}
}

// The client's own restore ladder is shown as reconnecting, and a later online
// transition clears the ladder without claiming the tunnel was never lost.
func TestRestoreTransitionsAreMirrored(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	manager := New(Options{Config: func() config.Config { return cfg }})
	client := atrust.NewClient(atrust.ClientOptions{})
	t.Cleanup(client.Close)
	manager.mu.Lock()
	manager.client = client
	manager.state = State{
		Phase:     PhaseOnline,
		Detail:    "vpn.example.com:443",
		ClientIP:  "192.0.2.10",
		Resources: Resources{},
		Since:     time.Now(),
	}
	manager.mu.Unlock()

	manager.observeSessionState(client, atrust.SessionRestoring, "renewal failed")
	if state := manager.State(); state.Phase != PhaseReconnecting {
		t.Fatalf("phase = %q, want reconnecting while the client restores", state.Phase)
	}

	manager.observeSessionState(client, atrust.SessionOnline, "renewal succeeded")
	state := manager.State()
	if state.Phase != PhaseOnline {
		t.Fatalf("phase = %q, want online after the restore", state.Phase)
	}
	if state.Detail != "vpn.example.com:443" {
		t.Fatalf("detail = %q, want the gateway address", state.Detail)
	}
	if state.ClientIP != "192.0.2.10" {
		t.Fatalf("client IP = %q, want the address kept across the restore", state.ClientIP)
	}
}

// A stale callback from a closed client must not change the new session's
// state: every login creates a client that reports its own transitions.
func TestForeignClientTransitionsAreIgnored(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})
	live := atrust.NewClient(atrust.ClientOptions{})
	stale := atrust.NewClient(atrust.ClientOptions{})
	t.Cleanup(live.Close)
	t.Cleanup(stale.Close)
	manager.mu.Lock()
	manager.client = live
	manager.state = State{Phase: PhaseOnline, Detail: "vpn.example.com:443", Since: time.Now()}
	manager.mu.Unlock()

	manager.observeSessionState(stale, atrust.SessionReloginRequired, "old client")

	if state := manager.State(); state.Phase != PhaseOnline {
		t.Fatalf("phase = %q, want the live session untouched", state.Phase)
	}
}

// Keep-alive results are evidence, not transitions: a failed probe keeps the
// phase, and the next successful probe replaces the failure.
func TestKeepAliveResultsUpdateTheState(t *testing.T) {
	var states []State
	manager := New(Options{
		Config:  func() config.Config { return config.Default() },
		OnState: func(state State) { states = append(states, state) },
	})
	manager.set(State{Phase: PhaseOnline, Detail: "vpn.example.com:443", Since: time.Now()})

	at := time.Now()
	manager.recordKeepAlive(service.KeepAliveResult{
		Target: "http://192.0.2.10/", Named: true, StatusCode: 200, OK: true, At: at,
	})

	state := manager.State()
	if !state.KeepAlive.OK || state.KeepAlive.Status != 200 || state.KeepAlive.Target != "http://192.0.2.10/" {
		t.Fatalf("keep-alive = %+v, want the successful probe", state.KeepAlive)
	}
	if state.KeepAlive.At != at || state.Phase != PhaseOnline {
		t.Fatalf("state = %+v, want the probe timestamp and the online phase", state)
	}

	manager.recordKeepAlive(service.KeepAliveResult{
		Target: "http://192.0.2.10/", Err: errors.New("dial timeout"), At: at.Add(time.Minute),
	})
	state = manager.State()
	if state.KeepAlive.OK || state.KeepAlive.Err != "dial timeout" {
		t.Fatalf("keep-alive = %+v, want the failed probe", state.KeepAlive)
	}
	if state.Phase != PhaseOnline {
		t.Fatalf("phase = %q, a failed probe must not fake a transition", state.Phase)
	}
	if len(states) != 3 {
		t.Fatalf("observed %d states, want the initial one and one per probe", len(states))
	}
}

// The user's targets are probed first and the derived candidates after them,
// so a deliberately chosen health URL always wins placement.
func TestKeepAliveTargetsPutNamedBeforeDerived(t *testing.T) {
	profile := config.Default().Active()
	profile.Keep.Targets = []string{"http://192.0.2.5/health"}
	resources := []clientpkg.IPResource{{
		IPMin:    net.ParseIP("192.0.2.10"),
		IPMax:    net.ParseIP("192.0.2.10"),
		PortMin:  80,
		PortMax:  80,
		Protocol: "tcp",
	}}

	targets, _ := keepAliveTargets(profile, resources)
	if len(targets) != 2 {
		t.Fatalf("targets = %+v, want the named one and one derived", targets)
	}
	if targets[0].URL != "http://192.0.2.5/health" || !targets[0].Named {
		t.Fatalf("first target = %+v, want the named one", targets[0])
	}
	if targets[1].URL != "http://192.0.2.10/" || targets[1].Named {
		t.Fatalf("second target = %+v, want the derived candidate", targets[1])
	}
}
