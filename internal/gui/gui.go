// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package gui is the tray front-end: the windows, the tray icon, the event
// loop and the mapping from session state to what a person sees.
//
// Every operation it performs goes through core/session, core/config,
// core/proxy or core/logging. It never touches the protocol layer under
// core/atrust, so the same layer can serve a front-end without a toolkit;
// internal/guards enforces that.
package gui

import (
	"context"
	"errors"
	"fmt"
	"log"
	neturl "net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/handoff"
	"github.com/YangtseSu/goTrust/core/logging"
	"github.com/YangtseSu/goTrust/core/profilelock"
	gotrustproxy "github.com/YangtseSu/goTrust/core/proxy"
	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/data"
	"github.com/YangtseSu/goTrust/locales"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

// Run starts the tray app against the settings file at configPath, or the
// default location when it is empty, and blocks until the app quits. The
// stored account material lives beside the settings file.
func Run(configPath string) error {
	cfg, path, err := config.LoadOrCreate(configPath)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}

	application := &app{
		cfg:        cfg,
		configPath: path,
		view:       gotrustui.State{Config: cfg},
	}

	fileLog, logErr := logging.New(logging.Options{
		Verbose: cfg.Logging.Verbose,
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
			session.SetProtocolDebug(true)
		}
	}

	// One settings draft lives as long as the window: a page switch or hiding
	// the window keeps the edits, and the pages render it in place.
	application.settings = gotrustui.NewSettingsState(cfg, application.logDir)
	if fileLog != nil {
		application.settings.LogPath = fileLog.Path()
	}
	application.mu.Lock()
	application.view.Settings = application.settings
	application.mu.Unlock()

	// The prompts belong to the window: a click captcha needs an image, an SSO
	// login needs a pasted callback URL. The headless front-end lives in
	// cmd/gotrust-cli, so this binary has exactly one set of answers.
	sessionOptions := session.Options{
		Config:            application.currentConfig,
		Logf:              application.logf,
		OnState:           application.onSessionState,
		ResourceCachePath: config.ResourcesPath(path),
		Prompt: func(ctx context.Context) session.Prompter {
			return &uiPrompter{app: application, ctx: ctx}
		},
		ChooseAuth:        application.chooseAuth,
		ChangePassword:    application.changePasswordPrompt,
		ConfirmDeviceBind: application.confirmDeviceBind,
	}
	sessionOptions.OnPasswordChanged = application.storeChangedPassword
	application.manager = session.New(sessionOptions)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if listen := strings.TrimSpace(cfg.Handoff.Listen); listen != "" {
		pac := handoff.New(listen, application.pacSnapshot, application.logf)
		if err := pac.Start(ctx); err != nil {
			application.logf("handoff: %v", err)
		} else {
			application.mu.Lock()
			application.view.PACURL = pac.URL()
			application.mu.Unlock()
			application.logf("PAC at %s", pac.URL())
			defer func() { _ = pac.Close() }()
		}
	}

	return application.runUI(ctx)
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
	// view is the window and tray snapshot.
	view gotrustui.State
	// settings is the window's settings draft. It is created at startup and
	// kept for the life of the process, so hiding the window or switching a
	// page keeps the edits; only 保存, 放弃, a profile switch and a login that
	// saves the draft reload it.
	settings *gotrustui.SettingsState

	proxy *gotrustproxy.Server
	// profileLock is the live login's claim on its profile. The CLI takes the
	// same lock, so two processes cannot share one profile's client data and
	// tunnel identity.
	profileLock *profilelock.Lock
	window      atomicWindow
	tray        atomicTray
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
	captcha    chan captchaResult
	code       chan codeResult
	external   chan externalResult
	choice     chan choiceResult
	password   chan passwordResult
	deviceBind chan deviceBindResult
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
	// localeChoice is the language picker's entry: 0 follows the
	// desktop, 1 is Chinese, 2 is English. mygo remembers the language itself;
	// this only keeps the picker in step for the rest of the run.
	localeChoice int
}

// attemptCancel groups the cancel function with an identity, so a stale
// goroutine cannot clear a newer attempt's cancel.
type attemptCancel struct {
	cancel context.CancelFunc
}

