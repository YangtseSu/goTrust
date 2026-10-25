// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ui draws the goTrust windows.
//
// The views are plain functions of the app state: they read State and write
// only through the callbacks the app provides. Nothing here touches the
// network, so the whole interface can be rendered headlessly in a test.
package ui

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
)

// State is what the main window and the tray show.
type State struct {
	Session session.State
	// Config is the settings in effect. The main window shows the active
	// profile; editing happens in the settings window.
	Config config.Config
	// Addresses are the bound proxy listeners.
	Addresses []string
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
}

// Actions are what the window and the tray can ask the application to do.
type Actions struct {
	Login      func()
	Logout     func()
	Settings   func()
	ShowWindow func()
	Quit       func()
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
	// CopyResource puts an entry's copy value on the clipboard.
	OpenResource func(string)
	CopyResource func(string)
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
		return fmt.Sprintf("配置 %s ｜ 尚未填写网关", name)
	}
	return fmt.Sprintf("配置 %s ｜ %s:%d", name, profile.ServerAddress(), profile.Server.Port)
}

// phaseLabel is the short word next to the title.
func phaseLabel(s session.State) string {
	switch s.Phase {
	case session.PhaseOnline:
		return "已连接"
	case session.PhaseConnecting:
		return "连接中"
	case session.PhaseReconnecting:
		return "重连中"
	case session.PhaseFailed:
		return "失败"
	default:
		return "未连接"
	}
}

// phaseDetail explains the phase in one line, including how long it has held.
func phaseDetail(s session.State) string {
	if s.Phase == session.PhaseReconnecting {
		if s.Reconnect.Attempt == 0 {
			return s.Detail + "…"
		}
		return reconnectLabel(s.Reconnect)
	}
	if s.Since.IsZero() {
		return s.Detail
	}
	held := time.Since(s.Since).Round(time.Second)
	switch s.Phase {
	case session.PhaseOnline:
		if s.ClientIP != "" {
			return fmt.Sprintf("%s ｜ 内网地址 %s ｜ 已保持 %s", s.Detail, s.ClientIP, held)
		}
		return fmt.Sprintf("%s ｜ 已保持 %s", s.Detail, held)
	case session.PhaseConnecting:
		return fmt.Sprintf("%s…", s.Detail)
	default:
		return s.Detail
	}
}

// reconnectLabel is the ladder position the state and the tray both show:
// "重连中 (2/5，4 秒后)" while it waits, "重连中 (2/5)" while an attempt runs.
func reconnectLabel(reconnect session.ReconnectState) string {
	label := fmt.Sprintf("重连中 (%d/%d", reconnect.Attempt, reconnect.Max)
	if wait := reconnectWait(reconnect.NextAt); wait != "" {
		label += "，" + wait
	}
	return label + ")"
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
	return fmt.Sprintf("%d 秒后", seconds)
}

// KeepAliveSummary renders the last probe for the window and the tray tooltip.
// An empty string means no probe has run yet.
func KeepAliveSummary(keepAlive session.KeepAliveState) string {
	if keepAlive.At.IsZero() {
		return ""
	}
	if keepAlive.Fallback {
		if keepAlive.OK {
			return "DNS OK · " + keepAlive.Target
		}
		return "DNS 失败 · " + keepAlive.Target
	}
	if keepAlive.OK {
		return fmt.Sprintf("OK %d · %s", keepAlive.Status, keepAlive.Target)
	}
	if keepAlive.Err != "" {
		return "失败 · " + keepAlive.Target
	}
	return fmt.Sprintf("失败 %d · %s", keepAlive.Status, keepAlive.Target)
}

