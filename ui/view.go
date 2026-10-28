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
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/proxy"
	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
)

// State is what the main window and the tray show.
type State struct {
	Session session.State
	// Config is the settings in effect. The main window shows the active
	// profile; editing happens in the settings window.
	Config config.Config
	// Listeners are the bound proxy endpoints.
	Listeners []proxy.Listener
	// PACURL is the loopback PAC a browser or extension can use, empty while
	// the endpoint is disabled.
	PACURL string
	// Status is a one-line message about the last user action. The window shows
	// it once, as a toast; StatusSeq increases with every new message, so the
	// same text twice is reported twice. StatusError reports a failure.
	Status      string
	StatusSeq   uint64
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
	// OpenLogs opens the log directory in the desktop's file manager.
	OpenLogs func()
}

// View renders one frame of the main window.
func View(state *State, actions Actions) func(c *ui.Context) {
	return func(c *ui.Context) {
		root := ui.Column(c).Fill()
		statusToast(c, root, state)
		root.Children(func() {
			titleBar(c, activeProfileName(state.Config))
			ui.Column(c).Grow(1).Shrink(1).Padding(windowPadding).Gap(sectionGap).Children(func() {
				header(c, state, actions)
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
					resources(c, state, actions)
				}
				footer(c, state)
			})
		})
	}
}

// TitleBarHeight is the strip a window with a hidden frame draws: the room its
// desktop window controls sit in. Both windows declare it, so a view drawn in a
// tester, where no frame reports one, still leaves the same room.
const TitleBarHeight = 34

// titleBar is the window's own title bar: the brand, at caption size and
// muted, inside the room the desktop's window controls leave. The strip drags
// the window and holds no control, so a press anywhere on it moves the window.
func titleBar(c *ui.Context, profile string) {
	bar := c.TitleBar()
	height := bar.Height
	if height <= 0 {
		// Without a hidden frame there is no room to keep clear of; the strip
		// still keeps the two windows aligned.
		height = TitleBarHeight
	}
	ui.Row(c).Height(height).AlignItems(ui.Center).
		Padding(0, bar.Right+windowPadding/2, 0, bar.Left+windowPadding/2).
		DragWindow().
		Children(func() {
			brand := "goTrust"
			if profile != "" {
				brand += " · " + profile
			}
			ui.Text(c, brand).FontSize(textSize(c, captionScale)).TextColor(c.Theme().TextMuted)
		})
}

// activeProfileName is the account the window is showing, as the title bar
// names it: the strip is the one place that stays true across every panel.
func activeProfileName(cfg config.Config) string {
	name := cfg.ActiveProfile
	if name == "" {
		name = config.DefaultProfileName
	}
	return locales.ProfileName(name)
}

// statusToast reports the last action's result once. The window never keeps it
// on screen: a result is something that happened, not a permanent line.
func statusToast(c *ui.Context, root ui.Element, state *State) {
	if state.Status == "" {
		return
	}
	shown := ui.Local[uint64](root, "status-seq", func() uint64 { return 0 })
	if shown == nil || *shown == state.StatusSeq {
		return
	}
	*shown = state.StatusSeq
	c.AddToast(ui.Toast{ID: "status", Title: state.Status})
}

// header is the state and what it acts on: the phase as the window's headline,
// its actions on the same line, and one line of session facts under them.
func header(c *ui.Context, state *State, actions Actions) {
	ui.Column(c).Gap(6).Children(func() {
		// Wrap: at the declared minimum width the three actions do not fit
		// beside the headline, and a clipped ⋯ would hide settings and quit.
		ui.Row(c).Wrap().AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(rowGap).Children(func() {
			statusChip(c, state)
			controls(c, state, actions)
		})
		if state.Setup == nil {
			// The first-run panel says what is missing itself; repeating it
			// above the form would spend the height the form needs.
			subline(c, state)
		}
	})
}

// subline is the one line under the state: where it connects, what the gateway
// assigned, and how long it has held. A failure or the reconnect ladder
// replaces the timing, in the tone that reports it.
func subline(c *ui.Context, state *State) {
	text, t := sessionSummary(state, c.Now())
	if t == toneNeutral {
		caption(c, text)
		return
	}
	captionTone(c, text, t)
}

// sessionSummary builds the subline: the endpoint first, then the fact that
// makes the phase readable on its own.
func sessionSummary(state *State, now time.Time) (string, tone) {
	profile := state.Config.Active()
	if profile.ServerAddress() == "" {
		// The title bar already names the profile; the subline says what is
		// missing from it.
		return locales.ProfileUnconfigured(), toneNeutral
	}
	parts := []string{locales.SessionEndpoint(profile.ServerAddress(), profile.Server.Port)}
	live := state.Session
	switch live.Phase {
	case session.PhaseFailed:
		if live.Detail != "" {
			parts = append(parts, live.Detail)
		}
		return strings.Join(parts, " · "), toneDanger
	case session.PhaseReconnecting:
		return strings.Join(append(parts, reconnectLabel(live.Reconnect)), " · "), toneWarning
	case session.PhaseOnline:
		if live.ClientIP != "" {
			parts = append(parts, locales.SessionIntranet(live.ClientIP))
		}
		if !live.Since.IsZero() {
			minutes := int(now.Sub(live.Since).Round(time.Minute) / time.Minute)
			if minutes < 1 {
				minutes = 1
			}
			parts = append(parts, locales.SessionHeldMinutes(minutes))
		}
	case session.PhaseConnecting:
		if live.Detail != "" {
			parts = append(parts, live.Detail)
		}
	}
	return strings.Join(parts, " · "), toneNeutral
}

