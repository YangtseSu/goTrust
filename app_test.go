// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/config"
	gotrustproxy "github.com/YangtseSu/goTrust/core/proxy"
	"github.com/YangtseSu/goTrust/core/session"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

func TestDecideAutoConnect(t *testing.T) {
	ready := config.Default()
	ready.App.AutoConnect = true
	ready.Profiles[0].Server.Address = "vpn.example.com"
	ready.Profiles[0].Auth.Username = "alice"

	cases := []struct {
		name   string
		mutate func(*config.Config)
		want   bool
		reason string
	}{
		{
			name:   "option off",
			mutate: func(cfg *config.Config) { cfg.App.AutoConnect = false },
		},
		{
			name:   "no gateway",
			mutate: func(cfg *config.Config) { cfg.Profiles[0].Server.Address = "" },
			reason: "网关",
		},
		{
			name:   "no username",
			mutate: func(cfg *config.Config) { cfg.Profiles[0].Auth.Username = "" },
			reason: "用户名",
		},
		{
			name:   "no secret",
			mutate: func(cfg *config.Config) {},
			reason: "密码",
		},
		{
			name: "stored password",
			mutate: func(cfg *config.Config) {
				cfg.Profiles[0].Auth.StorePass = true
				cfg.Profiles[0].Auth.Password = "secret"
			},
			want: true,
		},
		{
			name: "unstored password is not stored",
			mutate: func(cfg *config.Config) {
				cfg.Profiles[0].Auth.StorePass = false
				cfg.Profiles[0].Auth.Password = "secret"
			},
			reason: "密码",
		},
		{
			// A session belongs to its connection; stored client data alone
			// is not a reason to connect without a password.
			name:   "stored session is not a credential",
			mutate: func(cfg *config.Config) {},
			reason: "密码",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := ready.Clone()
			testCase.mutate(&cfg)
			decision := decideAutoConnect(cfg)
			if decision.Connect != testCase.want {
				t.Fatalf("connect = %v, want %v (reason %q)", decision.Connect, testCase.want, decision.Reason)
			}
			if testCase.reason != "" && !strings.Contains(decision.Reason, testCase.reason) {
				t.Fatalf("reason = %q, want it to mention %q", decision.Reason, testCase.reason)
			}
		})
	}
}

func TestCfgForDiskClearsUnstoredPasswords(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Auth.StorePass = true
	cfg.Profiles[0].Auth.Password = "stored"
	cfg.Profiles[0].Auth.TOTPSecret = "stored-secret"
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	cfg.Profiles[1].Auth.StorePass = false
	cfg.Profiles[1].Auth.Password = "memory-only"
	cfg.Profiles[1].Auth.TOTPSecret = "memory-only-secret"

	disk := cfgForDisk(cfg)
	if disk.Profiles[0].Auth.Password != "stored" || disk.Profiles[0].Auth.TOTPSecret != "stored-secret" {
		t.Fatal("a stored credential was cleared from the file")
	}
	if disk.Profiles[1].Auth.Password != "" || disk.Profiles[1].Auth.TOTPSecret != "" {
		t.Fatal("an unstored credential reached the file")
	}
	if cfg.Profiles[1].Auth.Password != "memory-only" || cfg.Profiles[1].Auth.TOTPSecret != "memory-only-secret" {
		t.Fatal("the in-memory credential was cleared")
	}
}

func TestActiveProfileFeedsTheSession(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "first.example.com"
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	cfg.Profiles[1].Server.Address = "second.example.com"

	application := &app{cfg: cfg}
	if got := application.currentConfig().Active().ServerAddress(); got != "second.example.com" {
		t.Fatalf("active server = %q", got)
	}

	cfg.ActiveProfile = config.DefaultProfileName
	application.applyConfig(cfg)
	if got := application.currentConfig().Active().ServerAddress(); got != "first.example.com" {
		t.Fatalf("active server after switch = %q", got)
	}
}

