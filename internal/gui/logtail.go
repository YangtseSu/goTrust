// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"time"
)

const (
	// logPollInterval is how often the tailer looks for new lines: a second is
	// invisible to a reader and costs one stat on a file that grows slowly.
	logPollInterval = time.Second
	// logTailWindow bounds the first read of a log that is already long: only
	// its end can reach the window, so the rest is not read at all.
	logTailWindow = 256 << 10
)

// logTail follows one log file for the 日志 page. It reads only what was
// appended since the last poll, so a long-running session costs one stat and
// one read per second. The file is reopened when it is replaced or truncated:
// the logger rotates through a rename, and an operator may empty the file.
//
// emit always receives a copy of the retained lines, so the window can render
// them while the tailer moves on.
type logTail struct {
	path  string
	limit int
	emit  func([]string)

	offset int64
	info   os.FileInfo
	lines  []string
}

// run polls until ctx ends.
func (t *logTail) run(ctx context.Context) {
	t.poll()
	ticker := time.NewTicker(logPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.poll()
		}
	}
}

// poll reads the lines written since the previous call and hands the last
// limit of them to emit. A partial line waits for its newline, so a line that
// is still being written is never shown half-finished.
func (t *logTail) poll() {
	info, err := os.Stat(t.path)
	if err != nil {
		// A rotation removes the old name for an instant; the next poll finds
		// the new file and starts over.
		t.info = nil
		return
	}
	if t.info == nil || !os.SameFile(t.info, info) || info.Size() < t.offset {
		// The first read, a rotation, or a truncation: the old text belongs to
		// a file the page no longer shows.
		if t.lines != nil {
			t.lines = nil
			t.emitLines()
		}
		t.offset = 0
	}
	t.info = info
	if info.Size() <= t.offset {
		return
	}

	file, err := os.Open(t.path)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	start, cut := t.offset, false
	if start == 0 && info.Size() > logTailWindow {
		// Only the end of a long log can reach the page; the first line of
		// the window is probably partial and is dropped below.
		start = info.Size() - logTailWindow
		cut = true
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		// Nothing complete yet; read it again when the line ends.
		return
	}
	t.offset = start + int64(end) + 1

	chunk := strings.Split(string(data[:end]), "\n")
	if cut {
		chunk = chunk[1:]
	}
	for _, line := range chunk {
		t.lines = append(t.lines, strings.TrimSuffix(line, "\r"))
	}
	if t.limit > 0 && len(t.lines) > t.limit {
		t.lines = t.lines[len(t.lines)-t.limit:]
	}
	t.emitLines()
}

// emitLines hands out a copy: the caller keeps it after the tailer appends.
func (t *logTail) emitLines() {
	if t.emit == nil {
		return
	}
	lines := make([]string, len(t.lines))
	copy(lines, t.lines)
	t.emit(lines)
}
