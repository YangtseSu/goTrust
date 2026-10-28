// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ui draws the goTrust windows.
//
// The views are plain functions of the app state: they read State and write
// only through the callbacks the app provides. Nothing here touches the
// network, so the whole interface can be rendered headlessly in a test.
package ui

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
)

// State is what the main window and the tray show.
type State struct {
	Session session.State
	// Config is the settings in effect. The main window shows the active
	// profile; editing happens in the settings window.
	Config config.Config
	// Addresses are the bound proxy listeners.
	Addresses []string
	// PACURL is the loopback PAC a browser or extension can use, empty while
	// the endpoint is disabled.
	PACURL string
	// Status is a one-line message about the last user action. StatusError
	// paints it as a failure instead of a note.
	Status      string
	StatusError bool
	// Busy disables the controls that would race a login or a logout.
	Busy bool
	// Captcha is set while the gateway waits for a click captcha. It replaces
	// the connection controls until the user submits or cancels it.
	Captcha *Captcha
	// Code, External and Choice are the other pre-login questions. A login
	// asks one at a time, so at most one of the four is set.
	Code     *CodePrompt
	External *ExternalLogin
	Choice   *AuthChoice
	// DeviceBind is the gateway's device-binding request, shown before the
	// login continues.
	DeviceBind *DeviceBindPrompt
	// Password is the gateway's forced password change, shown in the same slot.
	Password *PasswordPrompt
	// Setup is the first-run form. It is set while the active profile has no
	// gateway or account yet, and replaces the connection controls and the
	// resource list until it does.
	Setup *SetupDraft
}

// ClashFormat selects one of the three Clash/Mihomo renderings. It mirrors the
// CLI flags so both front-ends expose the same outputs.
type ClashFormat string

const (
	ClashPlain  ClashFormat = "clash"
	ClashScript ClashFormat = "clash-script"
	ClashConfig ClashFormat = "clash-config"
)

// Actions are what the window and the tray can ask the application to do.
type Actions struct {
	Login      func()
	Logout     func()
	Settings   func()
	ShowWindow func()
	Quit       func()
	// SaveSetup writes the first-run form and, when connect is set, logs in
	// with it right away.
	SaveSetup func(draft SetupDraft, connect bool)
	// SelectProfile switches the active account. The application disconnects
	// first and reconnects when auto-connect is on.
	SelectProfile func(string)
	// The captcha callbacks drive the pending click captcha: one point in
	// image pixels at a time, an undo, a clear, the submit, and a cancel that
	// abandons the login.
	AddCaptchaPoint func(Point)
	UndoCaptcha     func()
	ClearCaptcha    func()
	SubmitCaptcha   func()
	CancelCaptcha   func()
	// The second-factor callbacks answer a code challenge (SMS, TOTP, RADIUS)
	// and an external SSO login.
	SubmitCode     func()
	CancelCode     func()
	OpenExternal   func()
	SubmitExternal func()
	CancelExternal func()
	// ChooseAuth picks one of the offered authentication methods by index.
	ChooseAuth   func(int)
	CancelChoice func()
	// AllowDeviceBind confirms the gateway's device-binding request; cancel
	// abandons the login.
	AllowDeviceBind  func()
	CancelDeviceBind func()
	// SubmitPassword answers the forced password change; CancelPassword
	// abandons the login.
	SubmitPassword func()
	CancelPassword func()
	// OpenResource opens a granted resource's suggested URL in the browser;
	// CopyResource puts an entry's copy value on the clipboard. CopyPAC copies
	// the PAC URL; CopyClash copies one of the three Clash renderings.
	OpenResource func(string)
	CopyResource func(string)
	CopyPAC      func()
	CopyClash    func(ClashFormat)
}

// View renders one frame of the main window.
func View(state *State, actions Actions) func(c *ui.Context) {
	return func(c *ui.Context) {
		ui.Column(c).Fill().Padding(windowPadding).Gap(sectionGap).Children(func() {
			header(c, state)
			switch {
			case state.Choice != nil:
				choicePanel(c, state, actions)
			case state.Captcha != nil:
				captchaPanel(c, state, actions)
			case state.Code != nil:
				codePanel(c, state, actions)
			case state.External != nil:
				externalPanel(c, state, actions)
			case state.DeviceBind != nil:
				deviceBindPanel(c, state, actions)
			case state.Password != nil:
				passwordPanel(c, state, actions)
			case state.Setup != nil:
				setupPanel(c, state, actions)
			default:
				controls(c, state, actions)
				resources(c, state, actions)
			}
			footer(c, state)
		})
	}
}

