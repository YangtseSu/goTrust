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
	"sort"
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

// Page is the page the window shows. It is the window's whole navigation
// state: the pages share the title bar, the sidebar and the save bar.
type Page uint8

const (
	PageDashboard Page = iota
	PageAccount
	PageProxy
	PageKeepAlive
	PageStartup
	PageTerminals
	PageLogs
	PageSettings
	PageProfiles
)

// State is what the window and the tray show.
type State struct {
	Session session.State
	// Page is the page the window shows. The zero value is the dashboard.
	Page Page
	// Config is the settings in effect, as the active profile and the tray
	// see them.
	Config config.Config
	// Settings is the draft the settings pages edit. The application creates
	// one at startup and keeps it for the life of the window, so a page
	// switch or hiding the window does not discard the edits.
	Settings *SettingsState
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
	// Login starts a login from the window: a draft that differs from the
	// settings file is saved first, so a gateway just typed is the one used.
	Login func()
	// TrayLogin starts the saved configuration instead, because a tray click
	// must not save edits the window is still making.
	TrayLogin func()
	Logout    func()
	// ShowPage selects the page the window shows, then shows and focuses the
	// window. ShowWindow shows the window without changing the page.
	ShowPage   func(Page)
	ShowWindow func()
	Quit       func()
	// Settings are the callbacks the settings pages use on the draft.
	Settings SettingsActions
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

// View renders one frame of the window: the title bar, the sidebar, the
// selected page, and the status and save bars, which belong to the window
// rather than to any page.
func View(state *State, actions Actions) func(c *ui.Context) {
	return func(c *ui.Context) {
		if state.Settings != nil {
			// The profile buffers follow the draft, whichever page changed the
			// list inside the last frame.
			state.Settings.refresh()
		}
		root := ui.Column(c).Fill()
		statusToast(c, root, "status", state.Status, state.StatusSeq, state.StatusError)
		if state.Settings != nil {
			statusToast(c, root, "settings-status", state.Settings.Status, state.Settings.StatusSeq, state.Settings.StatusError)
		}
		searchFocus := ui.Local(root, "search-focus", func() uint64 { return 0 })
		windowShortcuts(c, state, actions, searchFocus)
		root.Children(func() {
			titleBar(c, activeProfileName(state.Config))
			// A Row centers its children vertically by default; the sidebar and
			// the content have to fill the window instead.
			ui.Row(c).Grow(1).Shrink(1).AlignItems(ui.Stretch).Children(func() {
				sidebar(c, state, actions)
				content(c, state, actions, searchFocus)
			})
		})
	}
}

// The shell's fixed sizes. The window is 900 DIPs wide by default, so the
// sidebar leaves one comfortable column; the page cap keeps a maximized
// window from stretching a two-field form across the display.
const (
	sidebarWidth  float32 = 208
	navItemHeight float32 = 36
	navIconSize   float32 = 18
	pageMaxWidth  float32 = 680
)

// sidebar is the window's navigation: the dashboard first, the pages that
// edit the account and the app under it, and the account manager at the
// bottom, where it does not read as one step in that list. The items are tabs
// for assistive technology, and a panel that owns the flow disables them.
func sidebar(c *ui.Context, state *State, actions Actions) {
	theme := c.Theme()
	disabled := panelOpen(state)
	// The row centers its children vertically by default; the column and the
	// divider beside it have to fill the window's height.
	ui.Row(c).Shrink(0).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(sidebarWidth).Shrink(0).
			Padding(theme.Space(3), theme.Space(2.5), theme.Space(2.5), theme.Space(2.5)).
			Gap(theme.Space(0.5)).Background(theme.Surface).Role(ui.RoleTabList).
			Children(func() {
				navItem(c, state, actions, PageDashboard, disabled)
				navSeparator(c)
				navItem(c, state, actions, PageAccount, disabled)
				navItem(c, state, actions, PageProxy, disabled)
				navItem(c, state, actions, PageKeepAlive, disabled)
				navItem(c, state, actions, PageStartup, disabled)
				navItem(c, state, actions, PageTerminals, disabled)
				navItem(c, state, actions, PageLogs, disabled)
				navItem(c, state, actions, PageSettings, disabled)
				ui.Box(c).Grow(1)
				navSeparator(c)
				navItemTagged(c, state, actions, PageProfiles, disabled, activeAccountName(state.Config))
			})
		ui.Divider(c)
	})
}

