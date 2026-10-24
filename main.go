// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	zjuconnectlog "github.com/mythologyli/zju-connect/log"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/logging"
	gotrustproxy "github.com/YangtseSu/goTrust/proxy"
	"github.com/YangtseSu/goTrust/session"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

func main() {
	headless := flag.Bool("no-ui", false, "run without a window; connect, serve the proxies, and exit on SIGINT or SIGTERM")
	configPath := flag.String("config", "", "TOML settings file (default: the user config directory); the stored account material lives beside it")
	printResources := flag.Bool("print-resources", false, "print the resources cached by the last login and exit")
	flag.Parse()

	cfg, path, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("settings: %v", err)
	}

	if *printResources {
		snapshot, err := session.ReadResourceCache(config.ResourcesPath(path))
		if err != nil {
			log.Fatalf("resources: %v", err)
		}
		fmt.Println(session.FormatResources(snapshot.Resources))
		return
	}

	application := &app{
		cfg:        cfg,
		configPath: path,
		view:       gotrustui.State{Config: cfg},
	}

	var mirror io.Writer
	if *headless {
		mirror = os.Stderr
	}
	fileLog, logErr := logging.New(logging.Options{
		Verbose: cfg.Logging.Verbose,
		Mirror:  mirror,
	})
	if logErr != nil {
		// A tray app with no log file is still better than no app.
		application.logger = log.New(os.Stderr, "", log.LstdFlags)
		application.logf = application.logger.Printf
		log.SetOutput(os.Stderr)
		application.logger.Printf("logging: %v; continuing on stderr", logErr)
	} else {
		defer func() { _ = fileLog.Close() }()
		application.log = fileLog
		application.logDir = fileLog.Dir()
		application.logger = log.New(fileLog, "", log.LstdFlags)
		application.logf = application.logger.Printf
		// The aTrust client logs through the standard library's global logger,
		// so pointing that at the same writer is the whole bridge.
		log.SetOutput(fileLog)
		if cfg.Logging.Verbose {
			zjuconnectlog.EnableDebug()
		}
	}

	// A headless run cannot show a captcha, a code dialog or the method
	// chooser, so it fails with a message instead of waiting for a window.
	sessionOptions := session.Options{
		Config:            application.currentConfig,
		Logf:              application.logf,
		OnState:           application.onSessionState,
		ResourceCachePath: config.ResourcesPath(path),
	}
	if *headless {
		sessionOptions.Prompt = func(context.Context) session.Prompter {
			return cliPrompter{logger: application.logger}
		}
	} else {
		sessionOptions.Prompt = func(ctx context.Context) session.Prompter {
			return &uiPrompter{app: application, ctx: ctx}
		}
		sessionOptions.ChooseAuth = application.chooseAuth
	}
	application.manager = session.New(sessionOptions)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *headless {
		application.runHeadless(ctx)
		return
	}
	application.runUI(ctx)
}

// app ties the session, the proxies and the windows together.
type app struct {
	cfg        config.Config
	configPath string
	manager    *session.Manager
	logger     *log.Logger
	logf       func(string, ...any)
	log        *logging.Logger
	logDir     string

	mu sync.RWMutex
	// view is the main window and tray snapshot.
	view gotrustui.State
	// settings is the settings window's draft, nil while the window is closed.
	settings *gotrustui.SettingsState

	proxy  *gotrustproxy.Server
	window atomicWindow
	// settingsWindow is the separate settings window; the main window stays a
	// status and connection page.
	settingsWindow atomicWindow
	tray           atomicTray
	// proxyCancel ends the listeners of the live session. It is separate from
	// the login attempt's context, which the async helper cancels on return.
	proxyCancel context.CancelFunc
	// proxyEpoch counts listener releases. A server that finishes binding after
	// a release belongs to a session that is gone, so it closes itself instead
	// of holding the ports the next connect needs.
	proxyEpoch uint64
	// attempt is the login or action in flight, so 断开 can cancel a login
	// that is waiting on the network or on the user.
	attempt *attemptCancel
	// captcha is the reply channel of the challenge the window is showing,
	// nil when no login waits on the user. code, external and choice are the
	// same for a one-time code, an SSO callback and the method chooser.
	captcha  chan captchaResult
	code     chan codeResult
	external chan externalResult
	choice   chan choiceResult
	// browser opens a URL; nil uses the desktop's default browser. Tests
	// replace it because opening a browser is a visible side effect.
	browser func(string) error
	// logout joins the session; nil calls Manager.Logout. Tests replace it to
	// observe that a UI-thread disconnect never waits for the join.
	logout func()
	// logoutPending is the channel of the join running off the UI thread, nil
	// when none is pending. A login waits for it before Manager.Login.
	logoutPending chan struct{}
	// stopping suppresses the failure reveal while 断开 cancels a login: the
	// user asked for it, so the window must not pop up with "cancelled".
	stopping bool
	// quitting lets the main window's close handler destroy it during Quit
	// instead of hiding it in the tray.
	quitting bool
	// settingsEpoch invalidates a hide queued before the window was reopened.
	settingsEpoch int
}

