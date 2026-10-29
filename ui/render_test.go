// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image"
	"image/color"
	"image/png"
	"math"
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
				Traffic:   renderTraffic(),
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
	savePage := func(name string, view *State) {
		t.Helper()
		save(name, ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 900, 740))
	}

	t.Run("settings", func(t *testing.T) { savePage("settings", renderPageState(cfg, PageAccount)) })
	t.Run("settings-proxy", func(t *testing.T) { savePage("settings-proxy", renderPageState(cfg, PageProxy)) })
	t.Run("settings-keepalive", func(t *testing.T) { savePage("settings-keepalive", renderPageState(cfg, PageKeepAlive)) })
	t.Run("settings-startup", func(t *testing.T) { savePage("settings-startup", renderPageState(cfg, PageStartup)) })
	t.Run("settings-terminal", func(t *testing.T) { savePage("settings-terminal", renderPageState(cfg, PageTerminals)) })
	t.Run("about", func(t *testing.T) { savePage("about", renderPageState(cfg, PageAbout)) })
	t.Run("settings-logs", func(t *testing.T) { savePage("settings-logs", renderPageState(cfg, PageLogs)) })
	t.Run("settings-accounts", func(t *testing.T) {
		view := renderPageState(cfg, PageProfiles)
		state := view.Settings
		if _, err := state.Draft.AddProfile("工作"); err != nil {
			t.Fatalf("add account: %v", err)
		}
		state.Draft.Profiles[1].Server.Address = "vpn2.example.com"
		view.Config = state.Draft
		state.refresh()
		state.Select(1)
		savePage("settings-accounts", view)
	})
	t.Run("settings-unsaved", func(t *testing.T) {
		view := renderPageState(cfg, PageAccount)
		view.Settings.Gateways[0] = "vpn.example.com:8443"
		savePage("settings-unsaved", view)
	})
	// The minimum width the window declares, tall enough for the whole page:
	// the labels that share a row with a button are the ones that overflow.
	t.Run("settings-narrow", func(t *testing.T) {
		view := renderPageState(cfg, PageAccount)
		save("settings-narrow", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 720, 560))
	})
	t.Run("settings-small", func(t *testing.T) {
		view := renderPageState(cfg, PageAccount)
		save("settings-small", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 720, 560))
	})
	t.Run("account-fresh", func(t *testing.T) {
		savePage("account-fresh", renderPageState(fresh, PageAccount))
	})
	t.Run("account-fresh-small", func(t *testing.T) {
		view := renderPageState(fresh, PageAccount)
		save("account-fresh-small", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 720, 560))
	})
	t.Run("settings-password", func(t *testing.T) {
		view := renderPageState(cfg, PageAccount)
		view.Settings.Password = &PasswordPrompt{Username: "alice", Strategy: passwordTestStrategy()}
		savePage("settings-password", view)
	})
	t.Run("settings-dark", func(t *testing.T) {
		view := renderPageState(cfg, PageAccount)
		tester := ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 900, 740)
		tester.SetDark(true)
		save("settings-dark", tester)
	})

	// Every page at the minimum size and in the dark appearance: the layout
	// and the forms are what break there, not the strings the other tests
	// assert.
	pages := []struct {
		name string
		page Page
	}{
		{"dashboard", PageDashboard},
		{"account", PageAccount},
		{"proxy", PageProxy},
		{"keepalive", PageKeepAlive},
		{"startup", PageStartup},
		{"terminals", PageTerminals},
		{"logs", PageLogs},
		{"settings", PageSettings},
		{"about", PageAbout},
		{"profiles", PageProfiles},
	}
	for _, page := range pages {
		name, chosen := page.name, page.page
		t.Run("page-"+name+"-small", func(t *testing.T) {
			view := renderPageState(cfg, chosen)
			save("page-"+name+"-small", ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 720, 560))
		})
		t.Run("page-"+name+"-dark", func(t *testing.T) {
			view := renderPageState(cfg, chosen)
			tester := ui.NewTester(View(view, Actions{Settings: SettingsActions{}}), 900, 740)
			tester.SetDark(true)
			save("page-"+name+"-dark", tester)
		})
	}

	t.Run("main-online-dark", func(t *testing.T) {
		state := State{Session: online, Config: cfg, Listeners: renderListeners(), Traffic: renderTraffic()}
		tester := ui.NewTester(View(&state, Actions{}), 900, 740)
		tester.SetDark(true)
		save("main-online-dark", tester)
	})
	t.Run("main-small", func(t *testing.T) {
		state := State{Session: online, Config: cfg, Listeners: renderListeners(), Traffic: renderTraffic()}
		save("main-small", ui.NewTester(View(&state, Actions{}), 720, 560))
	})
	t.Run("main-resources", func(t *testing.T) {
		state := State{Session: longGrant(), Config: cfg, Listeners: renderListeners(), Traffic: renderTraffic()}
		save("main-resources", ui.NewTester(View(&state, Actions{}), 900, 740))
	})
	t.Run("main-resources-small", func(t *testing.T) {
		state := State{Session: longGrant(), Config: cfg, Listeners: renderListeners(), Traffic: renderTraffic()}
		save("main-resources-small", ui.NewTester(View(&state, Actions{}), 720, 560))
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
}

// renderPageState is one page with a live session behind it: the terminal
// list, the log tail and the PAC address a user sees after a successful login.
func renderPageState(cfg config.Config, page Page) *State {
	state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
	state.LogPath = "/home/user/.local/state/gotrust/gotrust.log"
	state.LogLines = renderLogLines()
	state.Online = true
	state.TrustKnown = true
	state.Trusted = true
	state.TerminalsKnown = true
	state.Terminals = &session.Terminals{
		SelfID:         "self",
		MaxDeviceCount: 5,
		DeviceTrusted:  true,
		Trusted: []session.Terminal{
			{
				ID: "self", DevDBID: "6f1c9a2e4b7d", DeviceName: "本机", DeviceType: "client",
				OS: "Linux", OSVersion: "6.12", OnlineStatus: true,
				LastLoginAddress: "LAN", LastLoginIP: "192.0.2.10",
				LastLoginTime: "2026-10-29 09:12", NetworkZoneList: []string{"校园网", "无线"},
			},
			{
				ID: "other", DevDBID: "2a9d51c0f8e3", DeviceName: "MacBook Pro", DeviceType: "browser",
				OS: "macOS", OSVersion: "15.1", Browser: "Safari 18",
				LastLoginAddress: "203.0.113.7", LastLoginTime: "2026-10-28 21:40",
				NetworkZoneList: []string{"互联网"},
			},
		},
		Temporary: []session.Terminal{{
			ID: "temp", DevDBID: "7c3e0f9a1b62", DeviceName: "Windows 工作站", DeviceType: "client",
			OS: "Windows", OSVersion: "11 23H2", LastLoginAddress: "198.51.100.24",
			LastLoginTime: "2026-10-28 18:05", NetworkZoneList: []string{"互联网"},
		}},
	}
	return &State{
		Settings:  state,
		Config:    cfg,
		Page:      page,
		PACURL:    "http://127.0.0.1:1082/pac",
		Session:   session.State{Phase: session.PhaseOnline, ClientIP: "203.0.113.4", Since: time.Now().Add(-17 * time.Minute)},
		Listeners: renderListeners(),
		Traffic:   renderTraffic(),
	}
}

// renderLogLines is a short tail for the 日志 page: what the app writes after a
// login, without any account material.
func renderLogLines() []string {
	return []string{
		"2026/10/29 09:12:03 goTrust ready; profile 默认",
		"2026/10/29 09:12:03 keep-alive: 1 target(s) every 1m30s",
		"2026/10/29 09:12:04 Perform GET /passport/v1/security/queryDevice status=trust",
		"2026/10/29 09:12:04 SOCKS5 listening on 127.0.0.1:1080",
		"2026/10/29 09:12:04 HTTP proxy listening on 127.0.0.1:1081",
		"2026/10/29 09:12:05 keep-alive: example.com answered 200",
	}
}

// renderTraffic is a minute of proxy throughput for the dashboard card: a
// download with a few waves and an upload a fraction of it, so both lines and
// the fading fill are visible at both window sizes.
func renderTraffic() TrafficState {
	up := make([]float64, TrafficPoints)
	down := make([]float64, TrafficPoints)
	for i := range TrafficPoints {
		down[i] = 1.6e6 + 1.1e6*math.Sin(float64(i)/6.5) + 0.35e6*math.Sin(float64(i)/2.2)
		up[i] = 0.28e6 + 0.16e6*math.Sin(float64(i)/9)
	}
	return TrafficState{Up: up[TrafficPoints-1], Down: down[TrafficPoints-1], UpSeries: up, DownSeries: down}
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
