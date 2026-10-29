// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"

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

// dashboardPage is the session page: the session card with its one control and
// its measurements, and the resources the gateway granted in a second card.
func dashboardPage(c *ui.Context, state *State, actions Actions, searchFocus *uint64) {
	ui.Column(c).Grow(1).Shrink(1).Gap(sectionGap).Children(func() {
		windowHeading(c, pageLabel(PageDashboard))
		statusCard(c, state, actions)
		resources(c, state, actions, searchFocus)
	})
}

// statusCard is the session on the dashboard: the phase and its facts on the
// left, the one control on the right, and the measurements under a rule. The
// session reads before the resources because the card comes first.
func statusCard(c *ui.Context, state *State, actions Actions) {
	card(c).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(sectionGap).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Children(func() { sessionLine(c, state) })
			powerControl(c, state, actions)
		})
		ui.Divider(c).Margin(c.Theme().Space(3), 0, c.Theme().Space(2.5), 0)
		metricsRow(c, state)
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

// profilesPage manages the accounts: one row per account for the draft's
// selection, the selected account's name, and the three list actions. A row is
// the picker: choosing one edits that account, and 保存 makes the choice the
// live account and moves the stored client data after a rename.
func profilesPage(c *ui.Context, state *State, actions Actions) {
	settings := state.Settings
	if settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageProfiles))
		settings.refresh()
		accounts := settings.Draft.ProfileNames()
		ui.Column(c).Gap(2).Role(ui.RoleRadioGroup).Children(func() {
			for index := range accounts {
				accountRow(c, settings, actions, index)
			}
		})
		ui.Form(c, func() {
			ui.Field(c, locales.NameField(), func() {
				ui.TextInput(c, &settings.NameText).MaxWidth(fieldWidth)
			})
		})
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(rowGap).Children(func() {
			if ui.Button(c, locales.ProfileNew()).Clicked() && actions.Settings.NewProfile != nil {
				actions.Settings.NewProfile()
			}
			if ui.Button(c, locales.ProfileDuplicate()).Clicked() && actions.Settings.DuplicateProfile != nil {
				actions.Settings.DuplicateProfile()
			}
			if ui.Button(c, locales.ProfileDelete()).Disabled(len(accounts) == 1).Clicked() &&
				actions.Settings.DeleteProfile != nil {
				actions.Settings.DeleteProfile()
			}
		})
		caption(c, locales.ProfilesHint())
	})
}

// accountRow is one account in the draft: its name, its gateway and whether the
// session is already on it. The tint marks the account the form edits, 当前
// marks the live one.
func accountRow(c *ui.Context, settings *SettingsState, actions Actions, index int) {
	theme := c.Theme()
	profile := settings.Draft.Profiles[index]
	gateway := strings.TrimSpace(settings.Gateways[index])
	if gateway == "" {
		gateway = locales.ProfileUnconfigured()
	}
	chosen := index == settings.Selected
	row := ui.ButtonBase(c).AlignItems(ui.Center).Padding(theme.Space(1.5), theme.Space(2)).
		Radius(theme.Radius).Gap(theme.Space(2)).Role(ui.RoleRadio).
		Label(profile.Name).Checked(chosen)
	if chosen {
		row.Background(theme.Accent.Alpha(0.12).Over(theme.Surface))
	} else if row.Hovered() {
		row.Background(theme.SurfaceHover)
	}
	row.Children(func() {
		ui.Avatar(c, profile.Name, nil)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			name := ui.Text(c, profile.Name).FontSize(theme.FontSize).SingleLine()
			if chosen {
				name.FontWeight(600)
			}
			caption(c, gateway).SingleLine()
		})
		if profile.Name == settings.ActiveAtOpen {
			caption(c, locales.ProfileCurrent())
		}
	})
	if row.Clicked() && actions.Settings.SelectProfile != nil {
		actions.Settings.SelectProfile(index)
	}
}