// attemptCancel groups the cancel function with an identity, so a stale
// goroutine cannot clear a newer attempt's cancel.
type attemptCancel struct {
	cancel context.CancelFunc
}

func loadConfig(path string) (config.Config, string, error) {
	if path != "" {
		cfg, err := config.LoadFile(path)
		return cfg, path, err
	}
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, "", err
	}
	defaultPath, err := config.Path()
	return cfg, defaultPath, err
}

func (a *app) runHeadless(ctx context.Context) {
	if err := a.connect(ctx); err != nil {
		a.logger.Fatalf("connect: %v", err)
	}
	fmt.Println(session.FormatResources(a.manager.State().Resources))
	<-ctx.Done()
	a.disconnectNow()
}

func (a *app) runUI(ctx context.Context) {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	// Quit runs the quit sequence, so the tray disappears, the proxies close
	// and the single-instance lock is released. Exit would skip all three and
	// leave a stale socket behind.
	mygo.App.OnQuit(a.disconnect)
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.mu.Lock()
		a.quitting = true
		a.mu.Unlock()
	})

	mygo.App.WhenReady(func() {
		a.reconcileAutostart()

		hidden := a.cfg.App.StartHidden || mygo.App.WasOpenedAtLogin()
		a.openWindow(hidden)
		if !a.openTray() {
			// Without a tray icon a hidden window would leave no way back in.
			a.showWindow()
		}
		a.logf("goTrust ready; profile %s", a.cfg.ActiveProfile)

		decision := decideAutoConnect(a.cfg)
		switch {
		case decision.Connect:
			a.logf("auto-connect: on")
			a.async(a.connect)
		case decision.Reason != "":
			a.logf("auto-connect: %s", decision.Reason)
			a.setStatus(decision.Reason, false)
			a.showWindow()
		}
	})

	go func() {
		<-ctx.Done()
		mygo.App.Quit()
	}()

	if err := mygo.App.Run(); err != nil {
		a.logger.Fatalf("ui: %v", err)
	}
	a.disconnectNow()
}

// reconcileAutostart makes the desktop's entry match the settings file. The
// settings window applies a toggle immediately; this repairs a divergence
// after a crash or a manual edit.
func (a *app) reconcileAutostart() {
	if err := mygo.App.SetOpenAtLogin(a.cfg.App.Autostart); err != nil {
		a.logf("autostart: %v", err)
	}
}

func (a *app) openWindow(hidden bool) {
	if a.window.Load() != nil {
		return
	}
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:     "goTrust",
		Width:     540,
		Height:    640,
		MinWidth:  440,
		MinHeight: 400,
		Hidden:    hidden,
		Content: ui.View(func(c *ui.Context) {
			a.mu.RLock()
			state := a.view
			a.mu.RUnlock()
			gotrustui.View(&state, a.actions())(c)
		}),
	})
	// Closing the window hides it in the tray instead of destroying it; the
	// tray needs to be able to bring it back. Quit sets quitting first, so the
	// real quit still closes the window.
	window.OnClose(func(event *mygo.CloseEvent) {
		a.mu.RLock()
		quitting := a.quitting
		a.mu.RUnlock()
		if quitting {
			return
		}
		event.PreventDefault()
		window.Hide()
	})
	a.window.Store(window)
}

func (a *app) showWindow() {
	if window := a.window.Load(); window != nil {
		window.Show()
		window.Focus()
	}
}

