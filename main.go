package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/mythologyli/zju-connect/client/authchallenge"

	"github.com/YangtseSu/goTrust/config"
	gotrustproxy "github.com/YangtseSu/goTrust/proxy"
	"github.com/YangtseSu/goTrust/session"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

func main() {
	headless := flag.Bool("no-ui", false, "run without a window; connect, serve the proxies, and exit on SIGINT or SIGTERM")
	configPath := flag.String("config", "", "settings file (default: the user config directory)")
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		logger.Fatalf("settings: %v", err)
	}

	app := &app{
		cfg:    cfg,
		logf:   logger.Printf,
		view:   gotrustui.State{Config: cfg},
		logger: logger,
	}
	app.view.Server, app.view.Port = cfg.ServerAddress(), strconv.Itoa(cfg.Server.Port)
	app.view.Username = cfg.Auth.Username
	app.view.Password = cfg.Auth.Password

	app.manager = session.New(session.Options{
		Config:  app.currentConfig,
		Auth:    app.currentAuth,
		Prompt:  cliPrompter{logger: logger},
		Logf:    logger.Printf,
		OnState: app.onSessionState,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *headless {
		app.runHeadless(ctx)
		return
	}

	app.runUI(ctx)
}

// app ties the session, the proxies and the window together.
type app struct {
	cfg     config.Config
	manager *session.Manager
	logger  *log.Logger
	logf    func(string, ...any)

	mu     sync.RWMutex
	view   gotrustui.State
	proxy  *gotrustproxy.Server
	window atomicWindow
	tray   atomicTray
}

func loadConfig(path string) (config.Config, error) {
	if path == "" {
		return config.Load()
	}
	return config.LoadFile(path)
}

func (a *app) runHeadless(ctx context.Context) {
	if err := a.connect(ctx); err != nil {
		a.logger.Fatalf("connect: %v", err)
	}
	<-ctx.Done()
	a.disconnect()
}

func (a *app) runUI(ctx context.Context) {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	a.logger.SetOutput(os.Stderr)

	// Quit runs the quit sequence, so the tray disappears, the proxies close
	// and the single-instance lock is released. Exit would skip all three and
	// leave a stale socket behind.
	mygo.App.OnQuit(a.disconnect)

	mygo.App.WhenReady(func() {
		a.openWindow()
		a.openTray()
		a.logger.Printf("goTrust ready; fill in the gateway and press 连接")
	})

	go func() {
		<-ctx.Done()
		mygo.App.Quit()
	}()

	if err := mygo.App.Run(); err != nil {
		a.logger.Fatalf("ui: %v", err)
	}
	a.disconnect()
}

func (a *app) openWindow() {
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:  "goTrust",
		Width:  520,
		Height: 560,
		Content: ui.View(func(c *ui.Context) {
			a.mu.RLock()
			state := a.view
			a.mu.RUnlock()
			gotrustui.View(&state, a.actions())(c)
		}),
	})
	a.window.Store(window)
}

func (a *app) openTray() {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:    trayIcon(),
		ToolTip: "goTrust",
	})
	if err != nil {
		a.logger.Printf("tray unavailable: %v", err)
		return
	}
	a.tray.Store(tray)

	a.mu.RLock()
	state := a.view
	a.mu.RUnlock()
	tray.SetMenu(gotrustui.TrayMenu(&state, a.actions()))
	tray.OnClick(func() {
		if window := a.window.Load(); window != nil {
			window.Show()
			window.Focus()
		}
	})
}

// actions are the callbacks the window and the tray share.
func (a *app) actions() gotrustui.Actions {
	return gotrustui.Actions{
		Login:  func() { a.async(a.connect) },
		Logout: a.disconnect,
		Save:   func() { a.async(func(context.Context) error { return a.save() }) },
		Reload: func() {
			a.async(func(context.Context) error {
				cfg, err := loadConfig("")
				if err != nil {
					return err
				}
				a.applyConfig(cfg)
				return nil
			})
		},
		Quit: func() { mygo.App.Quit() },
	}
}

// async runs work off the UI thread and reports the outcome in the status line.
func (a *app) async(fn func(context.Context) error) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		a.setStatus("…", true)
		err := fn(ctx)
		if err != nil {
			a.setStatus(err.Error(), false)
			return
		}
		a.setStatus("", false)
	}()
}

