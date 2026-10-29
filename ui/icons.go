// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import "github.com/egoist/mygo/ui"

// The resource kinds have five glyphs: four for the kinds the protocol layer
// knows and one for a kind a later build may add. They are drawn here, as the
// paths that say what they mean, rather than pulled from an icon package: five
// shapes are smaller than a dependency, and every one of them inherits the
// colour of the text beside it.
//
// The weight matches mygo's own icons (a 24 grid, strokes of 2, round joins),
// so a glyph and a widget's magnifier look drawn by one hand.
var (
	// iconResourceApp is the portal's application list: four tiles.
	iconResourceApp = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.5"/></svg>`))

	// iconResourceDomain is a domain rule: a globe, because the name resolves
	// somewhere outside this network.
	iconResourceDomain = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3c2.1 2.5 3.2 5.5 3.2 9S14.1 18.5 12 21c-2.1-2.5-3.2-5.5-3.2-9S9.9 5.5 12 3Z"/></svg>`))

	// iconResourceAddress is an address rule: two hosts, side by side.
	iconResourceAddress = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="6.5" rx="2"/><rect x="3" y="13.5" width="18" height="6.5" rx="2"/><path d="M7 7.25h.01M7 16.75h.01"/></svg>`))

	// iconResourceDNS is a granted name: the name itself, and what it answers.
	iconResourceDNS = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 4.5A2.5 2.5 0 0 1 7.5 2H20v20H7.5A2.5 2.5 0 0 1 5 19.5Z"/><path d="M9.5 7.5H16M9.5 11.5h4.5"/></svg>`))

	// iconResourceOther marks a kind this build does not draw a row for yet.
	iconResourceOther = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5h.01"/></svg>`))

	// iconRowMenu is a row's own actions. It is drawn as three dots instead of
	// the arrowed menu button the header uses, because every row carries one
	// and a list of buttons outshouts the rows.
	iconRowMenu = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="19" cy="12" r="2"/></svg>`))
)

// The sidebar's pages have one glyph each, in the same weight as the rows
// above: they say what the page is for rather than spelling it out.
var (
	iconPageDashboard = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 15a8 8 0 0 1 16 0"/><path d="M12 15 15.5 10"/><circle cx="12" cy="15" r="1.6"/></svg>`))
	iconPageAccount   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="3.2"/><path d="M5 20c.8-3.4 3.5-5.2 7-5.2s6.2 1.8 7 5.2"/></svg>`))
	iconPageProxy     = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="8"/><path d="M4 12h16M12 4c2.2 2.2 3.4 4.9 3.4 8s-1.2 5.8-3.4 8c-2.2-2.2-3.4-4.9-3.4-8S9.8 6.2 12 4Z"/></svg>`))
	iconPageKeepAlive = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12h4l2.5-6 5 12 2.5-6h4"/></svg>`))
	iconPageStartup   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v9"/><path d="M7 6.5a7 7 0 1 0 10 0"/></svg>`))
	iconPageTerminals = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4.5" width="13" height="10" rx="1.6"/><path d="M3 18h13"/><rect x="18" y="10" width="3.5" height="7" rx="1"/></svg>`))
	iconPageLogs      = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3.5h8l4 4V20a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4.5a1 1 0 0 1 1-1Z"/><path d="M14 3.5V8h4M8 12h7M8 15h7M8 18h4"/></svg>`))
	iconPageSettings  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M18.7 5.3l-2.1 2.1M7.4 16.6l-2.1 2.1"/></svg>`))
	iconPageProfiles  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="9" cy="8" r="3"/><path d="M3 19c.7-3.1 3-4.8 6-4.8s5.3 1.7 6 4.8"/><path d="M16 5.2a3 3 0 0 1 0 5.6"/><path d="M18 19c-.2-1.4-.6-2.6-1.3-3.6"/></svg>`))
)

// pageIcon is the sidebar glyph of a page.
func pageIcon(page Page) *ui.SVG {
	switch page {
	case PageAccount:
		return iconPageAccount
	case PageProxy:
		return iconPageProxy
	case PageKeepAlive:
		return iconPageKeepAlive
	case PageStartup:
		return iconPageStartup
	case PageTerminals:
		return iconPageTerminals
	case PageLogs:
		return iconPageLogs
	case PageSettings:
		return iconPageSettings
	case PageProfiles:
		return iconPageProfiles
	default:
		return iconPageDashboard
	}
}