func TestTooltipMirrorsPhaseAndKeepAlive(t *testing.T) {
	application := &app{}
	application.view.Session = session.State{
		Phase:    session.PhaseOnline,
		ClientIP: "203.0.113.4",
		KeepAlive: session.KeepAliveState{
			Target: "http://192.0.2.10/",
			Status: 200,
			OK:     true,
			At:     time.Now(),
		},
	}

	tooltip := application.tooltip(application.view)
	if !strings.Contains(tooltip, "已连接 203.0.113.4") {
		t.Fatalf("tooltip = %q", tooltip)
	}
	if !strings.Contains(tooltip, "保活: OK 200") {
		t.Fatalf("tooltip = %q", tooltip)
	}

	application.view.Session = session.State{Phase: session.PhaseFailed, Detail: "bad password"}
	if got := application.tooltip(application.view); !strings.Contains(got, "失败: bad password") {
		t.Fatalf("tooltip = %q", got)
	}
}

// The reconnect phase is an active session: 断开 must stop it, and the tray
// must count down rather than claim the user is disconnected.
func TestReconnectIsAnActiveSession(t *testing.T) {
	for _, phase := range []session.Phase{session.PhaseOnline, session.PhaseConnecting, session.PhaseReconnecting} {
		if !sessionActive(phase) {
			t.Fatalf("phase %q is not treated as active", phase)
		}
	}
	for _, phase := range []session.Phase{session.PhaseIdle, session.PhaseFailed} {
		if sessionActive(phase) {
			t.Fatalf("phase %q is treated as active", phase)
		}
	}

	state := session.State{
		Phase: session.PhaseReconnecting,
		Reconnect: session.ReconnectState{
			Attempt: 2,
			Max:     5,
			NextAt:  time.Now().Add(4 * time.Second),
		},
	}
	if got := gotrustui.TrayStatus(state); !strings.Contains(got, "重连中 (2/5") {
		t.Fatalf("tray status = %q, want the ladder position", got)
	}
}

// Saving the settings window must write TOML, make the selected profile active
// and leave the in-memory password usable for this run.
func TestSaveSettingsWritesAndApplies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Default()
	application := &app{cfg: cfg, configPath: path}
	application.view.Config = cfg

	state := gotrustui.NewSettingsState(cfg, "/tmp/logs")
	state.NameText = "工作"
	state.Draft.Profiles[0].Server.Address = "vpn.example.com"
	state.Draft.Profiles[0].Auth.Username = "alice"
	state.Draft.Profiles[0].Auth.Password = "memory-only"
	state.Ports[0] = "8443"
	application.settings = state

	application.saveSettings()

	if state.Status != "设置已保存" {
		t.Fatalf("status = %q", state.Status)
	}
	if application.currentConfig().ActiveProfile != "工作" {
		t.Fatalf("active = %q", application.currentConfig().ActiveProfile)
	}
	if got := application.currentConfig().Active().Auth.Password; got != "memory-only" {
		t.Fatalf("in-memory password = %q", got)
	}

	onDisk, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	profile := onDisk.Active()
	if profile.Name != "工作" || profile.Server.Address != "vpn.example.com" ||
		profile.Server.Port != 8443 || profile.Auth.Username != "alice" {
		t.Fatalf("on-disk profile = %+v", profile)
	}
	if profile.Auth.Password != "" {
		t.Fatal("an unstored password reached the settings file")
	}
}

// A save that fails validation must not touch the file or the live settings.
func TestSaveSettingsRejectsBadValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Default()
	application := &app{cfg: cfg, configPath: path}
	application.view.Config = cfg

	state := gotrustui.NewSettingsState(cfg, "/tmp/logs")
	state.Ports[0] = "0"
	application.settings = state

	application.saveSettings()

	if state.Status == "" || state.Status == "设置已保存" {
		t.Fatalf("status = %q", state.Status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a rejected save wrote the file: %v", err)
	}
	if application.currentConfig().Active().Server.Port != 443 {
		t.Fatal("a rejected save changed the live settings")
	}
}

