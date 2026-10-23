// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactRemovesCredentials(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		leak string
	}{
		{
			name: "json password and session",
			in:   `{"username":"alice","password":"hunter2","sid":"abc123","ticket":"t-9"}`,
			want: `"username":"alice"`,
			leak: "hunter2",
		},
		{
			name: "session fields",
			in:   `{"sidTicket":"st-1","sid-legacy":"sl-2","sign_key":"k-3","straceid":"tr-4"}`,
			want: `{"sidTicket":"***"`,
			leak: "sl-2",
		},
		{
			name: "cookies and tokens",
			in:   `Set-Cookie: sid=xyz; token=abc; csrfToken=def`,
			want: `Set-Cookie: "***"`,
			leak: "xyz",
		},
		{
			name: "query string",
			in:   `GET /login?password=hunter2&code=123456&keep=1`,
			want: `GET /login?password=***&code=***&keep=1`,
			leak: "hunter2",
		},
		{
			name: "authorization header",
			in:   `Authorization: Bearer secret-token value`,
			want: `Authorization: ***`,
			leak: "secret-token",
		},
		{
			name: "captcha coordinates",
			in:   `authchallenge: {"coordinates":[[1,2],[3,4]],"width":80}`,
			want: `"coordinates":"***"`,
			leak: "[[1,2]",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Redact(testCase.in)
			if strings.Contains(got, testCase.leak) {
				t.Fatalf("redacted line still holds %q: %s", testCase.leak, got)
			}
			if !strings.Contains(got, testCase.want) {
				t.Fatalf("redacted line = %s, want it to contain %s", got, testCase.want)
			}
		})
	}
}

func TestRedactLeavesOrdinaryLines(t *testing.T) {
	line := "KeepAlive: OK status=200 target=http://192.0.2.10/"
	if got := Redact(line); got != line {
		t.Fatalf("ordinary line changed:\n got %s\nwant %s", got, line)
	}
	if got := Redact("passwordless login is not a secret"); got != "passwordless login is not a secret" {
		t.Fatalf("a word containing a key was redacted: %s", got)
	}
}

func TestLoggerRedactsBeforeWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goTrust.log")
	logger, err := New(Options{Path: path})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer func() { _ = logger.Close() }()

	logger.Printf("login failed: %s", `{"password":"hunter2","sid":"abc"}`)
	logger.Printf("keep-alive ok")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, secret := range []string{"hunter2", `"abc"`} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("log file holds %q:\n%s", secret, raw)
		}
	}
	if !strings.Contains(string(raw), "keep-alive ok") {
		t.Fatalf("ordinary line missing:\n%s", raw)
	}
}

func TestRotationKeepsBoundedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goTrust.log")
	logger, err := New(Options{Path: path, MaxBytes: 400, Backups: 2})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	for index := range 30 {
		logger.Printf("line %02d aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", index)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	for _, name := range []string{path, path + ".1", path + ".2"} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		if info.Size() > 400 {
			t.Fatalf("%s is %d bytes, over the limit", name, info.Size())
		}
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatalf("a third backup exists with Backups=2: %v", err)
	}
	latest, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read latest: %v", err)
	}
	if !strings.Contains(string(latest), "line 29") {
		t.Fatalf("latest file does not hold the last line:\n%s", latest)
	}
}

func TestLoggerCreatesPrivateFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	t.Setenv("XDG_STATE_HOME", dir)
	path, err := Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(dir, "goTrust")) {
		t.Fatalf("path = %q, want it under XDG_STATE_HOME", path)
	}

	logger, err := New(Options{})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	logger.Printf("hello")
	if err := logger.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("log mode = %o, want 600", mode)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("log directory mode = %o, want 700", mode)
	}
}

func TestCloseFlushesPartialLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goTrust.log")
	logger, err := New(Options{Path: path})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := logger.Write([]byte("no newline")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "no newline") {
		t.Fatalf("partial line lost: %q", raw)
	}
}
