// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// newTail starts a tailer on a fresh directory and records what it emits.
func newTail(t *testing.T, limit int) (*logTail, func() []string, func() int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gotrust.log")
	var got []string
	emitted := 0
	tail := &logTail{path: path, limit: limit, emit: func(lines []string) {
		emitted++
		got = lines
	}}
	return tail, func() []string { return got }, func() int { return emitted }
}

func writeLog(t *testing.T, path, text string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open the log: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(text); err != nil {
		t.Fatalf("append to the log: %v", err)
	}
}

func TestLogTailReadsWhatWasAppended(t *testing.T) {
	tail, lines, emitted := newTail(t, 10)
	writeLog(t, tail.path, "one\ntwo\n")

	tail.poll()
	if want := []string{"one", "two"}; !slices.Equal(lines(), want) {
		t.Fatalf("lines = %q, want %q", lines(), want)
	}

	// A poll without an append must not redraw the page.
	before := emitted()
	tail.poll()
	if emitted() != before {
		t.Fatal("a poll with no new line emitted again")
	}

	writeLog(t, tail.path, "three\n")
	tail.poll()
	if want := []string{"one", "two", "three"}; !slices.Equal(lines(), want) {
		t.Fatalf("lines = %q, want %q", lines(), want)
	}
}

// A line that is still being written is held back until its newline, so the
// page never shows a truncated message.
func TestLogTailWaitsForACompleteLine(t *testing.T) {
	tail, lines, _ := newTail(t, 10)
	writeLog(t, tail.path, "par")

	tail.poll()
	if len(lines()) != 0 {
		t.Fatalf("a partial line was shown: %q", lines())
	}

	writeLog(t, tail.path, "tial\n")
	tail.poll()
	if want := []string{"partial"}; !slices.Equal(lines(), want) {
		t.Fatalf("lines = %q, want %q", lines(), want)
	}
}

// The logger rotates by renaming the file and opening a new one; the tailer
// must read the new file, not the stale offset of the old one.
func TestLogTailFollowsRotation(t *testing.T) {
	tail, lines, _ := newTail(t, 10)
	writeLog(t, tail.path, "old one\nold two\n")
	tail.poll()

	if err := os.Rename(tail.path, tail.path+".1"); err != nil {
		t.Fatalf("rotate the log: %v", err)
	}
	writeLog(t, tail.path, "fresh\n")
	tail.poll()

	if want := []string{"fresh"}; !slices.Equal(lines(), want) {
		t.Fatalf("lines = %q, want %q", lines(), want)
	}
}

// An emptied file starts over: the lines of the old file are not the lines of
// the new one.
func TestLogTailResetsOnTruncation(t *testing.T) {
	tail, lines, _ := newTail(t, 10)
	writeLog(t, tail.path, "first\nsecond\n")
	tail.poll()

	if err := os.Truncate(tail.path, 0); err != nil {
		t.Fatalf("truncate the log: %v", err)
	}
	writeLog(t, tail.path, "next\n")
	tail.poll()

	if want := []string{"next"}; !slices.Equal(lines(), want) {
		t.Fatalf("lines = %q, want %q", lines(), want)
	}
}

func TestLogTailKeepsTheLastLines(t *testing.T) {
	tail, lines, _ := newTail(t, 2)
	writeLog(t, tail.path, "a\nb\nc\n")
	tail.poll()

	if want := []string{"b", "c"}; !slices.Equal(lines(), want) {
		t.Fatalf("lines = %q, want %q", lines(), want)
	}
}

// The first read of a long log must not replay it: the window only ever shows
// the end.
func TestLogTailReadsOnlyTheEndOfALongFile(t *testing.T) {
	tail, lines, _ := newTail(t, 2)
	long := make([]byte, logTailWindow+1024)
	for index := range long {
		if index%64 == 63 {
			long[index] = '\n'
		} else {
			long[index] = 'x'
		}
	}
	suffix := "\nlast one\nlast\n"
	copy(long[len(long)-len(suffix):], suffix)
	if err := os.WriteFile(tail.path, long, 0o600); err != nil {
		t.Fatalf("write the log: %v", err)
	}
	tail.poll()

	if got := lines(); len(got) != 2 || got[0] != "last one" || got[1] != "last" {
		t.Fatalf("lines = %q, want the last two", got)
	}
}
