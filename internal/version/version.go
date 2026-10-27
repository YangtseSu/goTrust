// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package version carries the release a binary was built from. A source
// checkout reports "devel"; the release workflow overrides the value at link
// time:
//
//	go build -ldflags "-X github.com/YangtseSu/goTrust/internal/version.Version=0.1.0" ./cmd/gotrust
package version

// Version is the release tag without its leading "v", or "devel" when the
// binary was built from a checkout rather than a release.
var Version = "devel"
