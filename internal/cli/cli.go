// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli is the headless front-end. It loads the settings, logs in, serves
// the same listeners as the tray app and reports through stdout and the log
// file, without a toolkit anywhere in its dependency graph.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	atrustlog "github.com/YangtseSu/goTrust/core/atrust/log"
	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/logging"
	"github.com/YangtseSu/goTrust/core/profilelock"
	gotrustproxy "github.com/YangtseSu/goTrust/core/proxy"
	"github.com/YangtseSu/goTrust/core/session"
)

// Exit codes. A supervisor or a script can tell a finished job from a failed
// one, a mistyped command line from a login that never happened, and
// --status's "nobody is using this profile" from its own errors.
const (
	ExitOK         = 0
	ExitFailure    = 1
	ExitUsage      = 2
	ExitNotRunning = 3
)

// program is the name the usage line and the error prefixes carry.
const program = "gotrust-cli"

// Run parses one command line and runs it, returning the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(program, flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "TOML settings file (default: the user config directory)")
	status := flags.Bool("status", false,
		"report whether the active profile is in use and print the cached grant, then exit")
	printResources := flags.Bool("print-resources", false,
		"print the grant cached by the last login, then exit")
	asJSON := flags.Bool("json", false, "encode the reported grant and status as JSON")

	switch err := flags.Parse(args); {
	case errors.Is(err, flag.ErrHelp):
		return ExitOK
	case err != nil:
		return ExitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected argument %q\n", program, flags.Arg(0))
		return ExitUsage
	}
	if *status && *printResources {
		fmt.Fprintf(stderr, "%s: -status and -print-resources are separate commands\n", program)
		return ExitUsage
	}

	cfg, path, err := config.LoadOrCreate(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "settings: %v\n", err)
		return ExitFailure
	}
	switch {
	case *status:
		return reportStatus(stdout, stderr, cfg.ActiveProfile, path, *asJSON)
	case *printResources:
		return reportCachedGrant(stdout, stderr, path, *asJSON)
	default:
		return runSession(stdout, stderr, cfg, path, *asJSON)
	}
}

// runSession is the long-running command: log in, publish the listeners, print
// the grant, and stay up until a signal or a failure that needs a person.
func runSession(stdout, stderr io.Writer, cfg config.Config, configPath string, asJSON bool) int {
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "settings: %v\n", err)
		return ExitFailure
	}

	logf, closeLog := openLog(cfg, stderr)
	defer closeLog()

	// The tray app takes the same lock before its login. A profile already held
	// by another instance has one client data file and one tunnel identity, and
	// two live logins would overwrite each other's session.
	lock, err := profilelock.Acquire(configPath, cfg.ActiveProfile)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitFailure
	}
	defer func() {
		if err := lock.Release(); err != nil {
			logf("release the profile lock: %v", err)
		}
	}()

	failures := make(chan string, 1)
	manager := session.New(session.Options{
		Config: func() config.Config { return cfg },
		Logf:   logf,
		Prompt: func(context.Context) session.Prompter { return refusingPrompter{} },
		// The reconnect ladder retries a transient loss itself. A failure that
		// needs a person cannot recover in a process nobody is watching, so it
		// ends the run with the reason instead of waiting forever.
		OnState: func(state session.State) {
			if state.Phase != session.PhaseFailed || !state.Attention {
				return
			}
			select {
			case failures <- state.Detail:
			default:
			}
		},
		ResourceCachePath: config.ResourcesPath(configPath),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := manager.Login(ctx); err != nil {
		if ctx.Err() != nil {
			logf("connect cancelled")
			return ExitOK
		}
		fmt.Fprintf(stderr, "connect: %v\n", err)
		return ExitFailure
	}
	defer manager.Logout()

	socks5, httpBind := cfg.ProxyAddresses()
	proxyCtx, cancelProxy := context.WithCancel(context.Background())
	defer cancelProxy()

	server, err := gotrustproxy.New(gotrustproxy.Options{
		SOCKS5Bind: socks5,
		HTTPBind:   httpBind,
		Dial:       manager.Dial,
		Resolver:   manager,
		Logf:       logf,
	})
	if err != nil {
		fmt.Fprintf(stderr, "proxy: %v\n", err)
		return ExitFailure
	}
	if err := server.Start(proxyCtx); err != nil {
		fmt.Fprintf(stderr, "proxy: %v\n", err)
		return ExitFailure
	}
	defer func() { _ = server.Close() }()
	logf("serving %s", strings.Join(server.Addresses(), ", "))

	state := manager.State()
	printGrant(stdout, liveGrant{
		Profile:   cfg.ActiveProfile,
		Gateway:   gatewayAddress(cfg),
		TunnelIP:  state.ClientIP,
		Resources: state.Resources,
	}, asJSON)

	select {
	case <-ctx.Done():
		logf("shutting down")
		return ExitOK
	case detail := <-failures:
		fmt.Fprintf(stderr, "session failed: %s\n", detail)
		return ExitFailure
	}
}

// openLog opens the log file and mirrors it to stderr, which is where a
// headless run's operator looks. A log file that cannot be opened degrades to
// stderr instead of stopping the login.
func openLog(cfg config.Config, stderr io.Writer) (func(string, ...any), func()) {
	logger, err := logging.New(logging.Options{
		Verbose: cfg.Logging.Verbose,
		Mirror:  stderr,
	})
	if err != nil {
		fmt.Fprintf(stderr, "logging: %v; continuing on stderr\n", err)
		log.SetOutput(stderr)
		return log.New(stderr, "", log.LstdFlags).Printf, func() {}
	}
	// The aTrust client logs through the standard library's global logger, so
	// pointing that at the same writer is the whole bridge.
	log.SetOutput(logger)
	if cfg.Logging.Verbose {
		atrustlog.EnableDebug()
	}
	return logger.Printf, func() {
		_ = logger.Close()
		log.SetOutput(io.Discard)
	}
}