func (a *app) runUI(ctx context.Context) error {
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
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

	// The tray and the window are rebuilt from the catalogs; a language change
	// only has to invalidate them.
	mygo.App.OnLocaleChanged(func(string) {
		a.refreshTray()
		a.invalidate()
	})

	mygo.App.WhenReady(func() {
		a.mu.Lock()
		a.localeChoice = initialLocaleChoice()
		if a.settings != nil {
			a.settings.LocaleIndex = a.localeChoice
		}
		a.mu.Unlock()
		a.reconcileAutostart()

		hidden := a.cfg.App.StartHidden || mygo.App.WasOpenedAtLogin()
		a.openWindow(hidden)
		if !a.openTray() {
			// Without a tray icon a hidden window would leave no way back in.
			a.showWindow()
		}
		a.logf("goTrust ready; profile %s", a.cfg.ActiveProfile)

		// 日志 shows the file as it grows; the tailer runs for the life of
		// the app, so a page opened later already has the lines behind it.
		if logFile := a.log; logFile != nil {
			tail := &logTail{path: logFile.Path(), limit: gotrustui.LogTailLines, emit: a.setLogLines}
			go tail.run(ctx)
		}

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
		return fmt.Errorf("ui: %w", err)
	}
	a.disconnectNow()
	return nil
}

// reconcileAutostart makes the desktop's entry match the settings file. The
// startup page applies a toggle immediately; this repairs a divergence after a
// crash or a manual edit.
func (a *app) reconcileAutostart() {
	if err := mygo.App.SetOpenAtLogin(a.cfg.App.Autostart); err != nil {
		a.logf("autostart: %v", err)
	}
}

// The window's size and the page minimum the views are drawn at. GTK applies
// the minimum hint to the whole surface, its CSD shadow margins included, so
// the hint carries that margin on top of the page minimum.
const (
	windowWidth        = 900
	windowHeight       = 740
	windowMinWidth     = 720
	windowMinHeight    = 560
	windowShadowWidth  = 52
	windowShadowHeight = 52
)

func (a *app) openWindow(hidden bool) {
	if a.window.Load() != nil {
		return
	}
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:             "goTrust",
		Width:             windowWidth,
		Height:            windowHeight,
		MinWidth:          windowMinWidth + windowShadowWidth,
		MinHeight:         windowMinHeight + windowShadowHeight,
		TitleBarStyle:     mygo.TitleBarHidden,
		TitleBarHeight:    gotrustui.TitleBarHeight,
		HideWindowButtons: true,
		Transparent:       true,
		Hidden:            hidden,
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

// clearLogs empties the log file and its rotations; the page shows the empty
// tail at once and the tailer restarts on the truncation.
func (a *app) clearLogs() {
	if a.log == nil {
		return
	}
	if err := a.log.Clear(); err != nil {
		a.setStatusError(locales.LogClearFailed(err.Error()))
		a.logf("clear log: %v", err)
		return
	}
	a.mu.Lock()
	if settings := a.settings; settings != nil {
		settings.LogLines = nil
	}
	a.mu.Unlock()
	a.setStatus(locales.LogsCleared(), false)
	a.invalidate()
}

// toggleTheme switches the whole application between the light and the dark
// appearance; the next frame paints the other palette.
func (a *app) toggleTheme() {
	if mygo.Theme.IsDark() {
		mygo.Theme.SetSource(mygo.ThemeLight)
	} else {
		mygo.Theme.SetSource(mygo.ThemeDark)
	}
	a.invalidate()
}

// minimizeWindow and toggleMaximize drive the frame from the title strip's own
// controls; closeWindow goes through OnClose, which hides the window.
func (a *app) minimizeWindow() {
	if window := a.window.Load(); window != nil {
		window.Minimize()
	}
}

func (a *app) toggleMaximize() {
	window := a.window.Load()
	if window == nil {
		return
	}
	if window.IsMaximized() {
		window.Unmaximize()
		return
	}
	window.Maximize()
}

func (a *app) closeWindow() {
	if window := a.window.Load(); window != nil {
		window.Close()
	}
}

// showWindow reveals the window on the page it already shows. The tray icon
// uses it, so a click never loses the page the user was reading.
func (a *app) showWindow() {
	if window := a.window.Load(); window != nil {
		window.Show()
		window.Focus()
	}
}

// showPage selects a page, then shows and focuses the window. The sidebar and
// the tray's 打开仪表盘 use it.
func (a *app) showPage(page gotrustui.Page) {
	a.mu.Lock()
	a.view.Page = page
	a.mu.Unlock()
	if page == gotrustui.PageTerminals {
		// The list is the portal's, not the draft's: read it when the page
		// opens rather than on every frame of the window.
		a.refreshSettingsTerminals()
	}
	a.invalidate()
	a.showWindow()
}

func (a *app) openTray() bool {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "goTrust",
		Icon:    data.TrayPNG(),
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
		Login:          a.startLogin,
		TrayLogin:      a.startSavedLogin,
		Logout:         a.disconnect,
		ShowPage:       a.showPage,
		MinimizeWindow: a.minimizeWindow,
		ToggleMaximize: a.toggleMaximize,
		CloseWindow:    a.closeWindow,
		ToggleTheme:    a.toggleTheme,
		SelectProfile:  a.selectProfile,
		Quit:           func() { mygo.App.Quit() },
		Settings:       a.settingsActions(),

		AddCaptchaPoint: a.addCaptchaPoint,
		UndoCaptcha:     a.undoCaptchaPoint,
		ClearCaptcha:    a.clearCaptchaPoints,
		SubmitCaptcha:   a.submitCaptcha,
		CancelCaptcha:   func() { a.cancelCaptcha(errors.New("captcha cancelled")) },

		SubmitCode:      a.submitCode,
		CancelCode:      func() { a.cancelCode(errors.New("code cancelled")) },
		OpenExternal:    a.openExternal,
		SubmitExternal:  a.submitExternal,
		CancelExternal:  func() { a.cancelExternal(errors.New("external login cancelled")) },
		ChooseAuth:      a.chooseAuthIndex,
		CancelChoice:    func() { a.cancelChoice(errors.New("authentication method cancelled")) },
		SubmitPassword:  a.submitPassword,
		CancelPassword:  func() { a.cancelPassword(errors.New("password change cancelled")) },
		AllowDeviceBind: a.allowDeviceBind,
		CancelDeviceBind: func() {
			a.cancelDeviceBind(errors.New("device binding cancelled"))
		},

		OpenResource: a.openURL,
		CopyResource: a.copyResource,
		CopyPAC:      a.copyPAC,
		CopyClash:    a.copyClash,
	}
}

// pacSnapshot builds the PAC from the settings and whatever the session holds
// now. The endpoint calls this per request, so a reconnect needs no restart.
func (a *app) pacSnapshot() handoff.Snapshot {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	return handoff.FromState(cfg, a.manager.State())
}

// copyPAC puts the PAC URL on the clipboard. The URL is fixed for the life of
// the process, so a user points ZeroOmega or the system proxy at it once.
func (a *app) copyPAC() {
	a.mu.RLock()
	url := a.view.PACURL
	a.mu.RUnlock()
	if url == "" {
		a.setStatusError(locales.PacOff())
		return
	}
	a.copyResource(url)
}

// copyClash copies one of the three Clash renderings built from the live grant.
// The CLI prints the same three from its cache.
func (a *app) copyClash(format gotrustui.ClashFormat) {
	snapshot := a.pacSnapshot()
	var (
		text string
		err  error
	)
	switch format {
	case gotrustui.ClashPlain:
		text, err = snapshot.Clash()
	case gotrustui.ClashScript:
		text, err = snapshot.ClashScript()
	case gotrustui.ClashConfig:
		text, err = snapshot.ClashConfig()
	default:
		a.setStatusError(locales.ClashUnknown())
		return
	}
	if err != nil {
		a.setStatusError(locales.ClashFailed(err.Error()))
		return
	}
	a.copyResource(text)
}

// openURL opens an address in the browser: a granted application, the project
// page or the license. A failure is reported in the window. Only http and
// https reach the desktop's URL handler: a portal-supplied file:, smb: or
// custom scheme must not launch a local application.
func (a *app) openURL(rawURL string) {
	parsed, err := neturl.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		a.setStatusError(locales.OpenUnsupported())
		a.logf("refuse to open %q: only http and https URLs are allowed", rawURL)
		return
	}
	if err := a.openBrowser(rawURL); err != nil {
		a.setStatusError(locales.OpenFailed(err.Error()))
		a.logf("open resource %s: %v", rawURL, err)
	}
}

