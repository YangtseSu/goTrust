// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package session owns one aTrust login and everything that hangs off it: the
// tunnel client, its netstack, the resolver, the dialer and the keep-alive.
//
// The package is deliberately the only place that knows the upstream module's
// assembly order. The UI and the proxy layer talk to it through State and
// Manager.Dial, never through the client type.
package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	clientpkg "github.com/mythologyli/zju-connect/client"
	"github.com/mythologyli/zju-connect/client/atrust"
	"github.com/mythologyli/zju-connect/client/atrust/auth"
	"github.com/mythologyli/zju-connect/client/authchallenge"
	"github.com/mythologyli/zju-connect/dial"
	"github.com/mythologyli/zju-connect/resolve"
	"github.com/mythologyli/zju-connect/service"
	"github.com/mythologyli/zju-connect/stack"
	"github.com/mythologyli/zju-connect/stack/gvisor"
	"github.com/mythologyli/zju-connect/stack/tcptunnel"
	"github.com/mythologyli/zju-connect/underlay"

	"github.com/YangtseSu/goTrust/config"
)

// Phase is the lifecycle position of the session, in the order a user sees it.
type Phase string

const (
	// PhaseIdle means no session exists.
	PhaseIdle Phase = "idle"
	// PhaseConnecting covers the whole login and data-path setup.
	PhaseConnecting Phase = "connecting"
	// PhaseOnline means the tunnel is usable and the proxies may serve.
	PhaseOnline Phase = "online"
	// PhaseFailed means the last attempt stopped with Detail as the reason.
	PhaseFailed Phase = "failed"
)

// State is one snapshot of the session, safe to render in a UI.
type State struct {
	Phase Phase
	// Detail explains the phase: the current step while connecting, the error
	// when failed, and the gateway address when online.
	Detail string
	// ClientIP is the address the gateway assigned, empty until online and
	// empty for a TCP-only gateway.
	ClientIP string
	// Since is when the phase was entered.
	Since time.Time
}

// ErrNotOnline reports a dial attempt while no session exists.
var ErrNotOnline = errors.New("the session is not online")

// Prompter asks the user for whatever the login needs beyond a password: a
// one-time code, a captcha answer, or a URL to open in a browser.
//
// The alias mirrors authchallenge.Handler on purpose: the same value is handed
// to the client. A headless run can supply one that fails with a clear message
// instead of waiting on a dialog that will never appear.
type Prompter = authchallenge.Handler

// Options describe one login attempt.
type Options struct {
	// Config reads the settings in effect. Like Auth it is a function: the
	// user edits the form after the manager exists, and a login must use what
	// is on screen now, not what was loaded at startup.
	Config func() config.Config
	// Prompt builds the handler for one login attempt. It is a function
	// because a handler that waits for the user must stop waiting when the
	// attempt's context ends: a cancelled login must not leave a dialog open.
	// A gateway without a second factor never calls it.
	Prompt func(ctx context.Context) Prompter
	// ChooseAuth asks which offered method to log in with. It is called only
	// when the gateway offers several and the profile stores none, so a
	// settings window that pins a method never asks. Nil fails the login with
	// the list, which is what a headless run should do.
	ChooseAuth func(ctx context.Context, methods []AuthMethod) (AuthMethod, error)
	// Logf receives progress lines. Nil discards them.
	Logf func(format string, args ...any)
	// OnState is called on every transition, from the goroutine that caused it.
	OnState func(State)
}

// Manager holds the live session and serialises login, logout and dialling.
type Manager struct {
	options Options
	logf    func(format string, args ...any)

	mu       sync.RWMutex
	state    State
	client   *atrust.Client
	dialer   *dial.Dialer
	resolver *resolve.Resolver
	underlay *underlay.Dialer
	cancel   context.CancelFunc
	login    sync.Mutex
}

// New creates a manager in the idle phase.
func New(options Options) *Manager {
	logf := options.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Manager{
		options: options,
		logf:    logf,
		state:   State{Phase: PhaseIdle, Detail: "not connected", Since: time.Now()},
	}
}

