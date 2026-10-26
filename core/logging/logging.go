// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package logging writes the application's diagnostics to a rotating file.
//
// A tray application has no terminal: without a file, a failed login leaves
// nothing to inspect. The file holds whatever the aTrust client logs, which
// includes session material in its debug dumps, so every line is redacted
// before it reaches the disk.
package logging

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

// Defaults bound the log so a long-running app cannot fill the disk.
const (
	DefaultMaxBytes = 5 << 20
	DefaultBackups  = 3
	fileName        = "gotrust.log"
)

// Options configure New.
type Options struct {
	// Path overrides the default file location.
	Path string
	// MaxBytes is the size at which the file rotates. Zero means 5 MiB.
	MaxBytes int64
	// Backups is how many rotated files to keep. Zero means 3.
	Backups int
	// Mirror receives the same lines, for a headless run where stderr is the
	// terminal. Nil discards nothing else: the file is always written.
	Mirror io.Writer
	// Verbose starts with debug logging enabled.
	Verbose bool
}

// Logger is an io.Writer that splits its input into lines, redacts them,
// rotates the file and mirrors each line.
type Logger struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	max     int64
	backups int
	size    int64
	mirror  io.Writer
	pending []byte
	closed  bool

	verbose atomic.Bool
	logger  *log.Logger
}

// Dir is the directory holding the log, following the XDG state directory on
// Linux and the platform's log location elsewhere.
func Dir() (string, error) {
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "gotrust"), nil
	}
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Logs", "gotrust"), nil
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", fmt.Errorf("LOCALAPPDATA is not set")
		}
		return filepath.Join(base, "gotrust", "logs"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "state", "gotrust"), nil
	}
}

// Path is the default log file location.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// New opens the log file, creating its directory with owner-only permissions.
func New(options Options) (*Logger, error) {
	path := options.Path
	if path == "" {
		defaultPath, err := Path()
		if err != nil {
			return nil, err
		}
		path = defaultPath
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	backups := options.Backups
	if backups <= 0 {
		backups = DefaultBackups
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create the log directory: %w", err)
	}

	writer := &Logger{
		path:    path,
		max:     maxBytes,
		backups: backups,
		mirror:  options.Mirror,
	}
	writer.verbose.Store(options.Verbose)
	if err := writer.openLocked(); err != nil {
		return nil, err
	}
	writer.logger = log.New(writer, "", log.LstdFlags)
	return writer, nil
}

func (l *Logger) openLocked() error {
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", l.path, err)
	}
	// An older run or another tool may have created the file readable.
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("restrict %s: %w", l.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("stat %s: %w", l.path, err)
	}
	l.file = file
	l.size = info.Size()
	return nil
}

// Path is the current log file.
func (l *Logger) Path() string { return l.path }

// Dir is the directory holding the log and its rotations.
func (l *Logger) Dir() string { return filepath.Dir(l.path) }

// Printf writes one line through the standard log format.
func (l *Logger) Printf(format string, args ...any) {
	if l.logger != nil {
		l.logger.Printf(format, args...)
	}
}

// SetVerbose turns debug logging on or off.
func (l *Logger) SetVerbose(verbose bool) { l.verbose.Store(verbose) }

// Verbose reports whether debug logging is on.
func (l *Logger) Verbose() bool { return l.verbose.Load() }

// Write implements io.Writer. It buffers a partial line until its newline, so
// a secret split across writes is still redacted as a whole.
func (l *Logger) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return len(p), nil
	}

	l.pending = append(l.pending, p...)
	for {
		index := bytes.IndexByte(l.pending, '\n')
		if index < 0 {
			break
		}
		l.emitLocked(l.pending[:index+1])
		l.pending = l.pending[index+1:]
	}
	// A dump without newlines must not grow without bound.
	if int64(len(l.pending)) > l.max {
		l.emitLocked(l.pending)
		l.pending = nil
	}
	return len(p), nil
}