// copyResource puts a resource value on the clipboard and confirms it in the
// status line.
func (a *app) copyResource(value string) {
	if err := mygo.Clipboard.Write(transfer.TextData(value)); err != nil {
		a.setStatusError(locales.CopyFailed(err.Error()))
		a.logf("copy resource: %v", err)
		return
	}
	a.setStatus(locales.Copied(), false)
}

// copyLogs puts the visible log lines on the clipboard. 日志 disables 复制 when
// there is nothing to copy, so an empty text here is a stale click.
func (a *app) copyLogs(text string) {
	if strings.TrimSpace(text) == "" {
		a.setStatusError(locales.LogEmpty())
		return
	}
	a.copyResource(text)
}

// settingsActions are the draft's callbacks.
func (a *app) settingsActions() gotrustui.SettingsActions {
	mutate := func(fn func(*gotrustui.SettingsState)) {
		a.mu.RLock()
		state := a.settings
		a.mu.RUnlock()
		if state == nil {
			return
		}
		fn(state)
		a.invalidate()
	}
	return gotrustui.SettingsActions{
		Save: a.saveSettings,
		Discard: func() {
			cfg := a.currentConfig()
			mutate(func(state *gotrustui.SettingsState) {
				state.Reload(cfg)
				state.SetStatus(locales.SettingsDiscarded(), false)
			})
		},
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
		ClearLogs:    a.clearLogs,
		OpenLogs:     a.openLogs,
		OpenConfig:   a.openConfig,
		CopyLogs:     a.copyLogs,
		OpenURL:      a.openURL,
		DiscoverAuth: a.discoverAuth,
		SelectAuth: func(index int) {
			mutate(func(state *gotrustui.SettingsState) { state.SelectAuth(index) })
		},
		OpenPassword:     a.openSettingsPassword,
		SubmitPassword:   a.submitSettingsPassword,
		CancelPassword:   a.cancelSettingsPassword,
		RefreshTerminals: a.refreshSettingsTerminals,
		SetTrusted:       a.setSettingsTrusted,
		SetLocale:        a.setLocale,
	}
}