func header(c *ui.Context, state *State) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			windowHeading(c, "goTrust")
			statusChip(c, state)
		})
		caption(c, profileSummary(state.Config))
		if detail := phaseDetail(state.Session); detail != "" && detail != phaseLabel(state.Session) {
			detailLine(c, detail, phaseTone(state.Session.Phase))
		}
	})
}

// detailLine is the one-line explanation under the header: muted while
// nothing is wrong, and in the phase's colour when the session reports a
// warning or a failure, so the reason is not quieter than the label.
func detailLine(c *ui.Context, text string, t tone) {
	line := ui.Text(c, text).FontSize(c.Theme().FontSize)
	if t == toneWarning || t == toneDanger {
		line.TextColor(toneColor(c.Theme(), t))
	} else {
		line.TextColor(c.Theme().TextMuted)
	}
}

// profileSummary names the active account and where it connects.
func profileSummary(cfg config.Config) string {
	name := cfg.ActiveProfile
	if name == "" {
		name = config.DefaultProfileName
	}
	profile := cfg.Active()
	if profile.ServerAddress() == "" {
		return locales.ProfileUnconfigured(name)
	}
	return locales.ProfileTarget(name, profile.ServerAddress(), profile.Server.Port)
}

// phaseLabel is the short word next to the title.
func phaseLabel(s session.State) string {
	switch s.Phase {
	case session.PhaseOnline:
		return locales.StatusOnline()
	case session.PhaseConnecting:
		return locales.StatusConnecting()
	case session.PhaseReconnecting:
		return locales.StatusReconnecting()
	case session.PhaseFailed:
		return locales.StatusFailed()
	default:
		return locales.StatusOffline()
	}
}

// phaseDetail explains the phase in one line, including how long it has held.
func phaseDetail(s session.State) string {
	if s.Phase == session.PhaseReconnecting {
		if s.Reconnect.Attempt == 0 {
			return locales.SessionDetailConnecting(s.Detail)
		}
		return reconnectLabel(s.Reconnect)
	}
	if s.Since.IsZero() {
		return s.Detail
	}
	held := time.Since(s.Since).Round(time.Second).String()
	switch s.Phase {
	case session.PhaseOnline:
		if s.ClientIP != "" {
			return locales.SessionDetailClientIP(s.Detail, s.ClientIP, held)
		}
		return locales.SessionDetailHeld(s.Detail, held)
	case session.PhaseConnecting:
		return locales.SessionDetailConnecting(s.Detail)
	default:
		return s.Detail
	}
}

// reconnectLabel is the ladder position the state and the tray both show:
// "重连中 (2/5，4 秒后)" while it waits, "重连中 (2/5)" while an attempt runs.
func reconnectLabel(reconnect session.ReconnectState) string {
	wait := reconnectWait(reconnect.NextAt)
	if wait == "" {
		return locales.ReconnectProgress(reconnect.Attempt, reconnect.Max)
	}
	return locales.ReconnectProgressWait(reconnect.Attempt, reconnect.Max, wait)
}

