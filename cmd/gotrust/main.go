// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command gotrust is the tray front-end. All it does is name the settings file
// and hand control to internal/gui, so the package stays importable and
// testable without a main.
package main

import (
	"flag"
	"log"

	"github.com/YangtseSu/goTrust/internal/gui"
)

func main() {
	configPath := flag.String("config", "", "TOML settings file (default: the user config directory); the stored account material lives beside it")
	flag.Parse()

	if err := gui.Run(*configPath); err != nil {
		log.Fatalf("goTrust: %v", err)
	}
}