// setLocale shows the app in the tag's language, "" to follow the desktop. The
// picker's index is the choice itself, not a reading of the resulting language,
// so 跟随系统 stays selected while it follows.
func (a *app) setLocale(tag string) {
	a.mu.Lock()
	a.localeChoice = localeChoiceFor(tag)
	if a.settings != nil {
		a.settings.LocaleIndex = a.localeChoice
	}
	a.mu.Unlock()
	a.invalidate()
	// The picker runs inside a frame and a language change redraws every
	// window, so it is applied on the next turn of the main loop, as windows
	// are created and hidden there.
	go func() { mygo.RunOnMain(func() { mygo.App.SetLocale(tag) }) }()
}

// localeChoiceFor maps a language tag to the settings picker's entry.
func localeChoiceFor(tag string) int {
	switch i18n.Base(tag) {
	case "zh":
		return 1
	case "en":
		return 2
	default:
		return 0
	}
}

// initialLocaleChoice reads whether the app follows the desktop: a language
// that differs from what the desktop asks for was chosen explicitly.
func initialLocaleChoice() int {
	if i18n.Match(mygo.App.PreferredLanguages(), mygo.App.Languages()) == mygo.App.Locale() {
		return 0
	}
	return localeChoiceFor(mygo.App.Locale())
}

// openConfig hands the settings file to the desktop so the [identity] section
// can be edited by hand; the window itself never writes it.
func (a *app) openConfig() {
	a.mu.RLock()
	state := a.settings
	path := a.configPath
	a.mu.RUnlock()
	if path == "" {
		if state != nil {
			state.SetStatus(locales.ConfigPathUnavailable(), true)
			a.invalidate()
		}
		return
	}
	if err := mygo.Shell.OpenPath(path); err != nil {
		if state != nil {
			state.SetStatus(locales.OpenConfigFailed(err.Error()), true)
			a.invalidate()
		}
		a.logf("open config: %v", err)
	}
}

// openSettingsPassword opens the voluntary change panel and loads the policy
// from the live session. A policy that cannot be read still leaves the form
// usable: the portal re-checks every rule when the change is sent.
func (a *app) openSettingsPassword() {
	a.mu.Lock()
	state := a.settings
	if state == nil || !state.CanChangePassword() {
		a.mu.Unlock()
		return
	}
	username := state.Draft.Active().Auth.Username
	state.Password = &gotrustui.PasswordPrompt{Username: username}
	state.PasswordBusy = true
	state.PasswordStatus = locales.RulesLoading()
	state.PasswordError = false
	a.mu.Unlock()
	a.invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		strategy, err := a.manager.PasswordStrategy(ctx)

		a.mu.Lock()
		if current := a.settings; current != nil && current.Password != nil {
			current.PasswordBusy = false
			if err != nil {
				current.PasswordStatus = locales.RulesFailed(err.Error())
				current.PasswordError = true
			} else {
				current.Password.Strategy = strategy
				current.PasswordStatus = ""
			}
		}
		a.mu.Unlock()
		a.invalidate()
	}()
}