// phaseLabel is the short word of the phase.
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
							ui.Text(c, strconv.Itoa(i+1)).FontSize(textSize(c, captionScale)).TextColor(c.Theme().AccentText).Bold()
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

// controls are the window's actions, next to the state: one primary action,
// the copy menu, and an overflow for everything that is not part of the
// session. Every action has exactly one control here.
func controls(c *ui.Context, state *State, actions Actions) {
	ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
		switch {
		case state.Setup != nil:
			// The first-run panel owns 连接 while it is up: its own primary
			// saves the form and signs in, so a second copy here would be a
			// second way to do the same thing with different failures.
		case phaseActive(state.Session.Phase):
			// 断开 stays available while a login runs: it is how a login
			// waiting on the network or on a prompt is cancelled.
			if ui.Button(c, locales.Disconnect()).Clicked() && actions.Logout != nil {
				actions.Logout()
			}
		case ui.PrimaryButton(c, locales.Connect()).Disabled(state.Busy).Clicked() && actions.Login != nil:
			actions.Login()
		}
		if state.Busy {
			busyLine(c)
		}
		ui.MenuButton(c, locales.CopyConfig(), func(menu *ui.Menu) {
			if menu.Item(locales.CopyPAC()).Disabled(state.PACURL == "").Chosen() && actions.CopyPAC != nil {
				actions.CopyPAC()
			}
			menu.Separator()
			for _, format := range []struct {
				label  string
				format ClashFormat
			}{
				{locales.ClashPlain(), ClashPlain},
				{locales.ClashScript(), ClashScript},
				{locales.ClashConfig(), ClashConfig},
			} {
				if menu.Item(format.label).Chosen() && actions.CopyClash != nil {
					actions.CopyClash(format.format)
				}
			}
		})
		ui.MenuButton(c, "⋯", func(menu *ui.Menu) {
			if menu.Item(locales.SettingsAction()).Chosen() && actions.Settings != nil {
				actions.Settings()
			}
			if menu.Item(locales.OpenLogsMenu()).Disabled(actions.OpenLogs == nil).Chosen() && actions.OpenLogs != nil {
				actions.OpenLogs()
			}
			menu.Separator()
			if menu.Item(locales.Quit()).Chosen() && actions.Quit != nil {
				actions.Quit()
			}
		}).Description(locales.MoreActions())
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

	ui.Column(c).Grow(1).Shrink(1).Gap(rowGap).Children(func() {
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

		ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
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

// footer is one quiet line: which listeners are up, and how long ago the
// session was last proved alive. The PAC URL and the probe's own result sit in
// the tooltip, so the line stays a line.
func footer(c *ui.Context, state *State) {
	line, detail, t := footerContent(state, c.Now())
	ui.Divider(c)
	text := ui.Text(c, line).FontSize(textSize(c, captionScale)).MaxLines(2)
	if t == toneNeutral {
		text.TextColor(c.Theme().TextMuted)
	} else {
		text.TextColor(toneInk(c.Theme(), t))
	}
	if len(detail) == 0 {
		return
	}
	text.Description(strings.Join(detail, " · "))
	footerTip(c, text, detail)
}

// footerContent builds the footer line and what the tooltip adds to it. Only a
// warning or a failure colours the line: a healthy probe is not news.
func footerContent(state *State, now time.Time) (string, []string, tone) {
	parts := make([]string, 0, len(state.Listeners)+1)
	for _, listener := range state.Listeners {
		parts = append(parts, listener.Protocol+" "+listener.Address)
	}
	if len(state.Listeners) == 0 {
		parts = append(parts, locales.ProxyDisabled())
	}

	detail := make([]string, 0, 2)
	if state.PACURL != "" {
		detail = append(detail, locales.PacURL(state.PACURL))
	}
	t := toneNeutral
	if keepAlive := state.Session.KeepAlive; !keepAlive.At.IsZero() {
		seconds := int(now.Sub(keepAlive.At) / time.Second)
		if seconds < 0 {
			seconds = 0
		}
		parts = append(parts, locales.KeepAliveAgo(seconds))
		switch probe := keepAliveTone(keepAlive); probe {
		case toneWarning, toneDanger:
			t = probe
		}
		if summary := KeepAliveSummary(keepAlive); summary != "" {
			detail = append(detail, locales.KeepAliveLine(summary))
		}
	}
	return strings.Join(parts, " · "), detail, t
}

// footerTip shows the footer's details above the line, one per row, in the
// inverse look the theme gives a tooltip.
func footerTip(c *ui.Context, anchor ui.Element, lines []string) {
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tip.Padding(6, 10).Radius(6).MaxWidth(360).
			Background(c.Theme().Text).TextColor(c.Theme().Background).
			FontSize(textSize(c, captionScale)).
			AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, 6, 0)
		tip.Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for _, line := range lines {
					ui.Text(c, line)
				}
			})
		})
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