// testProxyApp installs a proxy server bound to an ephemeral loopback port and
// points the settings at that same port, so a restart has to rebind it.
func testProxyApp(t *testing.T) (*app, *gotrustproxy.Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	server, err := gotrustproxy.New(gotrustproxy.Options{
		SOCKS5Bind: "127.0.0.1:0",
		Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, session.ErrNotOnline
		},
	})
	if err != nil {
		cancel()
		t.Fatalf("new proxy: %v", err)
	}
	if err := server.Start(ctx); err != nil {
		cancel()
		t.Fatalf("start proxy: %v", err)
	}

	cfg := config.Default()
	cfg.Proxy.SOCKS5Bind = server.Addresses()[0]
	cfg.Proxy.HTTPBind = ""

	application := &app{
		cfg:     cfg,
		manager: session.New(session.Options{Config: func() config.Config { return cfg }}),
		proxy:   server,
		view:    gotrustui.State{Addresses: server.Addresses()},
	}
	application.proxyCancel = cancel
	t.Cleanup(application.releaseProxy)
	return application, server
}

// A connect after a lost session has to rebind the configured ports. The old
// listeners were still open, so the new bind failed with "address already in
// use" on every retry until the process exited.
func TestRestartProxyClosesTheOldListenersBeforeBinding(t *testing.T) {
	application, old := testProxyApp(t)
	address := old.Addresses()[0]

	if err := application.restartProxy(); err != nil {
		t.Fatalf("restart the proxy on %s: %v", address, err)
	}

	application.mu.RLock()
	server := application.proxy
	addresses := application.view.Addresses
	application.mu.RUnlock()
	if server == nil || server == old {
		t.Fatal("the proxy was not replaced")
	}
	if len(addresses) != 1 || addresses[0] != address {
		t.Fatalf("addresses = %v, want [%s]", addresses, address)
	}
}

// expire publishes a terminal failure with Attention: the listeners belong to
// the dead session and must be released, or 连接 can never bind them again.
func TestTerminalSessionFailureReleasesTheProxyListeners(t *testing.T) {
	application, old := testProxyApp(t)
	address := old.Addresses()[0]

	application.onSessionState(session.State{
		Phase:     session.PhaseFailed,
		Detail:    "会话已过期，需要重新登录",
		Attention: true,
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("the listener still holds %s: %v", address, err)
	}
	_ = listener.Close()

	application.mu.RLock()
	server := application.proxy
	addresses := application.view.Addresses
	application.mu.RUnlock()
	if server != nil || addresses != nil {
		t.Fatalf("proxy = %v addresses = %v", server, addresses)
	}
}

// The ladder's interim login failures carry no Attention and must keep the
// listeners: the app does not restart a proxy when a retry succeeds, so closing
// them here would leave a live session with no proxy at all.
func TestInterimFailureKeepsTheProxyForTheLadder(t *testing.T) {
	application, old := testProxyApp(t)

	application.onSessionState(session.State{
		Phase:  session.PhaseFailed,
		Detail: "auto-reconnect attempt 1 failed",
	})

	application.mu.RLock()
	server := application.proxy
	application.mu.RUnlock()
	if server != old {
		t.Fatal("an interim ladder failure released the proxy")
	}
}

// 断开 runs inside a frame on the UI thread. Manager.Logout waits for the login
// it just cancelled, and that login publishes through onSessionState → onMain,
// which needs the frame to finish. Joining the logout on the caller deadlocks
// the process; it has to return first.
func TestDisconnectDoesNotJoinTheSessionOnTheCaller(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	exited := make(chan struct{})
	application := &app{cfg: config.Default()}
	application.logout = func() {
		close(entered)
		<-release
		close(exited)
	}

	returned := make(chan struct{})
	go func() {
		application.disconnect()
		close(returned)
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnect did not start the session join")
	}
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		close(release)
		<-exited
		t.Fatal("disconnect joined Manager.Logout on its caller")
	}
	close(release)
	<-exited
}