// refreshSettingsTerminals reads both 终端管理 lists through the live session.
// The settings state owns the status line; the call runs off the UI thread
// because the portal answers over the tunnel.
func (a *app) refreshSettingsTerminals() {
	a.mu.Lock()
	state := a.settings
	if state == nil || state.TrustBusy || !state.Online || a.manager == nil {
		a.mu.Unlock()
		return
	}
	state.TrustBusy = true
	state.TrustStatus = locales.TerminalsLoading()
	state.TrustError = false
	a.mu.Unlock()
	a.invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		terminals, err := a.manager.Terminals(ctx)

		a.mu.Lock()
		if current := a.settings; current != nil {
			current.TrustBusy = false
			if err != nil {
				current.TrustStatus = locales.TerminalsFailed(err.Error())
				current.TrustError = true
			} else {
				current.Terminals = terminals
				current.TerminalsKnown = true
				current.TrustKnown = true
				current.Trusted = terminals.DeviceTrusted
				current.TrustStatus = ""
				current.TrustError = false
			}
		}
		a.mu.Unlock()
		a.invalidate()
	}()
}

// setSettingsTrusted asks the portal to trust or untrust this device. A
// refusal is the portal's policy, not a UI error: it is shown verbatim.
func (a *app) setSettingsTrusted(trusted bool) {
	a.mu.Lock()
	state := a.settings
	if state == nil || state.TrustBusy {
		a.mu.Unlock()
		return
	}
	if !state.Online || a.manager == nil {
		state.TrustStatus = locales.TrustNeedsConnection()
		state.TrustError = true
		a.mu.Unlock()
		a.invalidate()
		return
	}
	state.TrustBusy = true
	state.TrustStatus = locales.Submitting()
	state.TrustError = false
	a.mu.Unlock()
	a.invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := a.manager.SetTrusted(ctx, trusted)

		a.mu.Lock()
		if current := a.settings; current != nil {
			current.TrustBusy = false
			if err != nil {
				current.TrustStatus = locales.TrustFailed(err.Error())
				current.TrustError = true
			} else {
				current.TrustKnown = true
				current.Trusted = trusted
				if trusted {
					current.TrustStatus = locales.Trusted()
				} else {
					current.TrustStatus = locales.Untrusted()
				}
				current.TrustError = false
			}
		}
		a.mu.Unlock()
		a.invalidate()
		if err == nil {
			go a.refreshSettingsTerminals()
		}
	}()
}

// cancelSettingsPassword returns to the sections. A change in flight cannot be
// cancelled from the window: the portal has no cancel call, so the panel stays
// busy until the answer arrives.
func (a *app) cancelSettingsPassword() {
	a.mu.Lock()
	state := a.settings
	if state == nil || state.PasswordBusy {
		a.mu.Unlock()
		return
	}
	state.Password = nil
	state.PasswordStatus = ""
	state.PasswordError = false
	a.mu.Unlock()
	a.invalidate()
}

// submitSettingsPassword sends the voluntary change through the live session.
// The weak-password answer is advisory: a database that cannot be reached is
// logged and the change still goes to the portal, which is the authority.
func (a *app) submitSettingsPassword() {
	a.mu.Lock()
	state := a.settings
	if state == nil || state.Password == nil || state.PasswordBusy {
		a.mu.Unlock()
		return
	}
	answer := *state.Password
	if err := (&answer).Validate(); err != nil {
		state.PasswordStatus = err.Error()
		state.PasswordError = true
		a.mu.Unlock()
		a.invalidate()
		return
	}
	state.PasswordBusy = true
	state.PasswordStatus = locales.PasswordResetting()
	state.PasswordError = false
	a.mu.Unlock()
	a.invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		err := a.changePasswordOnline(ctx, answer.OldPassword, answer.NewPassword)

		a.mu.Lock()
		if current := a.settings; current != nil && current.Password != nil {
			current.PasswordBusy = false
			if err != nil {
				current.PasswordStatus = locales.ResetFailed(err.Error())
				current.PasswordError = true
			} else {
				current.Password = nil
				current.Status = locales.PasswordResetDone()
				current.StatusError = false
			}
		}
		a.mu.Unlock()
		a.invalidate()
	}()
}