// reconnectWait renders the time left before the next attempt, rounded up so a
// countdown never shows "0 秒后" while the attempt has not started.
func reconnectWait(next time.Time) string {
	if next.IsZero() {
		return ""
	}
	seconds := int(math.Ceil(time.Until(next).Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return locales.ReconnectWait(seconds)
}

// KeepAliveSummary renders the last probe for the window and the tray tooltip.
// An empty string means no probe has run yet.
func KeepAliveSummary(keepAlive session.KeepAliveState) string {
	if keepAlive.At.IsZero() {
		return ""
	}
	if keepAlive.Fallback {
		if keepAlive.OK {
			return locales.KeepAliveDNSOK(keepAlive.Target)
		}
		return locales.KeepAliveDNSFailed(keepAlive.Target)
	}
	if keepAlive.OK {
		return locales.KeepAliveOK(keepAlive.Status, keepAlive.Target)
	}
	if keepAlive.Err != "" {
		return locales.KeepAliveFailed(keepAlive.Target)
	}
	return locales.KeepAliveFailedStatus(keepAlive.Status, keepAlive.Target)
}

// captchaPanel replaces the connection controls while the gateway waits for a
// click captcha. The image is drawn at a known size so a click maps back to the
// pixels the gateway expects, and the picked points are numbered in order.
func captchaPanel(c *ui.Context, state *State, actions Actions) {
	captcha := state.Captcha
	width, height := captcha.DisplaySize()

	ui.Column(c).Gap(sectionGap).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			panelHeading(c, locales.CaptchaTitle())
			caption(c, locales.CaptchaHint())
		})

		if captcha.Image == nil || width <= 0 || height <= 0 {
			captionTone(c, locales.CaptchaMissing(), toneDanger)
		} else {
			ui.Box(c).Width(width).Height(height).Children(func() {
				image := ui.Image(c, captcha.Image).Width(width).Height(height).Label(locales.CaptchaImage())
				image.OnClick(func() {
					x, y, ok := image.PointerPosition()
					if !ok {
						return
					}
					point, ok := captcha.PointAt(x, y)
					if ok && actions.AddCaptchaPoint != nil {
						actions.AddCaptchaPoint(point)
					}
				})
				for i, point := range captcha.Points {
					left := float32(point.X)*width/float32(captcha.Width) - 9
					top := float32(point.Y)*height/float32(captcha.Height) - 9
					ui.Box(c).Absolute().Left(left).Top(top).Width(18).Height(18).
						Radius(9).Background(c.Theme().Danger).Center().PassThrough().
						Children(func() {
							ui.Text(c, strconv.Itoa(i+1)).FontSize(10).TextColor(c.Theme().AccentText).Bold()
						})
				}
			})
		}

		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			caption(c, locales.CaptchaPicked(len(captcha.Points)))
			ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
				if ui.Button(c, locales.CaptchaUndo()).Disabled(len(captcha.Points) == 0).Clicked() && actions.UndoCaptcha != nil {
					actions.UndoCaptcha()
				}
				if ui.Button(c, locales.CaptchaClear()).Disabled(len(captcha.Points) == 0).Clicked() && actions.ClearCaptcha != nil {
					actions.ClearCaptcha()
				}
				if ui.PrimaryButton(c, locales.CommonSubmit()).Disabled(len(captcha.Points) == 0).Clicked() && actions.SubmitCaptcha != nil {
					actions.SubmitCaptcha()
				}
				if ui.Button(c, locales.CommonCancel()).Clicked() && actions.CancelCaptcha != nil {
					actions.CancelCaptcha()
				}
			})
		})
	})
}

func controls(c *ui.Context, state *State, actions Actions) {
	ui.Column(c).Gap(rowGap).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			if phaseActive(state.Session.Phase) {
				// 断开 stays available while a login runs: it is how a login
				// waiting on the network or on a prompt is cancelled.
				if ui.Button(c, locales.Disconnect()).Clicked() && actions.Logout != nil {
					actions.Logout()
				}
			} else if ui.PrimaryButton(c, locales.Connect()).Disabled(state.Busy).Clicked() && actions.Login != nil {
				actions.Login()
			}
			if ui.Button(c, locales.SettingsAction()).Clicked() && actions.Settings != nil {
				actions.Settings()
			}
			if ui.Button(c, locales.CopyPAC()).Disabled(state.PACURL == "").Clicked() && actions.CopyPAC != nil {
				actions.CopyPAC()
			}
			if ui.Button(c, locales.Quit()).Clicked() && actions.Quit != nil {
				actions.Quit()
			}
			if state.Busy {
				busyLine(c)
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			if ui.Button(c, locales.ClashPlain()).Clicked() && actions.CopyClash != nil {
				actions.CopyClash(ClashPlain)
			}
			if ui.Button(c, locales.ClashScript()).Clicked() && actions.CopyClash != nil {
				actions.CopyClash(ClashScript)
			}
			if ui.Button(c, locales.ClashConfig()).Clicked() && actions.CopyClash != nil {
				actions.CopyClash(ClashConfig)
			}
		})
	})
}

// phaseActive reports whether the phase has a session that 断开 must end.
func phaseActive(phase session.Phase) bool {
	switch phase {
	case session.PhaseOnline, session.PhaseConnecting, session.PhaseReconnecting:
		return true
	default:
		return false
	}
}