// openSettings creates the settings window, or reveals the one already open.
// Each opening starts from the saved settings, so closing without 保存 really
// discards the edits.
func (a *app) openSettings() {
	a.mu.Lock()
	a.settingsEpoch++
	a.mu.Unlock()

	if window := a.settingsWindow.Load(); window != nil {
		// The window is hidden, not destroyed, so a close cannot tear down a
		// GTK window while its own view is still running. Each opening starts
		// from the saved settings again.
		a.resetSettingsDraft()
		window.Show()
		window.Focus()
		return
	}

	a.resetSettingsDraft()

	window := mygo.NewWindow(mygo.WindowOptions{
		Title:     "goTrust 设置",
		Width:     600,
		Height:    760,
		MinWidth:  560,
		MinHeight: 440,
		Content: ui.View(func(c *ui.Context) {
			a.mu.RLock()
			state := a.settings
			a.mu.RUnlock()
			if state != nil {
				gotrustui.SettingsView(state, a.settingsActions())(c)
			}
		}),
	})
	a.settingsWindow.Store(window)
	// A close hides the window, as it does for the main window. Destroying a
	// window from its own button action crashes GTK through mygo's callbacks;
	// hiding also lets reopening show a fresh draft.
	window.OnClose(func(event *mygo.CloseEvent) {
		a.mu.RLock()
		quitting := a.quitting
		a.mu.RUnlock()
		if quitting {
			return
		}
		event.PreventDefault()
		a.hideSettings(window)
	})
	window.OnClosed(func() {
		a.settingsWindow.Store(nil)
		a.mu.Lock()
		a.settings = nil
		a.mu.Unlock()
	})
	window.Show()
}

func (a *app) resetSettingsDraft() {
	a.mu.Lock()
	a.settings = gotrustui.NewSettingsState(a.cfg, a.logDir)
	a.mu.Unlock()
}

func (a *app) openTray() bool {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "goTrust",
		Icon:    trayIcon(),
		ToolTip: "goTrust",
	})
	if err != nil {
		a.logf("tray unavailable: %v", err)
		return false
	}
	a.tray.Store(tray)

	a.mu.RLock()
	state := a.view
	a.mu.RUnlock()
	tray.SetMenu(gotrustui.TrayMenu(&state, a.actions()))
	tray.SetToolTip(a.tooltip(state))
	tray.OnClick(a.showWindow)
	return true
}

// actions are the callbacks the main window and the tray share.
func (a *app) actions() gotrustui.Actions {
	return gotrustui.Actions{
		Login:         func() { a.async(a.connect) },
		Logout:        a.disconnect,
		Settings:      func() { go func() { mygo.RunOnMain(a.openSettings) }() },
		ShowWindow:    a.showWindow,
		SelectProfile: a.selectProfile,
		Quit:          func() { mygo.App.Quit() },

		AddCaptchaPoint: a.addCaptchaPoint,
		UndoCaptcha:     a.undoCaptchaPoint,
		ClearCaptcha:    a.clearCaptchaPoints,
		SubmitCaptcha:   a.submitCaptcha,
		CancelCaptcha:   func() { a.cancelCaptcha(errors.New("captcha cancelled")) },

		SubmitCode:     a.submitCode,
		CancelCode:     func() { a.cancelCode(errors.New("code cancelled")) },
		OpenExternal:   a.openExternal,
		SubmitExternal: a.submitExternal,
		CancelExternal: func() { a.cancelExternal(errors.New("external login cancelled")) },
		ChooseAuth:     a.chooseAuthIndex,
		CancelChoice:   func() { a.cancelChoice(errors.New("authentication method cancelled")) },

		OpenResource: a.openResource,
		CopyResource: a.copyResource,
	}
}

// openResource opens a granted application or address in the browser. A
// failure is reported in the window; the URL is still visible there.
func (a *app) openResource(url string) {
	if err := a.openBrowser(url); err != nil {
		a.setStatusError("打开失败: " + err.Error())
		a.logf("open resource %s: %v", url, err)
	}
}

// copyResource puts a resource value on the clipboard and confirms it in the
// status line.
func (a *app) copyResource(value string) {
	if err := mygo.Clipboard.Write(transfer.TextData(value)); err != nil {
		a.setStatusError("复制失败: " + err.Error())
		a.logf("copy resource: %v", err)
		return
	}
	a.setStatus("已复制", false)
}

