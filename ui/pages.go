// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/locales"
)

// pageLabel is the page's name, in the sidebar and at the top of the page.
func pageLabel(page Page) string {
	switch page {
	case PageAccount:
		return locales.PageAccount()
	case PageProxy:
		return locales.PageProxy()
	case PageKeepAlive:
		return locales.PageKeepAlive()
	case PageStartup:
		return locales.PageStartup()
	case PageTerminals:
		return locales.PageTerminals()
	case PageLogs:
		return locales.PageLogs()
	case PageSettings:
		return locales.PageSettings()
	case PageProfiles:
		return locales.PageProfiles()
	default:
		return locales.PageDashboard()
	}
}

// pageBody draws the selected page. The dashboard is also what a page this
// build does not know falls back to, so a newer front-end still shows the
// session instead of an empty column. The voluntary password change owns the
// page while it is open: it is a flow the user finishes or cancels.
func pageBody(c *ui.Context, state *State, actions Actions, searchFocus *uint64) {
	if settings := state.Settings; settings != nil && settings.Password != nil {
		pageScroll(c, func() {
			windowHeading(c, pageLabel(PageAccount))
			settingsPasswordPanel(c, settings, actions.Settings)
		})
		return
	}
	switch state.Page {
	case PageAccount:
		accountPage(c, state, actions)
	case PageProxy:
		proxyPage(c, state)
	case PageKeepAlive:
		keepAlivePage(c, state)
	case PageStartup:
		startupPage(c, state, actions)
	case PageTerminals:
		terminalsPage(c, state, actions)
	case PageLogs:
		logsPage(c, state, actions.Settings)
	case PageSettings:
		settingsPage(c, state, actions)
	case PageProfiles:
		profilesPage(c, state, actions)
	default:
		dashboardPage(c, state, actions, searchFocus)
	}
}

// pageScroll is the room a page of forms gets: one column that scrolls when
// the window is shorter than the form, so the shell's bars stay in place.
func pageScroll(c *ui.Context, children func()) {
	ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
		ui.Column(c).Gap(sectionGap).Children(children)
	})
}

// dashboardPage is the session page: the state, the resources the gateway
// granted, and the one line of listener and probe facts under them.
func dashboardPage(c *ui.Context, state *State, actions Actions, searchFocus *uint64) {
	ui.Column(c).Grow(1).Shrink(1).Gap(sectionGap).Children(func() {
		windowHeading(c, pageLabel(PageDashboard))
		header(c, state, actions)
		resources(c, state, actions, searchFocus)
		footer(c, state)
	})
}

// accountPage edits the account: the gateway, the login and the password.
func accountPage(c *ui.Context, state *State, actions Actions) {
	settings := state.Settings
	if settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageAccount))
		// A discovered list belongs to the gateway it was read from, and the
		// gateway is edited on this page. Drop it as soon as the address or
		// the port names something else, before the picker can store another
		// gateway's method.
		settings.dropStaleMethods()
		index, profile := settings.selection()
		if profile == nil {
			caption(c, locales.ErrNoProfiles())
			return
		}
		if active := settings.ActiveAtOpen; active != "" && active != profile.Name {
			captionTone(c, locales.SettingsSwitchHint(profile.Name), toneWarning)
		}
		connectionSection(c, settings, actions.Settings, profile, index)
	})
}

func proxyPage(c *ui.Context, state *State) {
	if state.Settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageProxy))
		proxySection(c, state.Settings)
	})
}

func keepAlivePage(c *ui.Context, state *State) {
	settings := state.Settings
	if settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageKeepAlive))
		index, profile := settings.selection()
		if profile == nil {
			caption(c, locales.ErrNoProfiles())
			return
		}
		enabled := profile.Keep.IsEnabled()
		keepAliveSection(c, settings, profile, &enabled, index)
	})
}

func startupPage(c *ui.Context, state *State, actions Actions) {
	settings := state.Settings
	if settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageStartup))
		startupSection(c, settings, actions.Settings)
	})
}

func terminalsPage(c *ui.Context, state *State, actions Actions) {
	settings := state.Settings
	if settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageTerminals))
		terminalSection(c, settings, actions.Settings)
	})
}

// logsPage owns the log location until the live tail moves in beside it: the
// directory and the button that opens it, not a second copy on 设置.
func logsPage(c *ui.Context, state *State, actions SettingsActions) {
	settings := state.Settings
	if settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageLogs))
		ui.Form(c, func() {
			ui.Field(c, locales.LogDir(), func() {
				ui.Row(c).Wrap().AlignItems(ui.Center).Gap(rowGap).Children(func() {
					directory := settings.LogDir
					if directory == "" {
						directory = locales.LogDirUnavailable()
					}
					ui.Text(c, directory).FontSize(c.Theme().FontSize).TextColor(c.Theme().TextMuted)
					if ui.Button(c, locales.CommonOpen()).Description(locales.OpenLogsDesc()).
						Disabled(settings.LogDir == "").Clicked() && actions.OpenLogs != nil {
						actions.OpenLogs()
					}
				})
			})
		})
	})
}

func settingsPage(c *ui.Context, state *State, actions Actions) {
	settings := state.Settings
	if settings == nil {
		return
	}
	_, profile := settings.selection()
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageSettings))
		if profile == nil {
			caption(c, locales.ErrNoProfiles())
			return
		}
		advancedSection(c, settings, profile, actions.Settings)
	})
}

// profilesPage keeps account management reachable while the rows and the
// rename move here: the draft's selection and the three list actions.
func profilesPage(c *ui.Context, state *State, actions Actions) {
	settings := state.Settings
	if settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageProfiles))
		ui.Form(c, func() {
			ui.Field(c, locales.CurrentProfile(), func() {
				ui.Row(c).Wrap().AlignItems(ui.Center).Gap(rowGap).Children(func() {
					names := settings.Draft.ProfileNames()
					if ui.Select(c, &settings.SelectText, names).Label(locales.CurrentProfile()).
						MaxWidth(profilePickerWidth).Changed() {
						if chosen := indexOf(names, settings.SelectText); chosen >= 0 && actions.Settings.SelectProfile != nil {
							actions.Settings.SelectProfile(chosen)
						}
					}
					if ui.Button(c, locales.ProfileNew()).Clicked() && actions.Settings.NewProfile != nil {
						actions.Settings.NewProfile()
					}
					if ui.Button(c, locales.ProfileDuplicate()).Clicked() && actions.Settings.DuplicateProfile != nil {
						actions.Settings.DuplicateProfile()
					}
					if ui.Button(c, locales.ProfileDelete()).Disabled(len(names) == 1).Clicked() &&
						actions.Settings.DeleteProfile != nil {
						actions.Settings.DeleteProfile()
					}
				})
			})
		})
		caption(c, locales.ProfilesHint())
	})
}