// State returns the current snapshot.
func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Login establishes a session. It returns once the tunnel is usable, or with
// the reason it is not. Calling it while online is an error rather than a
// silent reconnect: a second full login rotates the device token and drops
// other clients of the same account.
func (m *Manager) Login(ctx context.Context) error {
	m.login.Lock()
	defer m.login.Unlock()

	switch m.State().Phase {
	case PhaseConnecting, PhaseOnline:
		return fmt.Errorf("the session is already %s", m.State().Phase)
	}

	cfg := m.settings()
	if err := cfg.Validate(); err != nil {
		m.fail(err)
		return err
	}
	// One snapshot drives the whole attempt: a profile switch while this login
	// runs must not mix one account's gateway with another's credentials.
	profile := cfg.Active()

	// The session must outlive the login attempt. The caller's context carries
	// the attempt's deadline and the application cancels it once Login returns,
	// so deriving the session from it would stop the keep-alive immediately.
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	m.set(State{Phase: PhaseConnecting, Detail: "preparing the tunnel", Since: time.Now()})

	underlayDialer, err := underlay.New(underlay.Options{})
	if err != nil {
		cancel()
		m.fail(fmt.Errorf("create the underlay dialer: %w", err))
		return err
	}

	// From here on every failure has to release what was already opened.
	fail := func(stage string, cause error) error {
		cancel()
		_ = underlayDialer.Close()
		wrapped := fmt.Errorf("%s: %w", stage, cause)
		m.fail(wrapped)
		return wrapped
	}

	clientDataPath, err := cfg.ClientDataPathFor(profile)
	if err != nil {
		return fail("locate the stored session", err)
	}
	clientData, err := readClientData(clientDataPath)
	if err != nil {
		return fail("read the stored session", err)
	}

	// The gateway decides which pair is valid; a saved pair that it no longer
	// offers has to be a visible error, not a silent fallback. An empty list
	// means the gateway has nothing to ask (already logged in), so the client
	// may restore the stored session instead of logging in.
	m.set(State{Phase: PhaseConnecting, Detail: "discovering the authentication methods", Since: time.Now()})
	methods, err := DiscoverAuthMethods(ctx, profile)
	if err != nil {
		return fail("discover the authentication methods", err)
	}

	method, err := m.resolveAuthMethod(ctx, methods, profile)
	if err != nil {
		return fail("choose the authentication method", err)
	}

	var loginMethod auth.LoginMethod
	if method != nil {
		loginMethod, err = newLoginMethod(profile.Auth, *method)
		if err != nil {
			return fail("configure the login", err)
		}
	}

	client := atrust.NewClient(atrust.ClientOptions{
		Session:        atrust.SessionOptions{Username: profile.Auth.Username},
		UnderlayDialer: underlayDialer,
	})

	m.set(State{Phase: PhaseConnecting, Detail: "logging in", Since: time.Now()})

	// A nil handler panics the upstream client when a gateway asks for a
	// challenge, so a configuration without one still gets a handler that
	// refuses with a reason.
	var prompt Prompter = authchallenge.UnsupportedHandler{}
	if m.options.Prompt != nil {
		prompt = m.options.Prompt(ctx)
	}

	if _, err := client.SetupContext(ctx, atrust.SetupOptions{
		ServerAddress:            profile.ServerAddress(),
		ServerPort:               profile.Server.Port,
		LoginMethod:              loginMethod,
		TOTPSecret:               profile.Auth.TOTPSecret,
		ClientData:               clientData,
		ChallengeHandler:         prompt,
		BestNodesRefreshInterval: 5 * time.Minute,
		SessionRefreshInterval:   60 * time.Second,
		SaveClientData: func(data []byte) error {
			return writeClientData(clientDataPath, data)
		},
	}); err != nil {
		client.Close()
		return fail("log in", err)
	}

	m.set(State{Phase: PhaseConnecting, Detail: "opening the data path", Since: time.Now()})

	ipResources, err := client.IPResources()
	if err != nil {
		m.logf("IP resources unavailable: %v", err)
	}
	domainResources, err := client.DomainResources()
	if err != nil {
		m.logf("domain resources unavailable: %v", err)
	}
	dnsResource, err := client.DNSResource()
	if err != nil {
		m.logf("DNS resources unavailable: %v", err)
	}

	vpnStack, err := newStack(client)
	if err != nil {
		client.Close()
		return fail("open the data path", err)
	}

	resolver := resolve.NewResolver(vpnStack, "auto", "auto", 3600, domainResources, dnsResource, true)
	vpnStack.SetupResolve(service.NewDnsServer(resolver, []string{"auto", "auto"}))
	vpnStack.SetupIPPool(resolver.IPPool)
	go vpnStack.Run()

	tunnelIP := ""
	if ip, ipErr := client.IP(); ipErr == nil && ip != nil {
		tunnelIP = ip.String()
	}

	gateway := net.JoinHostPort(profile.ServerAddress(), strconv.Itoa(profile.Server.Port))

	m.mu.Lock()
	m.client = client
	m.resolver = resolver
	m.underlay = underlayDialer
	m.dialer = dial.NewDialer(vpnStack, resolver, ipResources, false, "")
	m.cancel = cancel
	m.mu.Unlock()

	m.set(State{Phase: PhaseOnline, Detail: gateway, ClientIP: tunnelIP, Since: time.Now()})

	go m.runKeepAlive(sessionCtx, profile, ipResources)
	return nil
}

