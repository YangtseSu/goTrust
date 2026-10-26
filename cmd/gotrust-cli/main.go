// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command gotrust-cli logs in to an aTrust gateway, serves the SOCKS5 and HTTP
// listeners and keeps the session alive, without a window. It reads the same
// settings file as the tray app and imports core packages only.
package main

import (
	"os"

	"github.com/YangtseSu/goTrust/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