// A second disconnect while a join is pending shares it, so two Logout calls
// cannot race over the login lock. A login started meanwhile waits for the join
// before Manager.Login, or the old Logout would tear down the new session.
func TestPendingLogoutIsSharedAndAwaited(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	exited := make(chan struct{})
	application := &app{cfg: config.Default()}
	application.logout = func() {
		close(entered)
		<-release
		close(exited)
	}

	first := application.beginLogout()
	<-entered
	if second := application.beginLogout(); second != first {
		t.Fatal("a second stop started another session join")
	}

	waited := make(chan error, 1)
	go func() { waited <- application.waitForLogout(context.Background()) }()
	select {
	case err := <-waited:
		t.Fatalf("waitForLogout returned %v before the join finished", err)
	case <-time.After(50 * time.Millisecond):
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := application.waitForLogout(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled wait returned %v", err)
	}

	close(release)
	if err := <-waited; err != nil {
		t.Fatalf("wait: %v", err)
	}
	<-exited
	if err := application.waitForLogout(context.Background()); err != nil {
		t.Fatalf("wait after the join: %v", err)
	}
}

// A saved rename moves the profile's stored client data, and a saved delete
// removes it. Closing the window without saving must leave the files where the
// stored settings still point, so the draft reports changes and the save applies
// them.
func TestSavingProfileChangesMovesItsClientData(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	cfg := config.Default()
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	if err := config.SaveFile(configPath, cfg); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	loaded, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	workIndex := loaded.ProfileIndex("工作")
	workPath, err := loaded.ClientDataPathFor(loaded.Profiles[workIndex])
	if err != nil {
		t.Fatalf("client data path: %v", err)
	}
	if filepath.Dir(workPath) != dir {
		t.Fatalf("client data at %q, want it beside the settings", workPath)
	}
	if err := os.WriteFile(workPath, []byte(`{"sid":"kept"}`), 0o600); err != nil {
		t.Fatalf("write client data: %v", err)
	}

	application := &app{cfg: loaded, configPath: configPath, logf: func(string, ...any) {}}
	state := gotrustui.NewSettingsState(loaded, dir)
	application.settings = state

	state.Select(workIndex)
	state.NameText = "工作二"
	application.saveSettings()

	renamed := application.currentConfig()
	newIndex := renamed.ProfileIndex("工作二")
	if newIndex < 0 {
		t.Fatalf("profiles = %v, the rename was not saved", renamed.ProfileNames())
	}
	newPath, err := renamed.ClientDataPathFor(renamed.Profiles[newIndex])
	if err != nil {
		t.Fatalf("new client data path: %v", err)
	}
	if _, err := os.Stat(workPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the old file still exists (%v)", err)
	}
	raw, err := os.ReadFile(newPath)
	if err != nil || string(raw) != `{"sid":"kept"}` {
		t.Fatalf("moved client data = %q, %v", raw, err)
	}

	// The delete reaches the filesystem only when the draft is saved.
	state.Select(newIndex)
	state.DeleteProfile()
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("an unsaved draft touched the client data: %v", err)
	}
	application.saveSettings()
	if _, err := os.Stat(newPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the deleted profile's client data remains (%v)", err)
	}
}

// A hide queued before a reopen must take neither the reopened window nor its
// fresh draft. The epoch and the draft are claimed on the main thread in one
// step, so a reopen either invalidates the queued hide or happens after it.
func TestAReopenInvalidatesAQueuedSettingsHide(t *testing.T) {
	application := &app{}
	application.settings = gotrustui.NewSettingsState(config.Default(), "")

	hideEpoch := application.bumpSettingsEpoch()
	if !application.claimSettingsHide(hideEpoch) {
		t.Fatal("a hide of the current window was dropped")
	}

	// A reopen bumps the epoch and installs a new draft before showing.
	reopenEpoch := application.bumpSettingsEpoch()
	application.settings = gotrustui.NewSettingsState(config.Default(), "")
	reopenDraft := application.settings

	if application.claimSettingsHide(hideEpoch) {
		t.Fatal("a stale hide claimed the reopened window")
	}
	application.clearSettingsDraftAfterHide(hideEpoch)
	if application.settings != reopenDraft {
		t.Fatal("a stale hide cleared the reopened draft")
	}

	// The reopen's own hide still works.
	if !application.claimSettingsHide(reopenEpoch) {
		t.Fatal("the current hide was dropped")
	}
	application.clearSettingsDraftAfterHide(reopenEpoch)
	if application.settings != nil {
		t.Fatal("a current hide left its draft behind")
	}
}

