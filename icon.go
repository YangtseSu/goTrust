// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import _ "embed"

// The tray icon is rendered from the scalable icon the package installs, so
// the panel and the application menu show the same mark. Regenerate it after
// editing the SVG:
//
//	go generate ./...
//
//go:generate rsvg-convert -w 64 -h 64 data/icons/hicolor/scalable/apps/gotrust.svg -o data/icons/tray-64.png
//go:embed data/icons/tray-64.png
var iconPNG []byte

// trayIcon returns the PNG the tray shows, embedded so a packaged binary needs
// no resource directory beside it. 64x64 covers a 16pt icon at 4x, which keeps
// the shield's edges sharp on a HiDPI panel.
func trayIcon() []byte { return iconPNG }
