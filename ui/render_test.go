// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/proxy"
	"github.com/YangtseSu/goTrust/core/session"
)

// TestRenderEveryState writes the windows as PNGs for a human to look at:
//
//	GOTRUST_RENDER_DIR=/tmp/gotrust-ui go test ./ui -run TestRenderEveryState
//
// It is skipped without the variable, so the normal suite stays headless and
// writes nothing. The images are the evidence for a layout or colour change;
// the assertions in the other tests only prove that a string is present.
func TestRenderEveryState(t *testing.T) {
	dir := os.Getenv("GOTRUST_RENDER_DIR")
	if dir == "" {
		t.Skip("set GOTRUST_RENDER_DIR to write the rendered states")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create the render directory: %v", err)
	}

	// Both languages, because a label that fits in Chinese may not fit in
	// English. The source language keeps the plain names, so the images before
	// a change stay comparable.
	for _, lang := range []string{"zh", "en"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("MYGO_LOCALE", lang)
			renderStates(t, dir, lang)
		})
	}
}

// renderStates writes every window state in one language.
func renderStates(t *testing.T, dir, lang string) {
	t.Helper()

	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	cfg.Profiles[0].Auth.Username = "alice"

	// A fresh installation: no gateway, no account.
	fresh := config.Default()

	resources := session.Resources{Entries: []session.ResourceEntry{
		{Kind: session.ResourceApp, Title: "知识库", Detail: "门户应用 · Web", URL: "https://wiki.example.com", Copy: "https://wiki.example.com"},
		{Kind: session.ResourceApp, Title: "代码仓库", Detail: "研发 · Web", URL: "https://git.example.com", Copy: "https://git.example.com"},
		{Kind: session.ResourceIP, Title: "192.0.2.10", Detail: "tcp 443, 8080", Copy: "192.0.2.10"},
		{Kind: session.ResourceDomain, Title: "build.example.com", Detail: "tcp 22", Copy: "build.example.com"},
		{Kind: session.ResourceDNS, Title: "db.example.com", Detail: "192.0.2.20", Copy: "db.example.com"},
	}}

	online := session.State{
		Phase:     session.PhaseOnline,
		Detail:    "vpn.example.com:443",
		ClientIP:  "203.0.113.4",
		Since:     time.Now().Add(-17 * time.Minute),
		Resources: resources,
		KeepAlive: session.KeepAliveState{Target: "http://192.0.2.10/", Status: 200, OK: true, At: time.Now()},
	}

	// A hidden title bar leaves the desktop's window controls in the top
	// right; the renders reserve the room a Linux desktop's three buttons take.
	testerTitleBar := ui.TitleBar{Height: TitleBarHeight, Right: 120}

	save := func(name string, tester *ui.Tester) {
		t.Helper()
		if lang != "zh" {
			name += "-" + lang
		}
		tester.SetTitleBar(testerTitleBar)
		// MyGo's own widget strings read the window's language, while the views
		// read the app's catalog through i18n: the harness has to say both, or a
		// search field keeps its English placeholder in the Chinese window.
		tester.SetLocale(lang)
		tester.Frame()
		// The toast fades in; one frame at its start is transparent.
		tester.Advance(250 * time.Millisecond)
		tester.Frame()
		file, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer file.Close()
		if err := png.Encode(file, tester.Image()); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}

	cases := []struct {
		name  string
		state State
	}{
		{
			name:  "main-idle",
			state: State{Session: session.State{Phase: session.PhaseIdle, Detail: "未连接"}, Config: cfg},
		},
		{
			name: "main-connecting",
			state: State{
				Session:   session.State{Phase: session.PhaseConnecting, Detail: "正在登录", Since: time.Now()},
				Config:    cfg,
				Status:    "正在连接 vpn.example.com:443",
				StatusSeq: 1,
				Busy:      true,
			},
		},
		{
			name: "main-online",
			state: State{
				Session:   online,
				Config:    cfg,
				Listeners: renderListeners(),
			},
		},
		{
			name: "main-reconnecting",
			state: State{
				Session: session.State{
					Phase:     session.PhaseReconnecting,
					Detail:    "会话已过期",
					Since:     time.Now(),
					Reconnect: session.ReconnectState{Attempt: 2, Max: 5, NextAt: time.Now().Add(4 * time.Second)},
					KeepAlive: session.KeepAliveState{Target: "http://192.0.2.10/", Err: "timeout", At: time.Now()},
				},
				Config: cfg,
			},
		},
		{
			name: "main-failed",
			state: State{
				Session:     session.State{Phase: session.PhaseFailed, Detail: "认证失败：账号或密码错误", Since: time.Now()},
				Config:      cfg,
				Status:      "登录失败，请检查设置",
				StatusSeq:   1,
				StatusError: true,
			},
		},
		{
			name: "main-captcha",
			state: State{
				Session: session.State{Phase: session.PhaseConnecting, Detail: "需要点击验证码"},
				Config:  cfg,
				Captcha: &Captcha{
					Image:  renderBitmap(),
					Width:  120,
					Height: 48,
					Points: []Point{{X: 30, Y: 24}},
				},
				Status:    "请在窗口中点击图形验证码",
				StatusSeq: 1,
			},
		},
		{
			name: "main-code",
			state: State{
				Session: session.State{Phase: session.PhaseConnecting, Detail: "需要二次验证"},
				Config:  cfg,
				Code: &CodePrompt{
					Kind:        session.CodeTOTP,
					Message:     "请输入身份验证器中的 6 位数字",
					CanSkip:     true,
					StoreSecret: true,
				},
			},
		},
		{
			name: "main-code-granted",
			state: State{
				Session: session.State{
					Phase:     session.PhaseConnecting,
					Detail:    "需要二次验证",
					Resources: resources,
				},
				Config: cfg,
				Code: &CodePrompt{
					Kind:    session.CodeSMS,
					Message: "请输入短信验证码",
					CanSkip: true,
				},
			},
		},
		{
			name: "main-sso",
			state: State{
				Session: session.State{Phase: session.PhaseConnecting, Detail: "等待浏览器登录"},
				Config:  cfg,
				External: &ExternalLogin{
					Kind:     session.ExternalCAS,
					LoginURL: "https://sso.example.com/cas/login?service=https%3A%2F%2Fvpn.example.com%2Fpassport%2Fv1%2Fauth",
					Message:  "请在统一身份认证页面完成登录",
				},
			},
		},
		{
			name: "main-password",
			state: State{
				Session: session.State{Phase: session.PhaseConnecting, Detail: "网关要求重置密码"},
				Config:  cfg,
				Password: &PasswordPrompt{
					Message:  "密码已过期，请修改后继续登录",
					Username: "alice",
					Prompt:   "密码需包含至少两类字符",
					CanSkip:  true,
					Strategy: passwordTestStrategy(),
				},
			},
		},
		{
			name: "main-choice",
			state: State{
				Session: session.State{Phase: session.PhaseConnecting, Detail: "选择认证方式"},
				Config:  cfg,
				Choice: &AuthChoice{Methods: []session.AuthMethod{
					{Name: "密码登录", Type: "auth/psw", Domain: "local"},
					{Name: "统一认证", Type: "auth/cas", Domain: "local"},
				}},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			state := testCase.state
			save(testCase.name, ui.NewTester(View(&state, Actions{}), 900, 740))
		})
	}

	// The window's pages, one at a time: each is rendered, because a page that
	// does not fit shows its own scroll rather than the next page's fields.
	pageView := func(name string, chosen Page, mutate func(*SettingsState)) {
		t.Helper()
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		if mutate != nil {
			mutate(state)
		}
		view := &State{Settings: state, Config: cfg, Page: chosen}
		save(name, ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 900, 740))
	}

	t.Run("settings", func(t *testing.T) { pageView("settings", PageAccount, nil) })
	t.Run("settings-proxy", func(t *testing.T) { pageView("settings-proxy", PageProxy, nil) })
	t.Run("settings-keepalive", func(t *testing.T) {
		pageView("settings-keepalive", PageKeepAlive, nil)
	})
	t.Run("settings-startup", func(t *testing.T) { pageView("settings-startup", PageStartup, nil) })
	t.Run("settings-advanced", func(t *testing.T) {
		pageView("settings-advanced", PageSettings, nil)
	})
	t.Run("settings-terminal", func(t *testing.T) {
		pageView("settings-terminal", PageTerminals, func(state *SettingsState) {
			state.Online = true
			state.TrustKnown = true
			state.Trusted = true
			state.TerminalsKnown = true
			state.Terminals = &session.Terminals{
				SelfID: "self",
				Trusted: []session.Terminal{{
					ID: "trusted-1", DeviceName: "desk-01", OS: "Linux", OSVersion: "7.3.0",
					LastLoginAddress: "LAN", LastLoginIP: "192.0.2.10", LastLoginTime: "2026-10-25 13:17:59",
				}},
				Temporary: []session.Terminal{{
					ID: "self", DeviceName: "desk-02", OS: "Linux", OSVersion: "7.3.0",
					OnlineStatus: true, LastLoginAddress: "LAN", LastLoginIP: "192.0.2.11",
					LastLoginTime: "2026-10-25 13:17:59",
				}},
			}
		})
	})
	// 多账户: two accounts, the live one and the one the draft edits.
	t.Run("settings-accounts", func(t *testing.T) {
		pageView("settings-accounts", PageProfiles, func(state *SettingsState) {
			if _, err := state.Draft.AddProfile("工作"); err != nil {
				t.Fatalf("add account: %v", err)
			}
			state.Draft.Profiles[1].Server.Address = "vpn2.example.com"
			state.refresh()
			state.Select(1)
		})
	})
	t.Run("settings-unsaved", func(t *testing.T) {
		pageView("settings-unsaved", PageAccount, func(state *SettingsState) {
			state.Gateways[0] = "vpn.example.com:8443"
		})
	})
	// The minimum width the window declares, tall enough for the whole page:
	// the labels that share a row with a button are the ones that overflow.
	t.Run("settings-narrow", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		view := &State{Settings: state, Config: cfg, Page: PageAccount}
		save("settings-narrow", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 720, 560))
	})

	t.Run("main-online-dark", func(t *testing.T) {
		state := State{
			Session:   online,
			Config:    cfg,
			Listeners: renderListeners(),
		}
		tester := ui.NewTester(View(&state, Actions{}), 900, 740)
		tester.SetDark(true)
		save("main-online-dark", tester)
	})

	t.Run("main-small", func(t *testing.T) {
		state := State{
			Session:   online,
			Config:    cfg,
			Listeners: renderListeners(),
		}
		save("main-small", ui.NewTester(View(&state, Actions{}), 720, 560))
	})

	t.Run("main-resources", func(t *testing.T) {
		state := State{
			Session:   longGrant(),
			Config:    cfg,
			Listeners: renderListeners(),
		}
		save("main-resources", ui.NewTester(View(&state, Actions{}), 900, 740))
	})

	t.Run("main-resources-small", func(t *testing.T) {
		state := State{
			Session:   longGrant(),
			Config:    cfg,
			Listeners: renderListeners(),
		}
		save("main-resources-small", ui.NewTester(View(&state, Actions{}), 720, 560))
	})

	t.Run("settings-small", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		view := &State{Settings: state, Config: cfg, Page: PageAccount}
		save("settings-small", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 720, 560))
	})

	t.Run("main-password-small", func(t *testing.T) {
		state := State{
			Session: session.State{Phase: session.PhaseConnecting, Detail: "网关要求重置密码"},
			Config:  cfg,
			Password: &PasswordPrompt{
				Message:  "密码已过期，请修改后继续登录",
				Username: "alice",
				Prompt:   "密码需包含至少两类字符",
				CanSkip:  true,
				Strategy: passwordTestStrategy(),
			},
		}
		save("main-password-small", ui.NewTester(View(&state, Actions{}), 720, 560))
	})

	t.Run("account-fresh", func(t *testing.T) {
		state := NewSettingsState(fresh, "/home/user/.local/state/gotrust")
		view := &State{Settings: state, Config: fresh, Page: PageAccount}
		save("account-fresh", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 900, 740))
	})

	t.Run("account-fresh-small", func(t *testing.T) {
		state := NewSettingsState(fresh, "/home/user/.local/state/gotrust")
		view := &State{Settings: state, Config: fresh, Page: PageAccount}
		save("account-fresh-small", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 720, 560))
	})

	t.Run("settings-password", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		state.Online = true
		state.Password = &PasswordPrompt{Username: "alice", Strategy: passwordTestStrategy()}
		view := &State{Settings: state, Config: cfg, Page: PageAccount}
		save("settings-password", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 900, 740))
	})

	t.Run("settings-dark", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		view := &State{Settings: state, Config: cfg, Page: PageAccount}
		tester := ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 900, 740)
		tester.SetDark(true)
		save("settings-dark", tester)
	})
}

