// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package profilelock keeps one live process on a profile's login state.
//
// A profile owns a client data file and whatever tunnel identity the gateway
// handed it, and both front-ends reach a profile through the same core. Two
// live logins on one profile would fight over that file and over the stored
// session, so each front-end takes this lock before a login and releases it
// after the logout. Different profiles do not contend: the lock is keyed by the
// settings file and the profile, so a tray app on one account and a headless
// client on another can run side by side.
package profilelock

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrBusy reports that another live process holds the profile.
var ErrBusy = errors.New("the profile is in use by another process")

// BusyError names the profile that is taken and, when the holder wrote it, the
// holding process.
type BusyError struct {
	Profile string
	// PID is the holder's process id, 0 when the lock file did not name one.
	PID int
}

func (e *BusyError) Error() string {
	if e.PID > 0 {
		return fmt.Sprintf("profile %q is in use by process %d", e.Profile, e.PID)
	}
	return fmt.Sprintf("profile %q is in use by another process", e.Profile)
}

// Unwrap makes errors.Is(err, ErrBusy) report a taken profile.
func (e *BusyError) Unwrap() error { return ErrBusy }

// Lock is one held profile.
type Lock struct {
	file *os.File
	path string
}

// lockPath names the lock file for one profile of one settings file. It lives
// beside the settings, so --config keeps a scratch configuration away from the
// installed one, and the profile name is hashed because it is a user-chosen
// string that is not a legal file name on every platform.
func lockPath(configPath, profile string) string {
	sum := sha256.Sum256([]byte(profile))
	base := filepath.Base(configPath)
	return filepath.Join(filepath.Dir(configPath), fmt.Sprintf(".%s-%x.lock", base, sum[:8]))
}

// Acquire takes the lock for one profile. A profile another process holds is a
// *BusyError; every other error means the lock could not be created or taken,
// which the caller must treat as a failure rather than as a free profile.
func Acquire(configPath, profile string) (*Lock, error) {
	if strings.TrimSpace(configPath) == "" {
		return nil, errors.New("no settings file to lock")
	}
	path := lockPath(configPath, profile)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	taken, err := tryLock(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	if !taken {
		pid := readHolder(file)
		_ = file.Close()
		return nil, &BusyError{Profile: profile, PID: pid}
	}
	// The file carries the holder, so a refused instance can name the process
	// that already owns the profile.
	if err := writeHolder(file); err != nil {
		_ = unlock(file)
		_ = file.Close()
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	return &Lock{file: file, path: path}, nil
}

// Release gives the profile back. The file itself stays: unlinking it would let
// one process keep the old inode locked while another creates and locks a new
// one, which is two holders for one profile.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlock(l.file)
	if closeErr := l.file.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	l.file = nil
	if err != nil {
		return fmt.Errorf("unlock %s: %w", l.path, err)
	}
	return nil
}

// Probe reports whether a live process holds the profile, and that process when
// the lock file names it. It takes the lock and gives it back when the profile
// is free, so the answer is the one Acquire would give.
func Probe(configPath, profile string) (pid int, held bool, err error) {
	if strings.TrimSpace(configPath) == "" {
		return 0, false, errors.New("no settings file to probe")
	}
	path := lockPath(configPath, profile)
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	taken, err := tryLock(file)
	if err != nil {
		return 0, false, fmt.Errorf("lock %s: %w", path, err)
	}
	if !taken {
		return readHolder(file), true, nil
	}
	_ = unlock(file)
	return 0, false, nil
}

// readHolder reads the process id the holder wrote. A file that carries none is
// still a held profile, so only the id is lost.
func readHolder(file *os.File) int {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0
	}
	raw, err := io.ReadAll(io.LimitReader(file, 32))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

func writeHolder(file *os.File) error {
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := io.WriteString(file, strconv.Itoa(os.Getpid())+"\n")
	return err
}