// Logout tears the session down. It is safe to call when idle.
func (m *Manager) Logout() {
	m.login.Lock()
	defer m.login.Unlock()

	m.mu.Lock()
	cancel := m.cancel
	client := m.client
	underlayDialer := m.underlay

	m.cancel = nil
	m.client = nil
	m.dialer = nil
	m.resolver = nil
	m.underlay = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if client != nil {
		client.Close()
	}
	if underlayDialer != nil {
		_ = underlayDialer.Close()
	}
	m.set(State{Phase: PhaseIdle, Detail: "not connected", Since: time.Now()})
}

// Dial opens a connection through the tunnel. It is the proxy layer's only
// dependency on this package.
func (m *Manager) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	m.mu.RLock()
	dialer := m.dialer
	phase := m.state.Phase
	m.mu.RUnlock()

	if phase != PhaseOnline || dialer == nil {
		return nil, ErrNotOnline
	}
	return dialer.Dial(ctx, network, addr)
}

// Resolver exposes the tunnel resolver. It is nil while no session exists.
func (m *Manager) Resolver() *resolve.Resolver {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.resolver
}

// newStack picks the data path the gateway supports. A TCP-only gateway hands
// out no address, so the netstack cannot come up and the TCP tunnel is the only
// option there.
func newStack(client *atrust.Client) (stack.Stack, error) {
	if client.CanUseTCPTunnel() {
		return tcptunnel.NewStack(client)
	}
	if ip, err := client.IP(); err != nil || ip == nil {
		m, tcpErr := tcptunnel.NewStack(client)
		if tcpErr != nil {
			return nil, fmt.Errorf("no tunnel address and no TCP tunnel: %w", tcpErr)
		}
		return m, nil
	}
	gvisorStack, err := gvisor.NewStack(client)
	if err != nil {
		return nil, err
	}
	return gvisorStack, nil
}

// runKeepAlive holds the session open with real application traffic. Without a
// target the upstream loop falls back to a DNS probe, which it warns does not
// extend a session on gateways that expire idle tunnels.
func (m *Manager) runKeepAlive(ctx context.Context, profile config.Profile, ipResources []clientpkg.IPResource) {
	if !profile.Keep.IsEnabled() {
		m.logf("keep-alive is disabled by configuration")
		return
	}

	targets, warnings := keepAliveTargets(profile, ipResources)
	for _, warning := range warnings {
		m.logf("keep-alive: %s", warning)
	}

	m.mu.RLock()
	dialer := m.dialer
	resolver := m.resolver
	m.mu.RUnlock()
	if dialer == nil || resolver == nil {
		return
	}

	m.logf("keep-alive: %d target(s) every %s", len(targets), profile.KeepAliveInterval())
	service.KeepAlive(ctx, resolver, dialer, service.KeepAliveOptions{
		Targets:  targets,
		Interval: profile.KeepAliveInterval(),
	})
}

