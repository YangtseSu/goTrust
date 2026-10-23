// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/config"
)

// A manager that was never logged in must refuse to dial instead of handing
// back a dialer that does not exist: the proxy layer starts before the session
// is up far more often than it looks.
func TestDialRefusesWhileIdle(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})

	_, err := manager.Dial(context.Background(), "tcp", "example.com:80")
	if !errors.Is(err, ErrNotOnline) {
		t.Fatalf("err = %v, want ErrNotOnline", err)
	}
}

func TestInitialStateIsIdle(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})

	state := manager.State()
	if state.Phase != PhaseIdle {
		t.Fatalf("phase = %q, want %q", state.Phase, PhaseIdle)
	}
	if state.Since.IsZero() {
		t.Fatal("the initial state has no timestamp")
	}
}

// Invalid settings must fail before anything is dialled, and must leave the
// manager in the failed phase with the reason attached.
func TestLoginRejectsInvalidConfig(t *testing.T) {
	empty := config.Default() // no server address, no username

	manager := New(Options{Config: func() config.Config { return empty }})
	err := manager.Login(context.Background())
	if err == nil {
		t.Fatal("login succeeded with an empty server address")
	}

	state := manager.State()
	if state.Phase != PhaseFailed {
		t.Fatalf("phase = %q, want %q", state.Phase, PhaseFailed)
	}
	if state.Detail == "" {
		t.Fatal("the failed state carries no reason")
	}
}

// A second login while one is in flight must be refused rather than silently
// replacing the first: a full login rotates the device token.
func TestLoginRefusesWhileOnline(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})
	manager.set(State{Phase: PhaseOnline, Detail: "test", Since: time.Now()})

	err := manager.Login(context.Background())
	if err == nil {
		t.Fatal("a second login was accepted while online")
	}
}

// State changes reach the observer, which is what keeps the window honest.
func TestStateChangesAreReported(t *testing.T) {
	var seen []Phase
	manager := New(Options{
		Config: func() config.Config { return config.Default() },
		OnState: func(state State) {
			seen = append(seen, state.Phase)
		},
	})

	manager.set(State{Phase: PhaseConnecting, Detail: "x", Since: time.Now()})
	manager.set(State{Phase: PhaseOnline, Detail: "y", Since: time.Now()})

	if len(seen) != 2 || seen[0] != PhaseConnecting || seen[1] != PhaseOnline {
		t.Fatalf("observed phases = %v", seen)
	}
}

// Credentials are read at login time, not captured at construction: the user
// edits the form after the manager exists.
func TestCredentialsAreReadAtLoginTime(t *testing.T) {
	username := "first"
	manager := New(Options{
		Config: func() config.Config { return config.Default() },
		Auth:   func() config.AuthConfig { return config.AuthConfig{Username: username} },
	})

	if got := manager.credentials().Username; got != "first" {
		t.Fatalf("username = %q, want first", got)
	}

	username = "second"
	if got := manager.credentials().Username; got != "second" {
		t.Fatalf("username = %q, want second", got)
	}
}

// Logout is idempotent: the UI calls it on quit even when nothing connected.
func TestLogoutIsSafeWhenIdle(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})

	manager.Logout()
	manager.Logout()

	if state := manager.State(); state.Phase != PhaseIdle {
		t.Fatalf("phase = %q, want %q", state.Phase, PhaseIdle)
	}
}

// Keep-alive targets come from the configuration first and from the granted
// resources second, so a user-specified target always wins placement.
func TestKeepAliveTargetsMergeNamedAndDerived(t *testing.T) {
	cfg := config.Default()
	cfg.Keep.Targets = []string{"http://192.0.2.5/health"}

	targets, _ := keepAliveTargets(cfg, nil)
	if len(targets) != 1 {
		t.Fatalf("targets = %v, want one", targets)
	}
	if targets[0].URL != "http://192.0.2.5/health" || !targets[0].Named {
		t.Fatalf("target = %+v, want the named target", targets[0])
	}

	empty := config.Default()
	empty.Keep.Targets = nil
	targets, _ = keepAliveTargets(empty, nil)
	if len(targets) != 0 {
		t.Fatalf("targets = %v, want none without a config or resources", targets)
	}
}

func TestJoinTargets(t *testing.T) {
	if got := joinTargets(nil); got != "" {
		t.Fatalf("join(nil) = %q", got)
	}
	if got := joinTargets([]string{"a", "b"}); got != "a,b" {
		t.Fatalf("join = %q, want a,b", got)
	}
}