// settingsActions are the settings window's callbacks.
func (a *app) settingsActions() gotrustui.SettingsActions {
	mutate := func(fn func(*gotrustui.SettingsState)) {
		a.mu.RLock()
		state := a.settings
		a.mu.RUnlock()
		if state == nil {
			return
		}
		fn(state)
		a.invalidateSettings()
	}
	return gotrustui.SettingsActions{
		Save:  a.saveSettings,
		Close: a.closeSettings,
		NewProfile: func() {
			mutate(func(state *gotrustui.SettingsState) { state.AddProfile() })
		},
		DuplicateProfile: func() {
			mutate(func(state *gotrustui.SettingsState) { state.DuplicateProfile() })
		},
		DeleteProfile: func() {
			mutate(func(state *gotrustui.SettingsState) { state.DeleteProfile() })
		},
		SelectProfile: func(index int) {
			mutate(func(state *gotrustui.SettingsState) { state.Select(index) })
		},
		SetAutostart: a.setAutostart,
		SetVerbose:   a.setVerbose,
		OpenLogs:     a.openLogs,
		DiscoverAuth: a.discoverAuth,
		SelectAuth: func(index int) {
			mutate(func(state *gotrustui.SettingsState) { state.SelectAuth(index) })
		},
	}
}

func (a *app) closeSettings() {
	if window := a.settingsWindow.Load(); window != nil {
		a.hideSettings(window)
		return
	}
	a.mu.Lock()
	a.settings = nil
	a.mu.Unlock()
}

// hideSettings hides the settings window after the current frame. Hiding it
// from the button action itself runs surface callbacks while the frame still
// builds, which crashed GTK's GL teardown. The epoch stops a hide queued
// before a reopen from taking the new window away.
func (a *app) hideSettings(window *mygo.Window) {
	a.mu.Lock()
	a.settingsEpoch++
	epoch := a.settingsEpoch
	a.mu.Unlock()

	go func() {
		a.mu.Lock()
		stale := a.settingsEpoch != epoch
		a.mu.Unlock()
		if stale {
			return
		}
		window.Hide()
		a.mu.Lock()
		if a.settingsEpoch == epoch {
			a.settings = nil
		}
		a.mu.Unlock()
	}()
}

// async runs work off the UI thread and reports the outcome in the status line.
func (a *app) async(fn func(context.Context) error) {
	go func() {
		// The login may wait on a person reading a captcha, so the deadline
		// covers more than the network work.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		token := &attemptCancel{cancel: cancel}
		a.mu.Lock()
		a.attempt = token
		a.mu.Unlock()
		defer func() {
			cancel()
			a.mu.Lock()
			if a.attempt == token {
				a.attempt = nil
			}
			a.mu.Unlock()
		}()

		a.setStatus("", true)
		err := fn(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				a.setStatus("已取消", false)
				return
			}
			a.setStatusError(err.Error())
			return
		}
		a.setStatus("", false)
	}()
}

func (a *app) connect(ctx context.Context) error {
	// 断开 joins the session on another goroutine. Building a new session
	// before that join finishes would let the old Logout tear it down.
	if err := a.waitForLogout(ctx); err != nil {
		return err
	}
	if err := a.manager.Login(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		a.manager.Logout()
		return err
	}

	// ctx is the attempt's deadline, which async cancels as soon as this
	// returns. The listeners must live until the session ends.
	if err := a.restartProxy(); err != nil {
		a.manager.Logout()
		return err
	}
	return nil
}

// onSessionState mirrors the session into the view and refreshes what is on
// screen. It runs on the goroutine that changed the session.
func (a *app) onSessionState(state session.State) {
	a.mu.Lock()
	a.view.Session = state
	stopping := a.stopping
	a.mu.Unlock()

	// A terminal failure means the session will not come back on its own: the
	// client is torn down and 连接 is the user's next step. Its listeners would
	// keep the configured ports, so that next connect could not bind them. The
	// ladder's interim failures carry no Attention and keep the listeners: a
	// retry that succeeds reuses them, and the app does not restart a proxy
	// from inside the ladder.
	if state.Phase == session.PhaseFailed && state.Attention != "" && !stopping {
		a.releaseProxy()
	}

	if window := a.window.Load(); window != nil {
		window.Invalidate()
	}
	a.refreshTray()

	// A login that fails while the window is hidden must not fail silently, and
	// neither may a session that expired or a ladder that stopped at a prompt.
	if (state.Phase == session.PhaseFailed || state.Attention != "") && !stopping {
		a.showWindow()
	}
}

