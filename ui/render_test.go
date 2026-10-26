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

	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	cfg.Profiles[0].Auth.Username = "alice"

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

	save := func(name string, tester *ui.Tester) {
		t.Helper()
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
				Session: session.State{Phase: session.PhaseConnecting, Detail: "正在登录", Since: time.Now()},
				Config:  cfg,
				Status:  "正在连接 vpn.example.com:443",
				Busy:    true,
			},
		},
		{
			name: "main-online",
			state: State{
				Session:   online,
				Config:    cfg,
				Addresses: []string{"127.0.0.1:1080", "127.0.0.1:1081"},
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
				Status: "请在窗口中点击图形验证码",
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
			save(testCase.name, ui.NewTester(View(&state, Actions{}), 540, 640))
		})
	}

	t.Run("settings", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		save("settings", ui.NewTester(SettingsView(state, SettingsActions{}), 600, 760))
	})

	t.Run("settings-full", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		save("settings-full", ui.NewTester(SettingsView(state, SettingsActions{}), 600, 1500))
	})

	t.Run("main-online-dark", func(t *testing.T) {
		state := State{
			Session:   online,
			Config:    cfg,
			Addresses: []string{"127.0.0.1:1080", "127.0.0.1:1081"},
		}
		tester := ui.NewTester(View(&state, Actions{}), 540, 640)
		tester.SetDark(true)
		save("main-online-dark", tester)
	})

	t.Run("main-small", func(t *testing.T) {
		state := State{
			Session:   online,
			Config:    cfg,
			Addresses: []string{"127.0.0.1:1080", "127.0.0.1:1081"},
		}
		save("main-small", ui.NewTester(View(&state, Actions{}), 440, 400))
	})

	t.Run("settings-small", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		save("settings-small", ui.NewTester(SettingsView(state, SettingsActions{}), 560, 440))
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
		save("main-password-small", ui.NewTester(View(&state, Actions{}), 440, 400))
	})

	t.Run("settings-password", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		state.Online = true
		state.Password = &PasswordPrompt{Username: "alice", Strategy: passwordTestStrategy()}
		save("settings-password", ui.NewTester(SettingsView(state, SettingsActions{}), 600, 760))
	})

	t.Run("settings-dark", func(t *testing.T) {
		state := NewSettingsState(cfg, "/home/user/.local/state/gotrust")
		tester := ui.NewTester(SettingsView(state, SettingsActions{}), 600, 760)
		tester.SetDark(true)
		save("settings-dark", tester)
	})
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