// navItem is one page's row: its glyph and its name, tinted with the accent
// while the page is on screen.
func navItem(c *ui.Context, state *State, actions Actions, page Page, disabled bool) {
	navItemTagged(c, state, actions, page, disabled, "")
}

// navItemTagged is navItem with a short tag at its end, which the account
// manager uses for the live account.
func navItemTagged(c *ui.Context, state *State, actions Actions, page Page, disabled bool, tag string) {
	theme := c.Theme()
	label := pageLabel(page)
	item := ui.ButtonBase(c).Height(navItemHeight).AlignItems(ui.Center).
		Padding(0, theme.Space(2.5)).Radius(theme.Radius).Gap(theme.Space(2.5)).
		Role(ui.RoleTab).Label(label).Disabled(disabled)

	chosen := page == state.Page
	ink := theme.TextMuted
	if chosen {
		ink = theme.Accent
		item.Background(theme.Accent.Alpha(0.12).Over(theme.Surface))
	} else if item.Hovered() {
		item.Background(theme.SurfaceHover)
	}
	if disabled {
		// A disabled button is still drawn; the panel owns the flow, so the
		// navigation says so instead of looking one click away.
		ink = theme.TextMuted.Alpha(0.5)
	}
	item.Checked(chosen)

	item.Children(func() {
		ui.Icon(c, pageIcon(page)).Size(navIconSize, navIconSize).Shrink(0).TextColor(ink)
		text := ui.Text(c, label).FontSize(theme.FontSize - 1).TextColor(ink).SingleLine().Grow(1)
		if chosen {
			text.FontWeight(600)
		}
		if tag != "" {
			ui.Box(c).Padding(0, theme.Space(1.5)).Radius(999).Border(1, theme.Border).Shrink(0).Children(func() {
				ui.Text(c, tag).FontSize(textSize(c, captionScale)).TextColor(theme.TextMuted).SingleLine()
			})
		}
	})

	show := func() {
		if actions.ShowPage != nil {
			actions.ShowPage(page)
		}
	}
	if item.Clicked() {
		show()
	}
	item.OnShortcut(0, ui.KeyEnter, show).OnShortcut(0, ui.KeySpace, show)
}

// navSeparator divides the navigation's groups without a heading.
func navSeparator(c *ui.Context) {
	margin := c.Theme().Space(2)
	ui.Divider(c).Margin(margin, margin, margin, margin)
}

// content is the room right of the sidebar: the page, the panel that owns the
// flow, and the save bar while the draft differs from the file.
func content(c *ui.Context, state *State, actions Actions, searchFocus *uint64) {
	ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Padding(windowPadding).Gap(sectionGap).Children(func() {
		if panelOpen(state) {
			panelBody(c, state, actions)
			return
		}
		ui.Column(c).Grow(1).Shrink(1).MaxWidth(pageMaxWidth).Children(func() {
			pageBody(c, state, actions, searchFocus)
		})
		if settings := state.Settings; settings != nil && settings.Changed() {
			saveBar(c, settings, actions.Settings)
		}
	})
}

// panelBody is the pre-login question that owns the flow, wherever the
// sidebar points. The session's own line stays above it, so a question never
// hides the phase it belongs to; the selected page returns when the panel
// closes.
func panelBody(c *ui.Context, state *State, actions Actions) {
	ui.Column(c).Grow(1).Shrink(1).Gap(sectionGap).Children(func() {
		header(c, state, actions)
		switch {
		case state.Choice != nil:
			panelSlot(c, state, func() { choicePanel(c, state, actions) })
		case state.Captcha != nil:
			panelSlot(c, state, func() { captchaPanel(c, state, actions) })
		case state.Code != nil:
			panelSlot(c, state, func() { codePanel(c, state, actions) })
		case state.External != nil:
			panelSlot(c, state, func() { externalPanel(c, state, actions) })
		case state.DeviceBind != nil:
			panelSlot(c, state, func() { deviceBindPanel(c, state, actions) })
		case state.Password != nil:
			panelSlot(c, state, func() { passwordPanel(c, state, actions) })
		}
	})
}