// restartProxy starts the listeners for the login that just succeeded. The
// previous session's server is closed first: its ports are the ones the new
// bind needs, and a bind that fails here leaves every retry failing with
// "address already in use".
func (a *app) restartProxy() error {
	a.releaseProxy()
	proxyCtx, cancel := context.WithCancel(context.Background())
	if err := a.startProxy(proxyCtx, cancel); err != nil {
		cancel()
		return err
	}
	return nil
}

// logoutSession joins the session. Tests replace a.logout to hold the join
// open, which is how the deadlock regression test observes the caller.
func (a *app) logoutSession() {
	if a.logout != nil {
		a.logout()
		return
	}
	a.manager.Logout()
}

// releaseProxy closes the listeners the window still names and forgets them.
// It is safe to call with no session: releasing again only bumps the epoch, and
// a server that was still binding sees the change and closes itself.
func (a *app) releaseProxy() {
	a.mu.Lock()
	a.proxyEpoch++
	server := a.proxy
	cancel := a.proxyCancel
	a.proxy = nil
	a.proxyCancel = nil
	a.view.Addresses = nil
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if server != nil {
		_ = server.Close()
	}
}

func (a *app) startProxy(ctx context.Context, cancel context.CancelFunc) error {
	socks5, httpBind := a.currentConfig().ProxyAddresses()

	// The resolver resolves names inside the tunnel: a SOCKS5 client sends the
	// hostname, and the system resolver knows nothing about intranet names. The
	// manager reads its resolver per request: an auto-reconnect replaces it
	// while these listeners stay open.
	a.mu.RLock()
	epoch := a.proxyEpoch
	a.mu.RUnlock()

	server, err := gotrustproxy.New(gotrustproxy.Options{
		SOCKS5Bind: socks5,
		HTTPBind:   httpBind,
		Dial:       a.manager.Dial,
		Resolver:   a.manager,
		Logf:       a.logf,
	})
	if err != nil {
		return err
	}
	if err := server.Start(ctx); err != nil {
		return err
	}

	a.mu.Lock()
	if a.proxyEpoch != epoch {
		a.mu.Unlock()
		// A terminal failure or a disconnect closed the listeners while this
		// server was binding. Installing it would hold the ports for a session
		// that no longer exists.
		cancel()
		_ = server.Close()
		return nil
	}
	a.proxy = server
	a.proxyCancel = cancel
	a.view.Addresses = server.Addresses()
	a.mu.Unlock()

	a.invalidate()
	a.refreshTray()
	return nil
}

// disconnect stops the session for the UI: 断开, a profile switch and quit all
// call it from a frame or a tray callback on the main thread. It releases the
// prompts, the login attempt and the listeners synchronously, then joins
// Manager.Logout on another goroutine. Logout waits for the login that the
// cancellation just woke, and that login publishes through onSessionState →
// onMain, which needs the main thread this frame holds: joining it here
// deadlocks the process.
func (a *app) disconnect() {
	a.stopSession()
	_ = a.beginLogout()
}

// disconnectNow is the final teardown, run when no frame waits on the main
// loop: --no-ui, or after App.Run returned. onMain drops its callbacks then, so
// joining the session here cannot deadlock.
func (a *app) disconnectNow() {
	a.stopSession()
	a.logoutSession()

	a.mu.Lock()
	a.stopping = false
	a.mu.Unlock()

	a.invalidate()
	a.refreshTray()
}

// stopSession releases the user-facing state without joining the login.
func (a *app) stopSession() {
	// Mark the stop first: releasing a prompt wakes the login goroutine, and a
	// failure it reports to a user who asked to disconnect must not grab the
	// window.
	a.mu.Lock()
	a.stopping = true
	a.mu.Unlock()
	a.cancelPrompts(errors.New("login cancelled"))

	a.mu.Lock()
	attempt := a.attempt
	a.attempt = nil
	a.mu.Unlock()

	if attempt != nil {
		attempt.cancel()
	}
	a.releaseProxy()
	a.invalidate()
	a.refreshTray()
}

// beginLogout joins the session on another goroutine and returns the channel
// that closes when it finished. A join already running is shared: it tears down
// whatever session exists when it gets the login lock. A login that starts
// meanwhile waits for the channel, so that Logout cannot kill the session the
// login just built.
func (a *app) beginLogout() <-chan struct{} {
	a.mu.Lock()
	if pending := a.logoutPending; pending != nil {
		a.mu.Unlock()
		return pending
	}
	pending := make(chan struct{})
	a.logoutPending = pending
	a.stopping = true
	a.mu.Unlock()

	go func() {
		a.logoutSession()

		a.mu.Lock()
		a.logoutPending = nil
		a.stopping = false
		a.mu.Unlock()

		close(pending)
		a.invalidate()
		a.refreshTray()
	}()
	return pending
}

