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
	case PageAbout:
		return locales.AboutSection()
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
		proxyPage(c, state, actions)
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
	case PageAbout:
		aboutPage(c, actions.Settings)
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
		card(c).Gap(rowGap).Children(func() {
			sectionHeading(c, locales.ConnectSection())
			connectionSection(c, settings, actions.Settings, profile, index)
		})
		card(c).Gap(rowGap).Children(func() {
			sectionHeading(c, locales.SecuritySection())
			securitySection(c, settings, actions, profile)
		})
	})
}

func proxyPage(c *ui.Context, state *State, actions Actions) {
	if state.Settings == nil {
		return
	}
	pageScroll(c, func() {
		windowHeading(c, pageLabel(PageProxy))
		card(c).Gap(rowGap).Children(func() {
			sectionHeading(c, locales.ProxyListeners())
			proxySection(c, state.Settings)
		})
		card(c).Gap(rowGap).Children(func() {
			proxyCopySection(c, state, state.Settings, actions)
		})
	})
}

// proxyCopySection is what a browser or a Clash client needs from the session:
// the PAC address and one of the three renderings of the live grant. The two
// actions were the dashboard's copy menu before the one-window shell.
func proxyCopySection(c *ui.Context, state *State, settings *SettingsState, actions Actions) {
	theme := c.Theme()
	ui.Column(c).Gap(theme.Space(2)).Children(func() {
		sectionHeading(c, locales.CopyConfig())
		ui.Divider(c)
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				ui.Text(c, locales.PacAddress()).FontSize(theme.FontSize)
				url := strings.TrimSpace(state.PACURL)
				if url == "" {
					caption(c, locales.PacOff())
				} else {
					ui.Text(c, url).Font("monospace").FontSize(textSize(c, captionScale)).
						TextColor(theme.TextMuted).SingleLine()
				}
			})
			url := strings.TrimSpace(state.PACURL)
			if ui.Button(c, locales.CommonCopy()).Label(locales.CopyPAC()).
				Disabled(url == "").Clicked() && actions.CopyPAC != nil {
				actions.CopyPAC()
			}
		})
		ui.Divider(c)
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			ui.Text(c, locales.ClashMihomo()).FontSize(theme.FontSize).Grow(1)
			options := []string{locales.ClashPlain(), locales.ClashScript(), locales.ClashConfig()}
			chosen := clashLabel(settings.ClashFormat)
			if ui.Select(c, &chosen, options).Width(methodPickerWidth).Changed() {
				settings.ClashFormat = clashFormat(chosen)
			}
			if ui.Button(c, locales.CommonCopy()).Label(locales.CopyClashDesc()).
				Clicked() && actions.CopyClash != nil {
				actions.CopyClash(settings.ClashFormat)
			}
		})
	})
}

// clashLabel is the picker's text for one rendering.
func clashLabel(format ClashFormat) string {
	switch format {
	case ClashScript:
		return locales.ClashScript()
	case ClashConfig:
		return locales.ClashConfig()
	default:
		return locales.ClashPlain()
	}
}

// clashFormat is the rendering the picker's text names; an unknown text keeps
// the plain fragment, the one the manual documents first.
func clashFormat(label string) ClashFormat {
	switch label {
	case locales.ClashScript():
		return ClashScript
	case locales.ClashConfig():
		return ClashConfig
	default:
		return ClashPlain
	}
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
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			windowHeading(c, pageLabel(PageTerminals)).Grow(1)
			if ui.Button(c, locales.CommonRefresh()).Description(locales.TerminalsRefreshDesc()).
				Disabled(!settings.Online || settings.TrustBusy).Clicked() &&
				actions.Settings.RefreshTerminals != nil {
				actions.Settings.RefreshTerminals()
			}
		})
		terminalSection(c, settings, actions.Settings)
	})
}

// LogTailLines is how many lines the window keeps from the log file: the gui's
// tailer and the page's footer share it.
const LogTailLines = 500