// resources lists what the gateway granted this account: the portal's
// applications first, then the address rules they are reached through. The
// list scrolls, because an account can be granted far more entries than the
// window is tall.
func resources(c *ui.Context, state *State, actions Actions) {
	granted := state.Session.Resources

	ui.Column(c).Grow(1).Gap(rowGap).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			sectionHeading(c, locales.ResourcesTitle())
			caption(c, locales.ResourceCount(len(granted.Entries)))
		})

		if len(granted.Entries) == 0 {
			switch state.Session.Phase {
			case session.PhaseOnline:
				caption(c, locales.ResourcesEmptyOnline())
			case session.PhaseConnecting:
				caption(c, locales.ResourcesEmptyConnecting())
			default:
				caption(c, locales.ResourcesEmpty())
			}
			return
		}

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for index, entry := range granted.Entries {
					if index > 0 {
						ui.Divider(c)
					}
					resourceRow(c, entry, actions)
				}
			})
		})
	})
}

func resourceRow(c *ui.Context, entry session.ResourceEntry, actions Actions) {
	ui.Row(c).AlignItems(ui.Center).Gap(rowGap).PaddingY(6).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, entry.Title).FontSize(c.Theme().FontSize)
			if entry.Detail != "" {
				caption(c, entry.Detail)
			}
		})
		if entry.URL != "" {
			url := entry.URL
			if ui.Button(c, locales.CommonOpen()).Description(locales.ResourceOpen(entry.Title)).Clicked() && actions.OpenResource != nil {
				actions.OpenResource(url)
			}
		}
		value := entry.Copy
		if ui.Button(c, locales.CommonCopy()).Description(locales.ResourceCopy(entry.Title)).Clicked() && actions.CopyResource != nil {
			actions.CopyResource(value)
		}
	})
}

// keepAliveTone colours the last probe: a real HTTP answer is success, a
// failure is danger, and the DNS fallback is a warning because it does not
// hold the session open.
func keepAliveTone(keepAlive session.KeepAliveState) tone {
	switch {
	case keepAlive.At.IsZero():
		return toneNeutral
	case keepAlive.OK && keepAlive.Fallback:
		return toneWarning
	case keepAlive.OK:
		return toneSuccess
	default:
		return toneDanger
	}
}

func footer(c *ui.Context, state *State) {
	ui.Divider(c)
	ui.Column(c).Gap(4).Children(func() {
		if len(state.Addresses) == 0 {
			caption(c, locales.ProxyDisabled())
		} else {
			for _, address := range state.Addresses {
				caption(c, locales.ProxyListening(address))
			}
		}
		if state.PACURL != "" {
			caption(c, locales.PacURL(state.PACURL))
		}
		if summary := KeepAliveSummary(state.Session.KeepAlive); summary != "" {
			captionTone(c, locales.KeepAliveLine(summary), keepAliveTone(state.Session.KeepAlive))
		}
		if state.Status != "" {
			line := ui.Text(c, state.Status).FontSize(c.Theme().FontSize)
			if state.StatusError {
				line.TextColor(c.Theme().Danger)
			} else {
				line.TextColor(c.Theme().TextMuted)
			}
		}
	})
}