// waitForLogout blocks until a join started by disconnect finished. The caller
// runs off the UI thread, so waiting here cannot block onMain.
func (a *app) waitForLogout(ctx context.Context) error {
	a.mu.RLock()
	pending := a.logoutPending
	a.mu.RUnlock()
	if pending == nil {
		return nil
	}
	select {
	case <-pending:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// saveSettings persists the settings draft. Proxy binds and keep-alive take
// effect on the next connect; credentials on the next login; a profile switch
// disconnects first.
func (a *app) saveSettings() {
	a.mu.RLock()
	state := a.settings
	a.mu.RUnlock()
	if state == nil {
		return
	}

	cfg, err := state.Apply()
	if err != nil {
		state.Status = err.Error()
		state.StatusError = true
		a.invalidateSettings()
		return
	}
	if err := config.SaveFile(a.configPath, cfgForDisk(cfg)); err != nil {
		state.Status = "保存失败: " + err.Error()
		state.StatusError = true
		a.invalidateSettings()
		a.logf("save settings: %v", err)
		return
	}

	// The file now names the profiles the draft describes, so the stored account
	// material may follow a rename or a delete. Doing this before the save would
	// move files under a configuration that still names the old profile.
	a.applyClientDataChanges(cfg, state.ClientDataChanges())

	previous := a.currentConfig()
	a.applyConfig(cfg)

	// Reflect the normalised values back into the form.
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	if settings != nil {
		settings.Reload(cfg)
		settings.Status = "设置已保存"
		settings.StatusError = false
	}

	a.reconcileProfileSwitch(previous, cfg)
	a.invalidateSettings()
}

// selectProfile switches accounts from the tray or the settings window. One
// live session at a time: the old one disconnects first.
func (a *app) selectProfile(name string) {
	cfg := a.currentConfig()
	if cfg.ActiveProfile == name {
		return
	}
	if cfg.ProfileIndex(name) < 0 {
		a.setStatusError(fmt.Sprintf("配置 %q 不存在", name))
		return
	}

	a.mu.RLock()
	online := sessionActive(a.view.Session.Phase)
	a.mu.RUnlock()

	cfg.ActiveProfile = name
	if err := config.SaveFile(a.configPath, cfgForDisk(cfg)); err != nil {
		a.setStatusError("切换配置失败: " + err.Error())
		a.logf("switch profile: %v", err)
		return
	}

	a.applyConfig(cfg)
	if online {
		a.disconnect()
		if cfg.App.AutoConnect {
			a.async(a.connect)
		}
	}

	// Keep an open settings window in step with the switch.
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	if settings != nil {
		settings.Reload(cfg)
		settings.Status = "配置已切换，未保存的修改已丢弃"
		a.invalidateSettings()
	}
}

// reconcileProfileSwitch handles a save that changed active_profile: a live
// session belongs to the old account and must go first.
func (a *app) reconcileProfileSwitch(previous, next config.Config) {
	if previous.ActiveProfile == next.ActiveProfile {
		return
	}
	a.mu.RLock()
	online := sessionActive(a.view.Session.Phase)
	a.mu.RUnlock()
	if online {
		a.disconnect()
		if next.App.AutoConnect {
			a.async(a.connect)
		}
	}
}

// applyClientDataChanges moves or removes the stored account material the saved
// draft changed. It runs only after the settings file was written: a window
// closed without saving must leave every file where the stored profiles still
// point. A profile with a custom client data path owns that file, so the user's
// choice is never moved or deleted here.
func (a *app) applyClientDataChanges(cfg config.Config, changes []gotrustui.ClientDataChange) {
	for _, change := range changes {
		if strings.TrimSpace(change.From.Debug.ClientData) != "" ||
			strings.TrimSpace(change.To.Debug.ClientData) != "" {
			continue
		}
		from, err := cfg.ClientDataPathFor(change.From)
		if err != nil {
			a.logf("locate the client data of %q: %v", change.From.Name, err)
			continue
		}

		if change.To.Name == "" {
			if err := os.Remove(from); err != nil && !errors.Is(err, os.ErrNotExist) {
				a.logf("remove the client data of %q: %v", change.From.Name, err)
			}
			continue
		}

		to, err := cfg.ClientDataPathFor(change.To)
		if err != nil {
			a.logf("locate the client data of %q: %v", change.To.Name, err)
			continue
		}
		if from == to {
			continue
		}
		if _, statErr := os.Stat(from); errors.Is(statErr, os.ErrNotExist) {
			// Nothing was stored for this account yet: a rename has no file to
			// move, and the next login creates one under the new name.
			continue
		}
		if err := os.Rename(from, to); err != nil {
			// A file left behind by an earlier profile with the target name
			// would block the move where rename does not replace; that profile
			// is gone from the configuration, so its stale material goes.
			if removeErr := os.Remove(to); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				a.logf("replace the client data of %q: %v", change.To.Name, removeErr)
				continue
			}
			if err := os.Rename(from, to); err != nil {
				a.logf("move the client data of %q to %q: %v", change.From.Name, change.To.Name, err)
			}
		}
	}
}

// setAutostart changes the desktop entry immediately and persists the flag.
func (a *app) setAutostart(on bool) {
	a.mu.RLock()
	state := a.settings
	a.mu.RUnlock()

	if err := mygo.App.SetOpenAtLogin(on); err != nil {
		if state != nil {
			state.Status = "开机自启设置失败: " + err.Error()
			state.StatusError = true
			a.invalidateSettings()
		}
		a.logf("autostart: %v", err)
		return
	}
	a.persistFlag(func(cfg *config.Config) { cfg.App.Autostart = on })
	if state != nil {
		state.Status = "开机自启已" + map[bool]string{true: "开启", false: "关闭"}[on]
		state.StatusError = false
		a.invalidateSettings()
	}
}

// setVerbose changes the running log level immediately and persists it.
func (a *app) setVerbose(on bool) {
	if a.log != nil {
		a.log.SetVerbose(on)
	}
	if on {
		zjuconnectlog.EnableDebug()
	} else {
		zjuconnectlog.DisableDebug()
	}
	a.persistFlag(func(cfg *config.Config) { cfg.Logging.Verbose = on })
}

func (a *app) persistFlag(mutate func(*config.Config)) {
	cfg := a.currentConfig()
	mutate(&cfg)
	if err := config.SaveFile(a.configPath, cfgForDisk(cfg)); err != nil {
		a.setStatusError("保存设置失败: " + err.Error())
		a.logf("save settings: %v", err)
		return
	}
	a.applyConfig(cfg)
}

func (a *app) openLogs() {
	a.mu.RLock()
	state := a.settings
	a.mu.RUnlock()
	if a.logDir == "" {
		if state != nil {
			state.Status = "日志目录不可用"
			state.StatusError = true
			a.invalidateSettings()
		}
		return
	}
	if err := mygo.Shell.OpenPath(a.logDir); err != nil {
		if state != nil {
			state.Status = "打开日志目录失败: " + err.Error()
			state.StatusError = true
			a.invalidateSettings()
		}
		a.logf("open logs: %v", err)
	}
}

// discoverAuth asks the gateway in the settings draft for its authentication
// methods. The request runs off the UI thread; the answer is applied on it, so
// the view never reads a half-updated draft.
func (a *app) discoverAuth() {
	a.mu.RLock()
	state := a.settings
	a.mu.RUnlock()
	if state == nil {
		return
	}

	target, err := state.Target()
	if err != nil {
		state.Status = err.Error()
		state.StatusError = true
		a.invalidateSettings()
		return
	}

	key := keyForTarget(target)
	state.Discovering = true
	state.Methods = nil
	state.MethodsFrom = ""
	state.Status = "正在向网关查询认证方式…"
	a.invalidateSettings()

	go func() {
		// The pre-login request has a 20 s transport timeout of its own; the
		// context covers the underlay dial and a hung TLS handshake.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		methods, discoverErr := session.DiscoverAuthMethods(ctx, target)

		mygo.RunOnMain(func() {
			a.mu.RLock()
			current := a.settings
			a.mu.RUnlock()
			if current != state {
				return
			}
			switch {
			case discoverErr != nil:
				state.SetMethods(nil, key, "获取认证方式失败: "+discoverErr.Error(), true)
				a.logf("discover auth methods: %v", discoverErr)
			case len(methods) == 0:
				state.SetMethods(nil, key, "网关没有提供认证方式（可能已经登录）", false)
			default:
				state.SetMethods(methods, key, fmt.Sprintf("已获取 %d 种认证方式", len(methods)), false)
				a.logf("gateway offers %d authentication method(s)", len(methods))
			}
			a.invalidateSettings()
		})
	}()
}

// keyForTarget names the gateway a discovery ran against, in the same form the
// settings draft uses.
func keyForTarget(profile config.Profile) string {
	return profile.ServerAddress() + "|" + strconv.Itoa(profile.Server.Port)
}

// applyConfig makes cfg the settings in effect. It does not touch a live
// session: the apply rules are explicit at the call sites.
func (a *app) applyConfig(cfg config.Config) {
	a.mu.Lock()
	a.cfg = cfg
	a.view.Config = cfg
	a.mu.Unlock()

	a.refreshTray()
	a.invalidate()
}

// currentConfig returns the settings a login or a listener should use.
func (a *app) currentConfig() config.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// mutableConfig returns a private copy of the settings for a handler that edits
// a profile. currentConfig copies the struct but shares the Profiles array with
// the live configuration: writing a field through that copy would race with a
// login reading Active() on another goroutine, and the edit would survive a
// failed save.
func (a *app) mutableConfig() config.Config {
	return a.currentConfig().Clone()
}

// cfgForDisk clears a password the user chose not to store. The in-memory
// configuration keeps it for this run.
func cfgForDisk(cfg config.Config) config.Config {
	disk := cfg.Clone()
	for index := range disk.Profiles {
		if !disk.Profiles[index].Auth.StorePass {
			disk.Profiles[index].Auth.Password = ""
		}
	}
	return disk
}

// sessionActive reports whether the phase has a session that 断开 must end:
// connected, logging in, or waiting out an auto-reconnect.
func sessionActive(phase session.Phase) bool {
	switch phase {
	case session.PhaseOnline, session.PhaseConnecting, session.PhaseReconnecting:
		return true
	default:
		return false
	}
}

// autoConnectDecision says whether startup may log in without asking.
type autoConnectDecision struct {
	Connect bool
	Reason  string
}

func decideAutoConnect(cfg config.Config) autoConnectDecision {
	if !cfg.App.AutoConnect {
		return autoConnectDecision{}
	}
	profile := cfg.Active()
	if profile.ServerAddress() == "" {
		return autoConnectDecision{Reason: "自动连接未执行：请先在设置中填写网关地址"}
	}
	if strings.TrimSpace(profile.Auth.Username) == "" {
		return autoConnectDecision{Reason: "自动连接未执行：请先在设置中填写用户名"}
	}
	// A stored session is not a login credential: the portal ends a session
	// with its connection, so startup needs a saved password to reconnect.
	if profile.Auth.StorePass && profile.Auth.Password != "" {
		return autoConnectDecision{Connect: true}
	}
	return autoConnectDecision{Reason: "自动连接未执行：没有保存的密码，请先在设置中保存密码"}
}

func (a *app) setStatus(message string, busy bool) {
	a.mu.Lock()
	a.view.Status = message
	a.view.StatusError = false
	a.view.Busy = busy
	a.mu.Unlock()

	a.invalidate()
	// The tray disables 连接 while an idle-phase action is still running.
	a.refreshTray()
}

// setStatusError reports a failed action in the window: red, and never with
// the busy indicator, which would suggest the work is still going on.
func (a *app) setStatusError(message string) {
	a.mu.Lock()
	a.view.Status = message
	a.view.StatusError = true
	a.view.Busy = false
	a.mu.Unlock()

	a.invalidate()
	a.refreshTray()
}

func (a *app) invalidate() {
	if window := a.window.Load(); window != nil {
		window.Invalidate()
	}
}

func (a *app) invalidateSettings() {
	if window := a.settingsWindow.Load(); window != nil {
		window.Invalidate()
	}
}

func (a *app) refreshTray() {
	tray := a.tray.Load()
	if tray == nil {
		return
	}
	a.mu.RLock()
	state := a.view
	a.mu.RUnlock()
	tray.SetMenu(gotrustui.TrayMenu(&state, a.actions()))
	tray.SetToolTip(a.tooltip(state))
}

func (a *app) tooltip(state gotrustui.State) string {
	tooltip := "goTrust: " + gotrustui.TrayStatus(state.Session)
	if summary := gotrustui.KeepAliveSummary(state.Session.KeepAlive); summary != "" {
		tooltip += "\n保活: " + summary
	}
	return tooltip
}
