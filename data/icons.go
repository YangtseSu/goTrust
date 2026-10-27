// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package data carries the assets the binary embeds. The desktop entry and the
// scalable icons ship as files for the package to install; only the tray image
// has to live inside the executable, because the tray is created before any
// resource directory could be located.
package data

import _ "embed"

//go:generate rsvg-convert -w 64 -h 64 icons/hicolor/scalable/apps/gotrust.svg -o icons/tray-64.png
//go:embed icons/tray-64.png
var trayPNG []byte

// TrayPNG returns the tray image. It is rendered from the scalable icon the
// package installs, so the panel and the application menu show the same mark.
// 64x64 covers a 16pt icon at 4x, which keeps the shield's edges sharp on a
// HiDPI panel. Regenerate it with `go generate ./...` after editing the SVG.
func TrayPNG() []byte { return trayPNG }