// TrayStatus is the disabled first line of the tray menu: the phase and, when
// connected, the address the gateway assigned.
func TrayStatus(s session.State) string {
	switch s.Phase {
	case session.PhaseOnline:
		target := s.ClientIP
		if target == "" {
			target = s.Detail
		}
		if target == "" {
			return locales.TrayStatusOnline()
		}
		return locales.TrayStatusOnlineTarget(target)
	case session.PhaseConnecting:
		return locales.TrayStatusConnecting()
	case session.PhaseReconnecting:
		if s.Reconnect.Attempt > 0 {
			return reconnectLabel(s.Reconnect)
		}
		return locales.TrayStatusReconnecting()
	case session.PhaseFailed:
		return locales.TrayStatusFailed(truncate(s.Detail, 48))
	default:
		return locales.TrayStatusOffline()
	}
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

// TrayMenu builds the tray menu for the current state: a status line, the
// connect toggle, the profile submenu, and the window commands.
func TrayMenu(state *State, actions Actions) *mygo.Menu {
	status := &mygo.MenuItem{
		ID:       "status",
		Label:    TrayStatus(state.Session),
		Disabled: true,
	}

	toggle := &mygo.MenuItem{ID: "toggle", Label: locales.Connect()}
	if phaseActive(state.Session.Phase) {
		toggle.Label = locales.Disconnect()
		toggle.Click = func(*mygo.MenuItem, *mygo.Window) {
			if actions.Logout != nil {
				actions.Logout()
			}
		}
	} else {
		// A login started while another action is still finishing would race
		// it; the window disables the same button.
		toggle.Disabled = state.Busy
		toggle.Click = func(*mygo.MenuItem, *mygo.Window) {
			if actions.Login != nil {
				actions.Login()
			}
		}
	}

	profileItems := make([]*mygo.MenuItem, 0, len(state.Config.Profiles))
	for _, profile := range state.Config.Profiles {
		name := profile.Name
		profileItems = append(profileItems, &mygo.MenuItem{
			ID:      "profile:" + name,
			Label:   name,
			Type:    mygo.MenuItemRadio,
			Checked: name == state.Config.ActiveProfile,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				if actions.SelectProfile != nil {
					actions.SelectProfile(name)
				}
			},
		})
	}
	profiles := &mygo.MenuItem{ID: "profiles", Label: locales.TrayProfiles(), Submenu: profileItems}
	if len(profileItems) == 0 {
		profiles.Disabled = true
	}

	show := &mygo.MenuItem{ID: "show", Label: locales.TrayShow(), Click: func(*mygo.MenuItem, *mygo.Window) {
		if actions.ShowWindow != nil {
			actions.ShowWindow()
		}
	}}

	// 打开默认应用 opens the first portal application that has a launch
	// address with the desktop's default browser. The main window lists the
	// same URL on its resource row; the tray item is the shortcut for a user
	// who does not want to open the window first.
	openApp := &mygo.MenuItem{ID: "open-app", Label: locales.TrayOpenApp()}
	if url := state.Session.Resources.DefaultAppURL(); url != "" && actions.OpenResource != nil {
		openApp.Click = func(*mygo.MenuItem, *mygo.Window) {
			actions.OpenResource(url)
		}
	} else {
		openApp.Disabled = true
	}

	copyPAC := &mygo.MenuItem{ID: "copy-pac", Label: locales.CopyPAC()}
	if state.PACURL != "" && actions.CopyPAC != nil {
		copyPAC.Click = func(*mygo.MenuItem, *mygo.Window) {
			actions.CopyPAC()
		}
	} else {
		copyPAC.Disabled = true
	}

	clashItems := []*mygo.MenuItem{
		{ID: "copy-clash", Label: locales.TrayClashPlain(), Click: func(*mygo.MenuItem, *mygo.Window) {
			if actions.CopyClash != nil {
				actions.CopyClash(ClashPlain)
			}
		}},
		{ID: "copy-clash-script", Label: locales.TrayClashScript(), Click: func(*mygo.MenuItem, *mygo.Window) {
			if actions.CopyClash != nil {
				actions.CopyClash(ClashScript)
			}
		}},
		{ID: "copy-clash-config", Label: locales.TrayClashConfig(), Click: func(*mygo.MenuItem, *mygo.Window) {
			if actions.CopyClash != nil {
				actions.CopyClash(ClashConfig)
			}
		}},
	}
	copyClash := &mygo.MenuItem{ID: "copy-clash-menu", Label: locales.TrayCopyClash(), Submenu: clashItems}
	if actions.CopyClash == nil {
		copyClash.Disabled = true
	}

	settingsItem := &mygo.MenuItem{ID: "settings", Label: locales.TraySettings(), Click: func(*mygo.MenuItem, *mygo.Window) {
		if actions.Settings != nil {
			actions.Settings()
		}
	}}
	quit := &mygo.MenuItem{ID: "quit", Label: locales.TrayQuit(), Click: func(*mygo.MenuItem, *mygo.Window) {
		if actions.Quit != nil {
			actions.Quit()
		}
	}}

	return mygo.NewMenu([]*mygo.MenuItem{
		status,
		mygo.Separator(),
		toggle,
		profiles,
		mygo.Separator(),
		show,
		openApp,
		copyPAC,
		copyClash,
		settingsItem,
		mygo.Separator(),
		quit,
	})
}
