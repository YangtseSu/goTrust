// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens a URL with the desktop's default browser. It is the
// default people expect from a front-end with a display; a host where that
// cannot work passes no Open function at all and reads the printed URL
// instead.
func OpenBrowser(ctx context.Context, url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "open", url)
	case "linux":
		command = exec.CommandContext(ctx, "xdg-open", url)
	case "windows":
		// rundll32 takes the URL as its own argument, so no shell parses it.
		command = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("no browser launcher on %s", runtime.GOOS)
	}
	if err := command.Start(); err != nil {
		return err
	}
	// Reap the launcher without holding the login goroutine: xdg-open can stay
	// alive for as long as the browser it started.
	go func() { _ = command.Wait() }()
	return nil
}