// changePasswordOnline checks the weak-password database, sends the change and
// stores the new credential. The order matches the reset form: the weak
// check is what the form runs before the submit.
func (a *app) changePasswordOnline(ctx context.Context, oldPassword, newPassword string) error {
	result, err := a.manager.CheckWeakPassword(ctx, newPassword)
	switch {
	case err != nil:
		a.logf("weak-password check failed (%v); sending the change anyway", err)
	case result.Code != 0 && !result.Validate:
		message := result.Message
		if message == "" {
			message = locales.WeakPassword()
		}
		return errors.New(message)
	case result.Code != 0:
		if result.Message != "" {
			return errors.New(result.Message)
		}
		return errors.New(locales.PasswordRejected(result.Code))
	}

	profile := a.manager.LiveProfile()
	if profile == "" {
		profile = a.currentConfig().ActiveProfile
	}
	if err := a.manager.ResetPassword(ctx, oldPassword, newPassword); err != nil {
		return err
	}
	a.storeChangedPassword(profile, newPassword)
	return nil
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
			// 断开 cancels the attempt: the user asked for the stop, so report
			// it as one instead of painting the cancelled login red.
			if errors.Is(err, context.Canceled) || a.isStopping() {
				a.setStatus(locales.Cancelled(), false)
				return
			}
			a.setStatusError(err.Error())
			return
		}
		a.setStatus("", false)
	}()
}

// startLogin is 连接 in the window. The account page edits a draft, so a draft
// that differs from the file is saved first: the gateway the user just typed is
// the one the login uses. A draft that cannot be saved keeps its edits and
// moves the window to 账户, where the reason names a field.
func (a *app) startLogin() {
	if !a.saveDraftForLogin() {
		return
	}
	a.startSavedLogin()
}

// saveDraftForLogin writes the settings draft when it differs from the file and
// reports whether a login can go on. The window selects 账户 on a rejection:
// the reason is a sentence about one field, and the form has to be on screen.
func (a *app) saveDraftForLogin() bool {
	a.mu.RLock()
	state := a.settings
	a.mu.RUnlock()
	if state == nil || !state.Changed() {
		return true
	}

	previous := a.currentConfig()
	cfg, ok := a.writeDraft(state)
	if !ok {
		a.showPage(gotrustui.PageAccount)
		return false
	}
	// A saved draft may point at another account. The login ends the old
	// session, and connect waits for that join before it starts.
	a.mu.RLock()
	online := sessionActive(a.view.Session.Phase)
	a.mu.RUnlock()
	if online && previous.ActiveProfile != cfg.ActiveProfile {
		a.disconnect()
	}
	return true
}

// startSavedLogin is 连接 with the configuration already on disk. The tray uses
// it: a tray click must not save the draft the window is still editing. An
// incomplete profile cannot produce a login, and the library's validation
// message names a field the window does not offer, so the window opens on the
// account form with the window's own sentence instead.
func (a *app) startSavedLogin() {
	if problem := gotrustui.ProfileProblem(a.currentConfig().Active()); problem != "" {
		a.setStatusError(problem)
		a.showPage(gotrustui.PageAccount)
		return
	}
	a.async(a.connect)
}

func (a *app) connect(ctx context.Context) error {
	// 断开 joins the session on another goroutine. Building a new session
	// before that join finishes would let the old Logout tear it down.
	if err := a.waitForLogout(ctx); err != nil {
		return err
	}
	// Auto-connect and a caller that raced a settings change can reach here
	// with an incomplete profile; answer in the window's words, not the
	// library's.
	if problem := gotrustui.ProfileProblem(a.currentConfig().Active()); problem != "" {
		return errors.New(problem)
	}
	if err := a.acquireProfileLock(); err != nil {
		return err
	}
	if err := a.manager.Login(ctx); err != nil {
		a.releaseProfileLock()
		return err
	}
	if err := ctx.Err(); err != nil {
		a.manager.Logout()
		a.releaseProfileLock()
		return err
	}

	// ctx is the attempt's deadline, which async cancels as soon as this
	// returns. The listeners must live until the session ends.
	if err := a.restartProxy(); err != nil {
		a.manager.Logout()
		a.releaseProfileLock()
		return err
	}
	return nil
}