// logsPage is the live tail of the current log: the newest lines, narrowed by
// the filter, with 复制, 打开目录 and the 详细日志 switch that raises the
// running level. The lines carry no level of their own, so the page has no
// level filter; 详细日志 is what decides whether the diagnostic lines exist at
// all.
func logsPage(c *ui.Context, state *State, actions SettingsActions) {
	settings := state.Settings
	if settings == nil {
		return
	}
	lines := settings.visibleLogLines()
	theme := c.Theme()
	p := paletteOf(c)
	ui.Column(c).Grow(1).Shrink(1).Gap(sectionGap).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			windowHeading(c, pageLabel(PageLogs)).Grow(1)
			if switchLabel(c, &settings.Draft.Logging.Verbose, locales.VerboseLog()) &&
				actions.SetVerbose != nil {
				actions.SetVerbose(settings.Draft.Logging.Verbose)
			}
			if ui.Button(c, locales.ClearLogs()).Clicked() && actions.ClearLogs != nil {
				actions.ClearLogs()
			}
			if ui.Button(c, locales.OpenLogsDir()).Description(locales.OpenLogsDesc()).
				Disabled(settings.LogDir == "").Clicked() && actions.OpenLogs != nil {
				actions.OpenLogs()
			}
		})
		card(c).Grow(1).Shrink(1).Gap(rowGap).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
				ui.SearchField(c, &settings.LogFilter).Label(locales.LogFilter()).Grow(1)
				ui.Checkbox(c, &settings.LogAutoScroll, locales.LogAutoScroll())
				if ui.Button(c, locales.CommonCopy()).Label(locales.CopyLogsDesc()).
					Disabled(len(lines) == 0).Clicked() && actions.CopyLogs != nil {
					actions.CopyLogs(strings.Join(lines, "\n"))
				}
			})
			pane := ui.Scroll(c).Grow(1).Shrink(1).Padding(theme.Space(1.5)).
				Border(1, p.border).Radius(theme.Radius).Background(p.logBG)
			pane.Children(func() {
				ui.Column(c).Gap(0).Children(func() {
					if len(lines) == 0 {
						caption(c, locales.LogEmpty())
					}
					for _, line := range lines {
						ui.Text(c, line).Font("monospace").FontSize(textSize(c, captionScale)).
							SingleLine().TextColor(theme.TextMuted)
					}
					// The anchor follows the newest line only when they changed:
					// re-revealing it every frame would take the scrollbar away
					// from a user reading further up.
					anchor := ui.Box(c).Height(1)
					if settings.LogAutoScroll && settings.logSeen != len(lines) {
						anchor.ScrollIntoView()
					}
				})
			})
			settings.logSeen = len(lines)
			path := settings.logPath()
			if path == "" {
				path = locales.LogDirUnavailable()
			} else {
				path = shortPath(path)
			}
			caption(c, locales.LogTailFooter(path, LogTailLines))
		})
	})
}

// logPath is the file the tail follows, or the directory when the app runs
// without a file log.
func (s *SettingsState) logPath() string {
	if s.LogPath != "" {
		return s.LogPath
	}
	return s.LogDir
}

// visibleLogLines is the tail narrowed by the filter: a case-insensitive
// substring, as the resource filter is.
func (s *SettingsState) visibleLogLines() []string {
	filter := strings.ToLower(strings.TrimSpace(s.LogFilter))
	if filter == "" {
		return s.LogLines
	}
	lines := make([]string, 0, len(s.LogLines))
	for _, line := range s.LogLines {
		if strings.Contains(strings.ToLower(line), filter) {
			lines = append(lines, line)
		}
	}
	return lines
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
		languageSection(c, settings, actions.Settings)
		localDataSection(c, settings, profile, actions.Settings)
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
		card(c).Gap(rowGap).Children(func() {
			sectionHeading(c, locales.PageAccount())
			ui.Column(c).Gap(2).Role(ui.RoleRadioGroup).Children(func() {
				for index := range accounts {
					accountRow(c, settings, actions, index)
				}
			})
			formColumn(c, func() {
				formField(c, locales.NameField(), func() {
					ui.TextInput(c, &settings.NameText)
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
		})
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
	p := paletteOf(c)
	chosen := index == settings.Selected
	row := ui.ButtonBase(c).AlignItems(ui.Center).Padding(theme.Space(1.5), theme.Space(2)).
		Radius(theme.Radius).Gap(theme.Space(2)).Role(ui.RoleRadio).
		Label(profile.Name).Checked(chosen)
	if chosen {
		row.Background(p.accentWeak)
	} else if row.Hovered() {
		row.Background(p.hover)
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