// saveBar is the window's bottom bar: it appears while the draft differs from
// the settings file, so 保存 and 放弃 are reachable from every page.
func saveBar(c *ui.Context, settings *SettingsState, actions SettingsActions) {
	ui.Divider(c)
	ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
		captionTone(c, locales.SettingsUnsaved(), toneWarning)
		ui.Box(c).Grow(1)
		if ui.Button(c, locales.SettingsDiscard()).Clicked() && actions.Discard != nil {
			actions.Discard()
		}
		if ui.PrimaryButton(c, locales.CommonSave()).Clicked() && actions.Save != nil {
			actions.Save()
		}
		if settings.Busy {
			busyLine(c)
		}
	})
}

// TitleBarHeight is the strip a window with a hidden frame draws: the room its
// desktop window controls sit in. The window declares it, so a view drawn in a
// tester, where no frame reports one, still leaves the same room.
const TitleBarHeight = 34

// windowShortcuts are the keys the window answers wherever the focus is: the
// state's own action (Ctrl+Enter, or Cmd on a platform that has one), the way
// out of a login question (Esc) and the resource filter (/). A panel owns the
// flow while it is up, so Ctrl+Enter waits instead of ending a login the user
// is answering; Esc is what cancels.
func windowShortcuts(c *ui.Context, state *State, actions Actions, searchFocus *uint64) {
	switch {
	case panelOpen(state):
		// The panel's own foot has 取消登录; Esc does the same.
		if cancel := panelCancel(state, actions); cancel != nil {
			c.OnShortcut(0, ui.KeyEscape, cancel)
		}
	case phaseActive(state.Session.Phase):
		if actions.Logout != nil {
			c.OnShortcut(ui.Cmd, ui.KeyEnter, actions.Logout)
		}
	case !state.Busy && actions.Login != nil:
		c.OnShortcut(ui.Cmd, ui.KeyEnter, actions.Login)
	}
	if searchFocus != nil && *searchFocus != 0 && !resourceSearchShown(state) {
		// The filter left with the list; a request kept for it would focus the
		// next list the user never asked to search.
		*searchFocus = 0
	}
	if searchFocus == nil || *searchFocus != 0 || !resourceSearchShown(state) {
		return
	}
	// While the field has the focus, / belongs to the text: the shortcut
	// exists to reach the filter, not to stop it from taking a slash.
	if ui.FocusedValue(c, searchFocus) != 0 {
		return
	}
	c.OnShortcut(0, ui.KeySlash, func() { *searchFocus = 1 })
}

// panelOpen reports whether a pre-login panel owns the window body.
func panelOpen(state *State) bool {
	return state.Choice != nil || state.Captcha != nil || state.Code != nil ||
		state.External != nil || state.DeviceBind != nil || state.Password != nil
}

// panelCancel is the way out of the panel on screen, in the order View shows
// them. nil means no login question is up, so Esc has nothing to cancel.
func panelCancel(state *State, actions Actions) func() {
	switch {
	case state.Choice != nil:
		return actions.CancelChoice
	case state.Captcha != nil:
		return actions.CancelCaptcha
	case state.Code != nil:
		return actions.CancelCode
	case state.External != nil:
		return actions.CancelExternal
	case state.DeviceBind != nil:
		return actions.CancelDeviceBind
	case state.Password != nil:
		return actions.CancelPassword
	default:
		return nil
	}
}

// resourceSearchShown reports whether the body is the resource list and that
// list has a filter. A panel replaces the list, a short list needs none, and
// the list only exists on the dashboard.
func resourceSearchShown(state *State) bool {
	return state.Page == PageDashboard && !panelOpen(state) &&
		resourceSearchNeeded(state.Session.Resources.Entries)
}