// captchaPanel replaces the connection controls while the gateway waits for a
// click captcha. The image is drawn at a known size so a click maps back to the
// pixels the gateway expects, and the picked points are numbered in order.
func captchaPanel(c *ui.Context, state *State, actions Actions) {
	captcha := state.Captcha
	width, height := captcha.DisplaySize()

	ui.Column(c).Gap(sectionGap).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			panelHeading(c, "点击图形验证码")
			caption(c, "请按图中提示的顺序点击字符，然后提交。")
		})

		if captcha.Image == nil || width <= 0 || height <= 0 {
			captionTone(c, "验证码图片无法显示，请取消后重试。", toneDanger)
		} else {
			ui.Box(c).Width(width).Height(height).Children(func() {
				image := ui.Image(c, captcha.Image).Width(width).Height(height).Label("图形验证码图片")
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
			caption(c, fmt.Sprintf("已点击 %d 个位置", len(captcha.Points)))
			ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
				if ui.Button(c, "撤销").Disabled(len(captcha.Points) == 0).Clicked() && actions.UndoCaptcha != nil {
					actions.UndoCaptcha()
				}
				if ui.Button(c, "清空").Disabled(len(captcha.Points) == 0).Clicked() && actions.ClearCaptcha != nil {
					actions.ClearCaptcha()
				}
				if ui.PrimaryButton(c, "提交").Disabled(len(captcha.Points) == 0).Clicked() && actions.SubmitCaptcha != nil {
					actions.SubmitCaptcha()
				}
				if ui.Button(c, "取消").Clicked() && actions.CancelCaptcha != nil {
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
				if ui.Button(c, "断开").Clicked() && actions.Logout != nil {
					actions.Logout()
				}
			} else if ui.PrimaryButton(c, "连接").Disabled(state.Busy).Clicked() && actions.Login != nil {
				actions.Login()
			}
			if ui.Button(c, "设置").Clicked() && actions.Settings != nil {
				actions.Settings()
			}
			if ui.Button(c, "退出").Clicked() && actions.Quit != nil {
				actions.Quit()
			}
			if state.Busy {
				busyLine(c)
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
			sectionHeading(c, "已获授权的资源")
			caption(c, fmt.Sprintf("%d 项", len(granted.Entries)))
		})

		if len(granted.Entries) == 0 {
			switch state.Session.Phase {
			case session.PhaseOnline:
				caption(c, "网关没有为该账号授予资源")
			case session.PhaseConnecting:
				caption(c, "登录后显示网关授予的资源")
			default:
				caption(c, "连接后显示网关授予的资源")
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
			if ui.Button(c, "打开").Description("打开 "+entry.Title).Clicked() && actions.OpenResource != nil {
				actions.OpenResource(url)
			}
		}
		value := entry.Copy
		if ui.Button(c, "复制").Description("复制 "+entry.Title).Clicked() && actions.CopyResource != nil {
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
			caption(c, "代理端口尚未开启")
		} else {
			for _, address := range state.Addresses {
				caption(c, "代理监听 "+address)
			}
		}
		if summary := KeepAliveSummary(state.Session.KeepAlive); summary != "" {
			captionTone(c, "保活 "+summary, keepAliveTone(state.Session.KeepAlive))
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
			return "已连接"
		}
		return "已连接 " + target
	case session.PhaseConnecting:
		return "连接中"
	case session.PhaseReconnecting:
		if s.Reconnect.Attempt > 0 {
			return reconnectLabel(s.Reconnect)
		}
		return "重连中"
	case session.PhaseFailed:
		return "失败: " + truncate(s.Detail, 48)
	default:
		return "未连接"
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

	toggle := &mygo.MenuItem{ID: "toggle", Label: "连接"}
	if phaseActive(state.Session.Phase) {
		toggle.Label = "断开"
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
	profiles := &mygo.MenuItem{ID: "profiles", Label: "配置", Submenu: profileItems}
	if len(profileItems) == 0 {
		profiles.Disabled = true
	}

	show := &mygo.MenuItem{ID: "show", Label: "显示窗口", Click: func(*mygo.MenuItem, *mygo.Window) {
		if actions.ShowWindow != nil {
			actions.ShowWindow()
		}
	}}

	// 打开默认应用 opens the first portal application that has a launch
	// address with the desktop's default browser. The main window lists the
	// same URL on its resource row; the tray item is the shortcut for a user
	// who does not want to open the window first.
	openApp := &mygo.MenuItem{ID: "open-app", Label: "打开默认应用"}
	if url := state.Session.Resources.DefaultAppURL(); url != "" && actions.OpenResource != nil {
		openApp.Click = func(*mygo.MenuItem, *mygo.Window) {
			actions.OpenResource(url)
		}
	} else {
		openApp.Disabled = true
	}

	settingsItem := &mygo.MenuItem{ID: "settings", Label: "设置…", Click: func(*mygo.MenuItem, *mygo.Window) {
		if actions.Settings != nil {
			actions.Settings()
		}
	}}
	quit := &mygo.MenuItem{ID: "quit", Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) {
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
		settingsItem,
		mygo.Separator(),
		quit,
	})
}