// renderListeners are the two listeners a session binds by default.
func renderListeners() []proxy.Listener {
	return []proxy.Listener{
		{Protocol: proxy.ProtocolSOCKS5, Address: "127.0.0.1:1080"},
		{Protocol: proxy.ProtocolHTTP, Address: "127.0.0.1:1081"},
	}
}

// longGrant is an account with more rows than one screen: a portal application
// list long enough for the filter, and rules of every other kind under it.
func longGrant() session.State {
	entries := make([]session.ResourceEntry, 0, 20)
	for _, app := range []string{"知识库", "代码仓库", "构建平台", "监控面板", "工单系统", "数据分析", "日志检索", "制品库", "配置中心", "消息队列", "调度平台", "接口文档"} {
		url := "https://portal.example.com/" + app
		entries = append(entries, session.ResourceEntry{Kind: session.ResourceApp, Title: app, Detail: "门户应用 · Web", URL: url, Copy: url})
	}
	entries = append(entries,
		session.ResourceEntry{Kind: session.ResourceDomain, Title: "build.example.com", Detail: "tcp 22", Copy: "build.example.com"},
		session.ResourceEntry{Kind: session.ResourceDomain, Title: "git.example.com", Detail: "tcp 443", Copy: "git.example.com"},
		session.ResourceEntry{Kind: session.ResourceIP, Title: "192.0.2.10", Detail: "tcp 443, 8080", Copy: "192.0.2.10"},
		session.ResourceEntry{Kind: session.ResourceIP, Title: "192.0.2.0 – 192.0.2.255", Detail: "tcp 1-65535", Copy: "192.0.2.0 – 192.0.2.255"},
		session.ResourceEntry{Kind: session.ResourceDNS, Title: "db.example.com", Detail: "192.0.2.20", Copy: "db.example.com"},
	)

	return session.State{
		Phase:     session.PhaseOnline,
		Detail:    "vpn.example.com:443",
		ClientIP:  "203.0.113.4",
		Since:     time.Now().Add(-17 * time.Minute),
		Resources: session.Resources{Entries: entries},
		KeepAlive: session.KeepAliveState{Target: "http://192.0.2.10/", Status: 200, OK: true, At: time.Now()},
	}
}

// renderBitmap is a placeholder captcha image: grey with two dark bars, so the
// markers and the panel around it are visible in the rendered window.
func renderBitmap() *ui.Bitmap {
	width, height := 120, 48
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		for y := range height {
			shade := uint8(226)
			if x%30 < 6 {
				shade = 96
			}
			img.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	return ui.NewBitmap(img)
}