// panelMaxWidth bounds a pre-login form. At the window's own width it changes
// nothing; a maximized window must not stretch three fields across 2000 DIPs,
// where a label and its value stop reading as one row.
const panelMaxWidth float32 = 560

// panelSlot composes a login panel with the one line the resource list leaves
// behind: the panel takes the room the question needs, and the count of what
// the gateway already granted stays on screen under it. A question hides the
// list, not the fact that the account holds resources.
func panelSlot(c *ui.Context, state *State, panel func()) {
	ui.Column(c).Grow(1).Shrink(1).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MaxWidth(panelMaxWidth).Children(panel)
		grantedLine(c, state)
	})
}

// grantedLine is the count of granted resources, as one muted line: it is the
// only part of the list that survives a panel, and an account with none gets no
// line at all.
func grantedLine(c *ui.Context, state *State) {
	count := len(state.Session.Resources.Entries)
	if count == 0 {
		return
	}
	caption(c, locales.ResourcesGranted(count))
}

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
	return locales.ProfileName(activeAccountName(cfg))
}

// activeAccountName is the live account's own name, as the sidebar's tag and
// the tray use it: the name is user data and is not translated.
func activeAccountName(cfg config.Config) string {
	name := cfg.ActiveProfile
	if name == "" {
		name = config.DefaultProfileName
	}
	return name
}

// statusToast reports the last action's result once. The window never keeps it
// on screen: a result is something that happened, not a permanent line. id
// separates the results of the window from those of the settings draft, each
// with its own sequence.
func statusToast(c *ui.Context, root ui.Element, id, text string, seq uint64, failed bool) {
	if text == "" {
		return
	}
	shown := ui.Local[uint64](root, id+"-seq", func() uint64 { return 0 })
	if shown == nil || *shown == seq {
		return
	}
	*shown = seq
	toast := ui.Toast{ID: id, Title: text}
	if failed {
		toast.Type = "error"
	}
	c.AddToast(toast)
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
		subline(c, state)
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

// captchaMarkerSize is the diameter of a picked point's number. The web prompt
// page draws the same marker at 26px and numbers it at caption size, so a
// challenge answered in either surface marks the clicks the same way.
const captchaMarkerSize float32 = 26

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
					left := float32(point.X)*width/float32(captcha.Width) - captchaMarkerSize/2
					top := float32(point.Y)*height/float32(captcha.Height) - captchaMarkerSize/2
					ui.Box(c).Absolute().Left(left).Top(top).Width(captchaMarkerSize).Height(captchaMarkerSize).
						Radius(captchaMarkerSize / 2).Background(c.Theme().Danger).Center().PassThrough().
						Children(func() {
							ui.Text(c, strconv.Itoa(i+1)).FontSize(textSize(c, captionScale)).TextColor(c.Theme().AccentText).Bold()
						})
				}
			})
		}

		caption(c, locales.CaptchaPicked(len(captcha.Points)))
		panelActions(c, actions.CancelCaptcha, func() {
			if textButton(c, locales.CaptchaUndo()).Disabled(len(captcha.Points) == 0).Clicked() && actions.UndoCaptcha != nil {
				actions.UndoCaptcha()
			}
			if textButton(c, locales.CaptchaClear()).Disabled(len(captcha.Points) == 0).Clicked() && actions.ClearCaptcha != nil {
				actions.ClearCaptcha()
			}
			if ui.PrimaryButton(c, locales.CommonSubmit()).Disabled(len(captcha.Points) == 0).Clicked() && actions.SubmitCaptcha != nil {
				actions.SubmitCaptcha()
			}
		})
	})
}