// acquireProfileLock reserves the active profile for this process. The CLI
// takes the same lock before its login: one profile has one client data file
// and one tunnel identity, so a second live login would overwrite the session
// the first one stored.
func (a *app) acquireProfileLock() error {
	cfg := a.currentConfig()
	lock, err := profilelock.Acquire(a.configPath, cfg.ActiveProfile)
	if err != nil {
		return err
	}

	a.mu.Lock()
	if a.profileLock != nil {
		a.mu.Unlock()
		_ = lock.Release()
		return fmt.Errorf("profile %q is already held by this instance", cfg.ActiveProfile)
	}
	a.profileLock = lock
	a.mu.Unlock()
	return nil
}

// releaseProfileLock gives the profile back. It runs after Manager.Logout
// finished, so the next instance cannot start a login against a client that is
// still being torn down.
func (a *app) releaseProfileLock() {
	a.mu.Lock()
	lock := a.profileLock
	a.profileLock = nil
	a.mu.Unlock()
	if lock == nil {
		return
	}
	if err := lock.Release(); err != nil {
		a.logf("release the profile lock: %v", err)
	}
}

// setLogLines is the tailer's hand-off: the newest lines, oldest first. The
// assignment runs on the main loop, the one that renders the page, so the
// window never reads a slice the tailer is replacing.
func (a *app) setLogLines(lines []string) {
	mygo.RunOnMain(func() {
		a.mu.Lock()
		if a.settings != nil {
			a.settings.LogLines = lines
		}
		a.mu.Unlock()
		a.invalidate()
	})
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
	if state.Phase == session.PhaseFailed && state.Attention && !stopping {
		a.releaseProxy()
		a.releaseProfileLock()
	}

	a.mu.Lock()
	if a.settings != nil {
		a.settings.Online = state.Phase == session.PhaseOnline
	}
	a.mu.Unlock()
	a.invalidate()

	if window := a.window.Load(); window != nil {
		window.Invalidate()
	}
	a.refreshTray()

	// A login that fails while the window is hidden must not fail silently, and
	// neither may a session that expired or a ladder that stopped at a prompt.
	if (state.Phase == session.PhaseFailed || state.Attention) && !stopping {
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
	a.view.Listeners = nil
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

	if !a.installProxy(server, cancel, epoch) {
		return nil
	}
	return nil
}

// installProxy publishes a server unless the epoch moved while it was binding:
// a disconnect or a terminal failure in that window already closed the
// listeners and cleared the view, so installing it would hold the ports for a
// session that no longer exists. It reports whether the server was installed.
func (a *app) installProxy(server *gotrustproxy.Server, cancel context.CancelFunc, epoch uint64) bool {
	a.mu.Lock()
	if a.proxyEpoch != epoch {
		a.mu.Unlock()
		cancel()
		_ = server.Close()
		return false
	}
	a.proxy = server
	a.proxyCancel = cancel
	a.view.Listeners = server.Listeners()
	a.mu.Unlock()

	a.invalidate()
	a.refreshTray()
	return true
}

// isStopping reports whether 断开 or quit is tearing the session down.
func (a *app) isStopping() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stopping
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
// loop: after App.Run returned. onMain drops its callbacks then, so joining the
// session here cannot deadlock.
func (a *app) disconnectNow() {
	a.stopSession()
	a.logoutSession()
	a.releaseProfileLock()

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
		// The profile is free only after the client is gone: a new login that
		// started earlier would otherwise share the file this logout is still
		// closing.
		a.releaseProfileLock()

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
	previous := a.currentConfig()
	cfg, ok := a.writeDraft(state)
	if !ok {
		return
	}
	a.reconcileProfileSwitch(previous, cfg)
	a.invalidate()
}

// writeDraft stores the settings draft and makes it the settings in effect. It
// reports the saved configuration and whether the write happened; a rejected
// draft keeps its edits and says why, so 保存 and a login that saves first
// report the same reason.
func (a *app) writeDraft(state *gotrustui.SettingsState) (config.Config, bool) {
	cfg, err := state.Apply()
	if err != nil {
		state.SetStatus(err.Error(), true)
		a.invalidate()
		return config.Config{}, false
	}
	if err := config.SaveFile(a.configPath, cfg.ForDisk()); err != nil {
		state.SetStatus(locales.SaveFailed(err.Error()), true)
		a.invalidate()
		a.logf("save settings: %v", err)
		return config.Config{}, false
	}

	// The file now names the profiles the draft describes, so the stored account
	// material may follow a rename or a delete. Doing this before the save would
	// move files under a configuration that still names the old profile.
	a.applyClientDataChanges(cfg, state.ClientDataChanges())
	a.applyConfig(cfg)

	// Reflect the normalised values back into the form.
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	if settings != nil {
		settings.Reload(cfg)
		settings.SetStatus(locales.SettingsSaved(), false)
	}
	return cfg, true
}

// selectProfile switches accounts from the tray or the 多账户 page. One
// live session at a time: the old one disconnects first.
func (a *app) selectProfile(name string) {
	cfg := a.currentConfig()
	if cfg.ActiveProfile == name {
		return
	}
	if cfg.ProfileIndex(name) < 0 {
		a.setStatusError(locales.ProfileMissing(name))
		return
	}

	a.mu.RLock()
	online := sessionActive(a.view.Session.Phase)
	a.mu.RUnlock()

	cfg.ActiveProfile = name
	if err := config.SaveFile(a.configPath, cfg.ForDisk()); err != nil {
		a.setStatusError(locales.SwitchFailed(err.Error()))
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

	// Keep the pages' draft in step with the switch.
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	if settings != nil {
		settings.Reload(cfg)
		settings.SetStatus(locales.ProfileSwitched(), false)
		a.invalidate()
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
			state.SetStatus(locales.AutostartFailed(err.Error()), true)
			a.invalidate()
		}
		a.logf("autostart: %v", err)
		return
	}
	a.persistFlag(func(cfg *config.Config) { cfg.App.Autostart = on })
	if state != nil {
		status := locales.AutostartOn()
		if !on {
			status = locales.AutostartOff()
		}
		state.SetStatus(status, false)
		a.invalidate()
	}
}

// setVerbose changes the running log level immediately and persists it.
func (a *app) setVerbose(on bool) {
	if a.log != nil {
		a.log.SetVerbose(on)
	}
	session.SetProtocolDebug(on)
	a.persistFlag(func(cfg *config.Config) { cfg.Logging.Verbose = on })
}

func (a *app) persistFlag(mutate func(*config.Config)) {
	cfg := a.currentConfig()
	mutate(&cfg)
	if err := config.SaveFile(a.configPath, cfg.ForDisk()); err != nil {
		a.setStatusError(locales.SaveSettingsFailed(err.Error()))
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
			state.SetStatus(locales.LogDirUnavailable(), true)
			a.invalidate()
		}
		return
	}
	if err := mygo.Shell.OpenPath(a.logDir); err != nil {
		if state != nil {
			state.SetStatus(locales.OpenLogsFailed(err.Error()), true)
			a.invalidate()
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
		state.SetStatus(err.Error(), true)
		a.invalidate()
		return
	}

	key := keyForTarget(target)
	state.Discovering = true
	state.Methods = nil
	state.MethodsFrom = ""
	a.invalidate()

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
				state.SetMethods(nil, key, locales.MethodsDiscoverFailed(discoverErr.Error()), true)
				a.logf("discover auth methods: %v", discoverErr)
			case len(methods) == 0:
				state.SetMethods(nil, key, locales.MethodsNone(), false)
			default:
				state.SetMethods(methods, key, locales.MethodsLoaded(len(methods)), false)
				a.logf("gateway offers %d authentication method(s)", len(methods))
			}
			a.invalidate()
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
		return autoConnectDecision{Reason: locales.AutoConnectNoAddress()}
	}
	if strings.TrimSpace(profile.Auth.Username) == "" {
		return autoConnectDecision{Reason: locales.AutoConnectNoUsername()}
	}
	// A stored session is not a login credential: the portal ends a session
	// with its connection, so startup needs a saved password to reconnect.
	if profile.Auth.StorePass && profile.Auth.Password != "" {
		return autoConnectDecision{Connect: true}
	}
	return autoConnectDecision{Reason: locales.AutoConnectNoPassword()}
}

func (a *app) setStatus(message string, busy bool) {
	a.mu.Lock()
	a.view.Status = message
	a.view.StatusSeq++
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
	a.view.StatusSeq++
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
	status := gotrustui.TrayStatus(state.Session)
	if summary := gotrustui.KeepAliveSummary(state.Session.KeepAlive); summary != "" {
		return locales.TrayTooltipKeepAlive(status, summary)
	}
	return locales.TrayTooltip(status)
}