func (l *Logger) emitLocked(line []byte) {
	redacted := Redact(string(line))
	// A single huge line (a dump without newlines) must not fill the file. The
	// floor keeps a small MaxBytes from cutting every line, and it must stay
	// within the line: the old code sliced at 256 regardless and panicked on a
	// line shorter than that.
	limit := l.max / 2
	if limit < 256 {
		limit = 256
	}
	if int64(len(redacted)) > limit {
		redacted = redacted[:limit] + "… [truncated]\n"
	}
	raw := []byte(redacted)

	// A rotation that could not reopen the file (the directory was removed, the
	// rename failed, permissions changed) leaves file nil. Trying again here is
	// what keeps one failure from disabling file logging for the rest of the
	// process; the rotation check below no longer sees size 0 and skips it.
	if l.file == nil {
		_ = l.openLocked()
	}
	if l.file != nil && l.size > 0 && l.size+int64(len(raw)) > l.max {
		l.rotateLocked()
	}
	if l.file != nil {
		written, _ := l.file.Write(raw)
		l.size += int64(written)
	}
	if l.mirror != nil {
		_, _ = l.mirror.Write(raw)
	}
}

func (l *Logger) rotateLocked() {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	for index := l.backups - 1; index >= 1; index-- {
		_ = os.Rename(
			l.path+"."+strconv.Itoa(index),
			l.path+"."+strconv.Itoa(index+1),
		)
	}
	_ = os.Rename(l.path, l.path+".1")
	if err := l.openLocked(); err != nil {
		// Keep logging to the mirror rather than losing the line. emitLocked
		// retries the open on the next line, so a transient failure does not
		// silence the file for the rest of the process.
		l.file = nil
		l.size = 0
	}
}

// Close flushes a trailing partial line and closes the file. It is safe twice.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	if len(l.pending) > 0 {
		l.pending = append(l.pending, '\n')
		l.emitLocked(l.pending)
		l.pending = nil
	}
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

var (
	// credentialHeader masks the value of a credential-bearing header whole.
	// A cookie is a semicolon-separated family and an authorization value can
	// carry a scheme or a map/JSON wrapper, so masking one word left the rest
	// of the secret behind: "Cookie: theme=dark; session=…" kept the session,
	// and "map[Authorization:[Bearer …]]" kept the token. A bracketed or quoted
	// value is masked up to its close so the rest of the line survives; any
	// other value runs to the end of the line.
	credentialHeader = regexp.MustCompile(
		`(?i)("?\b(?:(?:proxy-)?authorization|set-cookie|cookie)\b"?\s*[:=]\s*)` +
			`(?:(?:bearer|basic)\s+)?(?:\[[^\]]*\]|"[^"]*"|'[^']*'|[^\r\n]+)`)

	// captchaData masks a click captcha payload on one line. The logged value
	// is the coordinates the user picked, which are login material.
	captchaData = regexp.MustCompile(`(?i)("coordinates"\s*:\s*)\[[^\n]*\]`)

	// secretField masks key/value pairs in JSON, query strings and log lines.
	// The alternation lists the longest names first so "sid" cannot hide
	// "sidTicket". Headers with a value list (Cookie, Authorization) are
	// handled by credentialHeader above.
	secretField = regexp.MustCompile(`(?i)("?\b(?:sid-legacy\.sig|sid-legacy|sidTicket|straceid\.sig|straceid|sign_key|signKey|access_token|refresh_token|csrfToken|csrf_token|password|passwd|captcha|ticket|token|pwd|sid)\b"?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;&}]+)`)

	// querySecret catches credentials that follow a query separator.
	querySecret = regexp.MustCompile(`(?i)([?&](?:password|passwd|pwd|token|ticket|sid|code|sign_key|signKey)=)[^&\s]+`)
)

// Redact masks passwords, cookies, session tokens and captcha data in one log
// line. It is a text filter, not a parser: a field it misses is a leaked
// credential, so the patterns are deliberately broad.
func Redact(line string) string {
	line = credentialHeader.ReplaceAllString(line, `$1***`)
	line = captchaData.ReplaceAllString(line, `$1"***"`)
	line = secretField.ReplaceAllString(line, `$1"***"`)
	line = querySecret.ReplaceAllString(line, `$1***`)
	return line
}