func (a *app) connect(ctx context.Context) error {
	if err := a.manager.Login(ctx); err != nil {
		return err
	}
	return a.startProxy(ctx)
}

// onSessionState mirrors the session into the view and refreshes what is on
// screen. It runs on the goroutine that changed the session.
func (a *app) onSessionState(state session.State) {
	a.mu.Lock()
	a.view.Session = state
	a.mu.Unlock()

	if window := a.window.Load(); window != nil {
		window.Invalidate()
	}
	if tray := a.tray.Load(); tray != nil {
		tray.SetToolTip("goTrust: " + string(state.Phase))
	}
}

func (a *app) startProxy(ctx context.Context) error {
	socks5, httpBind := a.currentConfig().ProxyAddresses()

	server, err := gotrustproxy.New(gotrustproxy.Options{
		SOCKS5Bind: socks5,
		HTTPBind:   httpBind,
		Dial:       a.manager.Dial,
		Logf:       a.logf,
	})
	if err != nil {
		return err
	}
	if err := server.Start(ctx); err != nil {
		return err
	}

	a.mu.Lock()
	a.proxy = server
	a.view.Addresses = server.Addresses()
	a.mu.Unlock()

	if window := a.window.Load(); window != nil {
		window.Invalidate()
	}
	return nil
}

func (a *app) disconnect() {
	a.mu.Lock()
	server := a.proxy
	a.proxy = nil
	a.view.Addresses = nil
	a.mu.Unlock()

	if server != nil {
		_ = server.Close()
	}
	a.manager.Logout()

	if window := a.window.Load(); window != nil {
		window.Invalidate()
	}
}

func (a *app) save() error {
	cfg := a.currentConfig()
	if cfg.Auth.StorePass {
		cfg.Auth.Password = a.currentAuth().Password
	} else {
		cfg.Auth.Password = ""
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	a.applyConfig(cfg)
	a.setStatus("设置已保存", false)
	return nil
}

func (a *app) applyConfig(cfg config.Config) {
	a.mu.Lock()
	a.cfg = cfg
	a.view.Config = cfg
	a.mu.Unlock()
}

// currentConfig merges the edited fields into the settings in effect.
func (a *app) currentConfig() config.Config {
	a.mu.RLock()
	cfg := a.cfg
	view := a.view
	a.mu.RUnlock()

	cfg.Server.Address = strings.TrimSpace(view.Server)
	if port, err := strconv.Atoi(strings.TrimSpace(view.Port)); err == nil {
		cfg.Server.Port = port
	}
	cfg.Auth.Username = view.Username
	return cfg
}

func (a *app) currentAuth() config.AuthConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()

	auth := a.cfg.Auth
	auth.Username = a.view.Username
	auth.Password = a.view.Password
	return auth
}

func (a *app) setStatus(message string, busy bool) {
	a.mu.Lock()
	a.view.Status = message
	a.view.Busy = busy
	a.mu.Unlock()

	if window := a.window.Load(); window != nil {
		window.Invalidate()
	}
}

// cliPrompter answers the login prompts nothing else can: in a window build the
// dialog is the right place, and here it refuses rather than blocking forever on
// a stdin that may not exist.
type cliPrompter struct {
	logger *log.Logger
}

func (p cliPrompter) HandleCodeChallenge(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
	return authchallenge.CodeResponse{}, fmt.Errorf("this login needs a one-time code (%s) and the dialog is not implemented yet", challenge.Message)
}

func (p cliPrompter) HandleTextCaptcha(authchallenge.TextCaptchaChallenge) (authchallenge.TextCaptchaResponse, error) {
	return authchallenge.TextCaptchaResponse{}, errors.New("this gateway asked for a text captcha, which the skeleton does not handle yet")
}

func (p cliPrompter) HandleClickCaptcha(authchallenge.ClickCaptchaChallenge) (authchallenge.ClickCaptchaResponse, error) {
	return authchallenge.ClickCaptchaResponse{}, errors.New("this gateway asked for a click captcha, which the skeleton does not handle yet")
}

func (p cliPrompter) HandleExternalLogin(challenge authchallenge.ExternalLoginChallenge) (authchallenge.ExternalLoginResponse, error) {
	return authchallenge.ExternalLoginResponse{}, fmt.Errorf("this login needs a browser at %s, which the skeleton does not open yet", challenge.LoginURL)
}