// controls are the session's actions, next to the state: one primary action
// and the copy menu. Quit and the log directory belong to the tray and to the
// pages; the dashboard is the session surface.
func controls(c *ui.Context, state *State, actions Actions) {
	ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
		switch {
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

// Resource rows are one line each: a glyph for the kind, the title, the ports
// or the group beside it, and one menu that holds the actions. The list scrolls
// and keeps room for three rows even at the window's minimum, because an
// account can be granted far more entries than the window is tall and a list
// that collapses to its heading hides the thing the window is for.
const (
	resourceIconSize     = 18 // the kind glyph, and so the height of a row's line
	resourceRowPadY      = 6  // above and below a row's line
	resourceSearchAt     = 8  // rows in one group that still read without a filter
	resourceGroupSpacing = 12 // above a group's heading, except the first
	resourceHeadingSpace = 4  // between a group's heading and its rows
	resourceListMinRows  = 3
)

// resourceGroup is one kind's rows, in the order the window lists the kinds.
type resourceGroup struct {
	kind    session.ResourceKind
	label   string
	entries []session.ResourceEntry
}

// resourceKinds fixes the order of the groups and where a row of that kind
// belongs: the portal's applications first, then the names and addresses they
// are reached through.
var resourceKinds = []struct {
	kind  session.ResourceKind
	label func() string
	glyph *ui.SVG
}{
	{session.ResourceApp, locales.ResourceGroupApp, iconResourceApp},
	{session.ResourceDomain, locales.ResourceGroupDomain, iconResourceDomain},
	{session.ResourceIP, locales.ResourceGroupAddress, iconResourceAddress},
	{session.ResourceDNS, locales.ResourceGroupDNS, iconResourceDNS},
}

// resourceKindStyle is a kind's heading and glyph. A kind this build does not
// know still gets rows, under 其他, so a newer gateway adds data the window can
// show instead of data it drops.
func resourceKindStyle(kind session.ResourceKind) (label string, glyph *ui.SVG) {
	for _, style := range resourceKinds {
		if style.kind == kind {
			return style.label(), style.glyph
		}
	}
	return locales.ResourceGroupOther(), iconResourceOther
}

// resourceKindRank orders the groups: the declared kinds in their own order,
// and any other kind after them.
func resourceKindRank(kind session.ResourceKind) int {
	for index, style := range resourceKinds {
		if style.kind == kind {
			return index
		}
	}
	return len(resourceKinds)
}

// groupResources sorts the granted rows into their kinds and applies the
// filter. A row matches on its title or its detail, the two things it shows, so
// what the user reads is what the search looks at. Empty groups disappear, so
// every heading counts rows that are on screen.
func groupResources(entries []session.ResourceEntry, query string) []resourceGroup {
	filter := strings.ToLower(strings.TrimSpace(query))
	groups := make([]resourceGroup, 0, len(resourceKinds)+1)
	at := make(map[session.ResourceKind]int, len(resourceKinds)+1)

	for _, entry := range entries {
		if filter != "" && !matchesResource(entry, filter) {
			continue
		}
		index, seen := at[entry.Kind]
		if !seen {
			label, _ := resourceKindStyle(entry.Kind)
			groups = append(groups, resourceGroup{kind: entry.Kind, label: label})
			index = len(groups) - 1
			at[entry.Kind] = index
		}
		groups[index].entries = append(groups[index].entries, entry)
	}

	// A stable sort keeps unknown kinds in the order they arrived, which is the
	// order the gateway listed them in.
	sort.SliceStable(groups, func(i, j int) bool {
		return resourceKindRank(groups[i].kind) < resourceKindRank(groups[j].kind)
	})
	return groups
}

// matchesResource reports whether a row answers a filter already in lower case.
func matchesResource(entry session.ResourceEntry, filter string) bool {
	return strings.Contains(strings.ToLower(entry.Title), filter) ||
		strings.Contains(strings.ToLower(entry.Detail), filter)
}

// resources lists what the gateway granted this account, one group per kind.
// searchFocus is the window's focus request for the filter, so / reaches it.
func resources(c *ui.Context, state *State, actions Actions, searchFocus *uint64) {
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

		var filter string
		if resourceSearchNeeded(granted.Entries) {
			filter = resourceSearch(c, searchFocus)
		}
		groups := groupResources(granted.Entries, filter)
		if len(groups) == 0 {
			caption(c, locales.ResourceSearchEmpty())
			return
		}

		ui.Scroll(c).Grow(1).Shrink(1).MinHeight(resourceListMinHeight(c)).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for index, group := range groups {
					resourceGroupHeading(c, group, index == 0)
					for _, entry := range group.entries {
						resourceRow(c, entry, actions)
					}
				}
			})
		})
	})
}

