// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import atrustlog "github.com/YangtseSu/goTrust/core/atrust/log"

// SetProtocolDebug turns the protocol layer's verbose lines on or off. They go
// through the standard library's logger, which both front-ends already point at
// their own writer, so this switch is the whole bridge: a front-end never has
// to import the protocol layer to control it.
func SetProtocolDebug(on bool) {
	if on {
		atrustlog.EnableDebug()
		return
	}
	atrustlog.DisableDebug()
}