// The portal supplies the URL, so openResource is the boundary that keeps a
// file: or custom scheme away from the desktop's URL handler.
func TestOpenResourceAllowsOnlyHTTP(t *testing.T) {
	var opened []string
	application := &app{
		cfg:  config.Default(),
		logf: func(string, ...any) {},
		browser: func(url string) error {
			opened = append(opened, url)
			return nil
		},
	}

	for _, refused := range []string{"file:///etc/passwd", "smb://192.0.2.10/share", "javascript:alert(1)", "http://", "not a url"} {
		application.openResource(refused)
		if len(opened) != 0 {
			t.Fatalf("openResource(%q) reached the browser with %v", refused, opened)
		}
		if !application.view.StatusError {
			t.Fatalf("openResource(%q) did not report the refusal", refused)
		}
		application.view.StatusError = false
	}

	application.openResource("http://192.0.2.10/")
	if len(opened) != 1 || opened[0] != "http://192.0.2.10/" {
		t.Fatalf("opened = %v, want the http URL", opened)
	}
}

// A server that finishes binding after 断开 must close itself: the epoch moved,
// so installing it would hold the ports for a session that is already gone. The
// release runs between the epoch read and the installation, which is the window
// the guard exists for.
func TestAProxyBoundAfterReleaseClosesItself(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe for a free port: %v", err)
	}
	address := listener.Addr().String()
	_ = listener.Close()

	cfg := config.Default()
	cfg.Proxy.SOCKS5Bind = address
	cfg.Proxy.HTTPBind = ""
	application := &app{cfg: cfg}

	ctx, cancel := context.WithCancel(context.Background())
	server, err := gotrustproxy.New(gotrustproxy.Options{
		SOCKS5Bind: address,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, session.ErrNotOnline
		},
	})
	if err != nil {
		cancel()
		t.Fatalf("new proxy: %v", err)
	}
	if err := server.Start(ctx); err != nil {
		cancel()
		t.Fatalf("start proxy: %v", err)
	}

	application.mu.RLock()
	epoch := application.proxyEpoch
	application.mu.RUnlock()

	application.releaseProxy()

	if application.installProxy(server, cancel, epoch) {
		t.Fatal("a server bound before the release was installed")
	}
	held, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("the refused server still holds %s: %v", address, err)
	}
	_ = held.Close()

	application.mu.RLock()
	installed := application.proxy
	addresses := application.view.Addresses
	application.mu.RUnlock()
	if installed != nil || addresses != nil {
		t.Fatalf("proxy = %v addresses = %v", installed, addresses)
	}
}

// 断开 cancels the attempt. The status line must report the stop instead of
// quoting the cancelled login in red: "context canceled" is English text in a
// Chinese window, and the user caused it.
func TestAStoppedAttemptReportsCancelled(t *testing.T) {
	application := &app{cfg: config.Default()}
	application.stopping = true

	application.async(func(context.Context) error { return errors.New("log in: context canceled") })

	deadline := time.Now().Add(2 * time.Second)
	for {
		application.mu.RLock()
		status := application.view.Status
		statusError := application.view.StatusError
		application.mu.RUnlock()
		if status != "" {
			if statusError {
				t.Fatalf("status = %q, want a plain cancellation", status)
			}
			if status != "已取消" {
				t.Fatalf("status = %q, want 已取消", status)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the status line was not updated")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
