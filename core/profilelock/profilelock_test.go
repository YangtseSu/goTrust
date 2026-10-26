// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package profilelock

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// testConfig is a settings path in a temporary directory.
func testConfig(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.toml")
}

func TestSecondHolderIsRefused(t *testing.T) {
	path := testConfig(t)

	first, err := Acquire(path, "default")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer func() { _ = first.Release() }()

	second, err := Acquire(path, "default")
	if err == nil {
		_ = second.Release()
		t.Fatal("a second Acquire() on one profile succeeded")
	}
	var busy *BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("error = %v, want *BusyError", err)
	}
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("errors.Is(err, ErrBusy) = false for %v", err)
	}
	if busy.PID != os.Getpid() {
		t.Fatalf("busy holder pid = %d, want %d", busy.PID, os.Getpid())
	}
}

func TestReleaseFreesTheProfile(t *testing.T) {
	path := testConfig(t)

	lock, err := Acquire(path, "default")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	again, err := Acquire(path, "default")
	if err != nil {
		t.Fatalf("Acquire() after Release() error = %v", err)
	}
	if err := again.Release(); err != nil {
		t.Fatalf("second Release() error = %v", err)
	}
	// Releasing twice must stay safe: a teardown path may run it again.
	if err := again.Release(); err != nil {
		t.Fatalf("repeated Release() error = %v", err)
	}
}

func TestProfilesAndSettingsDoNotCollide(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "config.toml")
	second := filepath.Join(dir, "scratch.toml")

	for _, target := range []struct{ path, profile string }{
		{first, "work"},
		{first, "home"},
		{second, "work"},
	} {
		lock, err := Acquire(target.path, target.profile)
		if err != nil {
			t.Fatalf("Acquire(%q, %q) error = %v", target.path, target.profile, err)
		}
		defer func() { _ = lock.Release() }()
	}
}

func TestProbeReportsTheHolder(t *testing.T) {
	path := testConfig(t)

	if pid, held, err := Probe(path, "default"); err != nil || held || pid != 0 {
		t.Fatalf("Probe() before any lock = (%d, %v, %v), want (0, false, nil)", pid, held, err)
	}

	lock, err := Acquire(path, "default")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	pid, held, err := Probe(path, "default")
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if !held || pid != os.Getpid() {
		t.Fatalf("Probe() = (%d, %v), want (%d, true)", pid, held, os.Getpid())
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	if pid, held, err := Probe(path, "default"); err != nil || held || pid != 0 {
		t.Fatalf("Probe() after Release() = (%d, %v, %v), want (0, false, nil)", pid, held, err)
	}
}

func TestAcquireCreatesTheDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "config.toml")

	lock, err := Acquire(path, "default")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer func() { _ = lock.Release() }()

	if _, err := os.Stat(lockPath(path, "default")); err != nil {
		t.Fatalf("the lock file was not created: %v", err)
	}
}

func TestLockFileIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not keep Unix permission bits")
	}
	path := testConfig(t)

	lock, err := Acquire(path, "default")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer func() { _ = lock.Release() }()

	info, err := os.Stat(lockPath(path, "default"))
	if err != nil {
		t.Fatalf("stat the lock file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("lock file mode = %o, want 600", perm)
	}
}

func TestEmptySettingsPathIsRefused(t *testing.T) {
	if _, err := Acquire("", "default"); err == nil {
		t.Fatal("Acquire() with no settings file succeeded")
	}
	if _, _, err := Probe("", "default"); err == nil {
		t.Fatal("Probe() with no settings file succeeded")
	}
}