// resourceSearchNeeded reports whether one kind holds more rows than the list
// shows at once. A filter over a short list costs a line and a click and saves
// neither.
func resourceSearchNeeded(entries []session.ResourceEntry) bool {
	counts := make(map[session.ResourceKind]int, len(resourceKinds))
	for _, entry := range entries {
		if counts[entry.Kind]++; counts[entry.Kind] > resourceSearchAt {
			return true
		}
	}
	return false
}

// resourceSearch is the filter above the list. What the user types belongs to
// the field, not to the app state: no other part of the window reads it, and a
// new login can replace the rows without carrying a stale query. The field
// follows focus, which is how the window's / reaches it.
func resourceSearch(c *ui.Context, focus *uint64) string {
	row := ui.Row(c).AlignItems(ui.Center)
	query := ui.Local(row, "resource-query", func() string { return "" })
	row.Children(func() {
		field := ui.SearchField(c, query).Label(locales.ResourceSearch()).Grow(1)
		if focus != nil {
			field.FocusBind(focus, *focus)
		}
	})
	return *query
}

// resourceListMinHeight is the height of three rows at the theme's size: the
// list keeps that much of the window when the footer and the header have taken
// theirs.
func resourceListMinHeight(c *ui.Context) float32 {
	line := float32(resourceIconSize)
	if text := c.Theme().FontSize * 1.3; text > line {
		line = text
	}
	return resourceListMinRows * (line + 2*resourceRowPadY)
}

// resourceGroupHeading names one kind and counts the rows under it. The count
// is what the filter leaves, so a heading never claims rows that are hidden.
func resourceGroupHeading(c *ui.Context, group resourceGroup, first bool) {
	heading := ui.Row(c).AlignItems(ui.Center)
	if first {
		heading.Margin(0, 0, resourceHeadingSpace, 0)
	} else {
		heading.Margin(resourceGroupSpacing, 0, resourceHeadingSpace, 0)
	}
	heading.Children(func() {
		sectionHeading(c, locales.ResourceGroupCount(group.label, len(group.entries)))
	})
}

// resourceRow is one granted entry: the kind's glyph, the title, its detail at
// the right, and the menu with 打开 and 复制. The actions sit in the menu
// because two buttons on every row of a long list outshout the rows.
func resourceRow(c *ui.Context, entry session.ResourceEntry, actions Actions) {
	_, glyph := resourceKindStyle(entry.Kind)

	ui.Row(c).AlignItems(ui.Center).Gap(rowGap).PaddingY(resourceRowPadY).Children(func() {
		ui.Icon(c, glyph).Size(resourceIconSize, resourceIconSize).Shrink(0).TextColor(c.Theme().TextMuted)
		ui.Text(c, entry.Title).FontSize(c.Theme().FontSize).MaxLines(1).Grow(1)
		if entry.Detail != "" {
			caption(c, entry.Detail).MaxLines(1).Shrink(0)
		}
		menu := ui.Icon(c, iconRowMenu).Size(resourceIconSize, resourceIconSize).Shrink(0).
			TextColor(c.Theme().TextMuted).Padding(4).
			Label(locales.ResourceMenu(entry.Title))
		menu.Menu(func(menu *ui.Menu) {
			if menu.Item(locales.CommonOpen()).Disabled(entry.URL == "").Chosen() && actions.OpenResource != nil {
				actions.OpenResource(entry.URL)
			}
			if menu.Item(locales.CommonCopy()).Chosen() && actions.CopyResource != nil {
				actions.CopyResource(entry.Copy)
			}
		})
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
// connect toggle, the profile submenu, and the window commands. Copying a
// PAC or a Clash rendering belongs to the window's 复制配置 menu alone: one
// owner per action, one wording, and a tray menu that does not repeat the
// window's exports.
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
			// The tray logs in with the saved configuration: a click here must
			// not save the draft the window is still editing.
			if actions.TrayLogin != nil {
				actions.TrayLogin()
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
		mygo.Separator(),
		quit,
	})
}