// keepAliveTargets merges the user's named targets with the ones derived from
// the granted resources.
func keepAliveTargets(profile config.Profile, ipResources []clientpkg.IPResource) ([]service.KeepAliveTarget, []string) {
	var targets []service.KeepAliveTarget
	var warnings []string

	if len(profile.Keep.Targets) > 0 {
		named, namedWarnings := service.NamedKeepAliveTargets(joinTargets(profile.Keep.Targets))
		targets = append(targets, named...)
		warnings = append(warnings, namedWarnings...)
	}
	if len(ipResources) > 0 {
		derived, derivedWarnings := service.DeriveKeepAliveTargets(ipResources)
		targets = append(targets, derived...)
		warnings = append(warnings, derivedWarnings...)
	}
	return targets, warnings
}

func joinTargets(targets []string) string {
	joined := ""
	for index, target := range targets {
		if index > 0 {
			joined += ","
		}
		joined += target
	}
	return joined
}

// config reads the settings in effect.
func (m *Manager) settings() config.Config {
	if m.options.Config == nil {
		return config.Default()
	}
	return m.options.Config()
}

// resolveAuthMethod turns the offered list into the pair to log in with. An
// empty list means the gateway has nothing to ask, so no login method is built
// and the client's own restore path decides whether the stored session is
// enough.
func (m *Manager) resolveAuthMethod(ctx context.Context, methods []AuthMethod, profile config.Profile) (*AuthMethod, error) {
	if len(methods) == 0 {
		m.logf("the gateway offered no authentication method; expecting a stored session")
		return nil, nil
	}

	method, err := ResolveAuthMethod(methods, profile.Auth.AuthType, profile.Auth.Domain)
	if !errors.Is(err, ErrAuthMethodChoice) {
		if err != nil {
			return nil, err
		}
		m.logf("authentication method: %s", method)
		return &method, nil
	}

	if m.options.ChooseAuth == nil {
		return nil, fmt.Errorf("%w: %s; store auth_type and domain in the settings file",
			ErrAuthMethodChoice, methodList(methods))
	}

	chosen, err := m.options.ChooseAuth(ctx, methods)
	if err != nil {
		return nil, err
	}
	if !offered(methods, chosen) {
		return nil, fmt.Errorf("the chosen method %s · %s is not offered", chosen.Type, chosen.Domain)
	}
	m.logf("authentication method: %s", chosen)
	return &chosen, nil
}

// offered reports whether the choice is one of the gateway's methods.
func offered(methods []AuthMethod, chosen AuthMethod) bool {
	for _, method := range methods {
		if method.Type == chosen.Type && method.Domain == chosen.Domain {
			return true
		}
	}
	return false
}

// newLoginMethod builds what the client needs to authenticate. A nil result is
// valid: the client falls back to the stored session material.
func newLoginMethod(cfg config.AuthConfig, method AuthMethod) (auth.LoginMethod, error) {
	switch method.Type {
	case "auth/psw", "auth/cas", "auth/httpsOauth2":
	default:
		return nil, fmt.Errorf("authentication type %s is not supported as the primary method; "+
			"goTrust logs in with a password, CAS or OAuth2", method.Type)
	}
	return auth.NewLoginMethod(auth.LoginMethodOptions{
		AuthType: method.Type,
		Username: cfg.Username,
		Password: cfg.Password,
		Domain:   method.Domain,
	})
}

func (m *Manager) set(state State) {
	m.mu.Lock()
	m.state = state
	m.mu.Unlock()

	if m.options.OnState != nil {
		m.options.OnState(state)
	}
}

func (m *Manager) fail(err error) {
	m.logf("session failed: %v", err)
	m.set(State{Phase: PhaseFailed, Detail: err.Error(), Since: time.Now()})
}

// readClientData loads the stored session material. A missing file is normal on
// a first run.
func readClientData(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return raw, nil
}

// writeClientData stores the session material. The file carries cookies, the
// sign key and the sid ticket, so it is restricted to its owner.
func writeClientData(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	// WriteFile does not change the mode of an existing file, and an older
	// version may have created it readable by others.
	return os.Chmod(path, 0o600)
}
