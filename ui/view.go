// Package ui draws the goTrust window.
//
// The view is a plain function of the app state: it reads State and writes only
// through the callbacks the App provides. Nothing here touches the network, so
// the whole interface can be rendered headlessly in a test.
package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
)

// State is what the window shows.
type State struct {
	Session session.State
	// Config is the settings currently in effect.
	Config config.Config
	// Addresses are the bound proxy listeners.
	Addresses []string
	// Status is a one-line message about the last user action.
	Status string
	// Busy disables the controls that would race a login or a logout.
	Busy bool
	// Username and Password are the fields being edited. They live here so the
	// login view can be redrawn without losing what was typed.
	Username string
	Password string
	Server   string
	Port     string
}

// Actions are what the window can ask the application to do.
type Actions struct {
	Login  func()
	Logout func()
	Reload func()
	Quit   func()
	// Save remembers the edited settings without connecting.
	Save func()
}

// View renders one frame.
func View(state *State, actions Actions) func(c *ui.Context) {
	return func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Gap(14).Children(func() {
			header(c, state)
			fields(c, state, actions)
			controls(c, state, actions)
			footer(c, state)
		})
	}
}

func header(c *ui.Context, state *State) {
	ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
		ui.Text(c, "goTrust").FontSize(20).Bold()
		ui.Text(c, phaseLabel(state.Session)).FontSize(13)
	})
	ui.Text(c, phaseDetail(state.Session)).FontSize(12)
}

// phaseLabel is the short word next to the title.
func phaseLabel(s session.State) string {
	switch s.Phase {
	case session.PhaseOnline:
		return "已连接"
	case session.PhaseConnecting:
		return "连接中"
	case session.PhaseFailed:
		return "失败"
	default:
		return "未连接"
	}
}

// phaseDetail explains the phase in one line, including how long it has held.
func phaseDetail(s session.State) string {
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
	case session.PhaseFailed:
		return s.Detail
	default:
		return s.Detail
	}
}

func fields(c *ui.Context, state *State, actions Actions) {
	ui.Form(c, func() {
		ui.Column(c).Gap(10).Children(func() {
			ui.Field(c, "网关地址", func() {
				ui.TextInput(c, &state.Server).Placeholder("vpn.example.com")
			})
			ui.Field(c, "端口", func() {
				ui.TextInput(c, &state.Port).Placeholder("443")
			})
			ui.Field(c, "用户名", func() {
				ui.TextInput(c, &state.Username)
			})
			ui.Field(c, "密码", func() {
				ui.TextInput(c, &state.Password).Password()
			})
		})
	})
}

func controls(c *ui.Context, state *State, actions Actions) {
	ui.Row(c).Gap(10).Children(func() {
		switch state.Session.Phase {
		case session.PhaseOnline, session.PhaseConnecting:
			if ui.Button(c, "断开").Clicked() && actions.Logout != nil {
				actions.Logout()
			}
		default:
			if ui.PrimaryButton(c, "连接").Clicked() && actions.Login != nil {
				actions.Login()
			}
		}
		if ui.Button(c, "保存设置").Clicked() && actions.Save != nil {
			actions.Save()
		}
		if ui.Button(c, "重新读取").Clicked() && actions.Reload != nil {
			actions.Reload()
		}
		if ui.Button(c, "退出").Clicked() && actions.Quit != nil {
			actions.Quit()
		}
	})
}

func footer(c *ui.Context, state *State) {
	ui.Divider(c)
	if len(state.Addresses) == 0 {
		ui.Text(c, "代理端口尚未开启").FontSize(12)
	} else {
		for _, address := range state.Addresses {
			ui.Text(c, "代理监听 "+address).FontSize(12)
		}
	}
	if state.Status != "" {
		ui.Text(c, state.Status).FontSize(12)
	}
}

// TrayMenu builds the tray menu for the current state.
func TrayMenu(state *State, actions Actions) *mygo.Menu {
	show := &mygo.MenuItem{ID: "show", Label: "显示窗口"}
	toggle := &mygo.MenuItem{ID: "toggle", Label: "连接"}
	if state.Session.Phase == session.PhaseOnline || state.Session.Phase == session.PhaseConnecting {
		toggle.Label = "断开"
		toggle.Click = func(*mygo.MenuItem, *mygo.Window) {
			if actions.Logout != nil {
				actions.Logout()
			}
		}
	} else {
		toggle.Click = func(*mygo.MenuItem, *mygo.Window) {
			if actions.Login != nil {
				actions.Login()
			}
		}
	}
	quit := &mygo.MenuItem{ID: "quit", Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) {
		if actions.Quit != nil {
			actions.Quit()
		}
	}}
	separator := &mygo.MenuItem{Type: mygo.MenuItemSeparator}
	return mygo.NewMenu([]*mygo.MenuItem{show, separator, toggle, quit})
}
