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
	"sync/atomic"
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
	// PhaseReconnecting means a lost session is being retried with a fresh
	// login, or the client is renewing or restoring its own live session. Dial
	// keeps serving while the client still holds a data path (a restore is
	// confirmed by traffic), and refuses once the ladder has torn it down.
	PhaseReconnecting Phase = "reconnecting"
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
	// Resources is what the gateway granted this account. It is retained from
	// the login that established the tunnel and cleared by Logout.
	Resources Resources
	// KeepAlive is the last application-traffic probe.
	KeepAlive KeepAliveState
	// Reconnect is the ladder position while a lost session is retried.
	Reconnect ReconnectState
	// Attention asks the UI to bring itself forward: the login needs a person,
	// or the session expired and cannot recover on its own.
	Attention string
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
	// ResourceCachePath is where the last granted resources are cached for
	// --print-resources. Empty disables the cache.
	ResourceCachePath string
	// OnState is called on every transition, from the goroutine that caused it.
	OnState func(State)
}

// tunnelDialer opens one connection through the session's live data path. It
// is an interface only so tests can hold a tunnel open; the real value is the
// upstream dialer.
type tunnelDialer interface {
	Dial(ctx context.Context, network, addr string) (net.Conn, error)
}

// Manager holds the live session and serialises login, logout and dialling.
type Manager struct {
	options Options
	logf    func(format string, args ...any)

	mu       sync.RWMutex
	state    State
	client   *atrust.Client
	dialer   tunnelDialer
	resolver *resolve.Resolver
	underlay *underlay.Dialer
	cancel   context.CancelFunc
	login    sync.Mutex

	// challenge records that the login asked the user something during the
	// current attempt, so the ladder stops instead of retrying blindly.
	challenge atomic.Bool
	// reconnectMu guards the ladder's cancel function, which is nil when no
	// ladder runs.
	reconnectMu     sync.Mutex
	reconnectCancel context.CancelFunc
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
// other clients of the same account. While the auto-reconnect ladder runs it is
// refused too, because the ladder owns the next attempt.
func (m *Manager) Login(ctx context.Context) error {
	if m.reconnectRunning() {
		return errors.New("auto-reconnect is already retrying the session")
	}
	return m.loginSession(ctx)
}

// loginSession is the login itself. The ladder calls it directly: it is the
// owner of the attempts, so the reconnect guard in Login does not apply.
func (m *Manager) loginSession(ctx context.Context) error {
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
	// means the gateway has nothing to ask (it already sees this client as
	// logged in), so the client continues with the stored cookies and lets the
	// portal answer. A session never outlives its connection: after a
	// disconnect the gateway offers the methods again and a login is made.
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
	// refuses with a reason. The wrapper marks the attempt as needing a person,
	// which is what stops a reconnect ladder at the prompt.
	var prompt Prompter = authchallenge.UnsupportedHandler{}
	if m.options.Prompt != nil {
		prompt = observingPrompter{
			inner:  m.options.Prompt(ctx),
			notify: func() { m.challenge.Store(true) },
		}
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
		OnSessionState: func(state atrust.SessionState, reason string) {
			m.observeSessionState(client, state, reason)
		},
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
	apps, err := client.Apps()
	if err != nil {
		m.logf("applications unavailable: %v", err)
	}
	resources := buildResources(ipResources, domainResources, dnsResource, apps)
	if resources.Empty() {
		m.logf("the gateway granted no resources to this account")
	} else {
		m.logf("the gateway granted %d displayable resources", len(resources.Entries))
	}

	vpnStack, err := m.newStack(client)
	if err != nil {
		client.Close()
		return fail("open the data path", err)
	}

	remoteDNSServer, secondaryDNSServer, useRemoteDNS := m.tunnelDNS(client)
	resolver := resolve.NewResolver(vpnStack, remoteDNSServer, secondaryDNSServer, 3600, domainResources, dnsResource, useRemoteDNS)
	vpnStack.SetupResolve(service.NewDnsServer(resolver, []string{remoteDNSServer, secondaryDNSServer}))
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

	m.set(State{Phase: PhaseOnline, Detail: gateway, ClientIP: tunnelIP, Resources: resources, Since: time.Now()})
	m.cacheResources(profile, gateway, tunnelIP, resources)

	go m.runKeepAlive(sessionCtx, profile, ipResources)
	return nil
}

// cacheResources stores the last grant beside the settings, so
// --print-resources works without another login. A cache that cannot be
// written is a convenience lost, not a login failure.
func (m *Manager) cacheResources(profile config.Profile, gateway, tunnelIP string, resources Resources) {
	if m.options.ResourceCachePath == "" {
		return
	}
	snapshot := ResourceSnapshot{
		SavedAt:   time.Now(),
		Profile:   profile.Name,
		Gateway:   gateway,
		TunnelIP:  tunnelIP,
		Resources: resources,
	}
	if err := WriteResourceCache(m.options.ResourceCachePath, snapshot); err != nil {
		m.logf("cache the granted resources: %v", err)
	}
}

// Logout tears the session down. It is safe to call when idle.
func (m *Manager) Logout() {
	m.stopReconnect()
	m.login.Lock()
	defer m.login.Unlock()

	m.teardown()
	m.set(State{Phase: PhaseIdle, Detail: "not connected", Since: time.Now()})
}

// teardown releases the live client and its data path. It does not publish a
// phase: the caller decides whether the result is idle, failed or a retry.
func (m *Manager) teardown() {
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
}

// Dial opens a connection through the tunnel. It is the proxy layer's only
// dependency on this package.
func (m *Manager) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	m.mu.RLock()
	dialer := m.dialer
	phase := m.state.Phase
	m.mu.RUnlock()

	// A reconnecting session has two shapes. While the client renews or
	// restores its own session the data path is still there, and a restored
	// session stays "restoring" until real traffic confirms it: refusing here
	// would keep the proxy from ever supplying that traffic. Once the ladder
	// starts a fresh login it has torn the tunnel down, and the nil dialer is
	// what refuses.
	if dialer == nil || (phase != PhaseOnline && phase != PhaseReconnecting) {
		return nil, ErrNotOnline
	}
	return dialer.Dial(ctx, network, addr)
}

// Resolve resolves a name through the live session. It is read per call, not
// captured: an auto-reconnect replaces the resolver together with the stack it
// was built on, while the SOCKS5 listener outlives both.
func (m *Manager) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	m.mu.RLock()
	resolver := m.resolver
	m.mu.RUnlock()

	if resolver == nil {
		return ctx, nil, ErrNotOnline
	}
	return resolver.Resolve(ctx, name)
}

// newStack picks the data path the gateway supports. The packet stack is the
// default because it carries UDP as well as TCP, which remote DNS needs, and
// it needs the address the gateway assigned. A TCP-only gateway assigns none,
// so the TCP tunnel is the only option there. Its DialUDP is unimplemented, so
// a packet stack that fails to come up still leaves a working TCP path.
func (m *Manager) newStack(client clientpkg.Client) (stack.Stack, error) {
	if ip, err := client.IP(); err != nil || ip == nil {
		return tcptunnel.NewStack(client)
	}
	vpnStack, err := gvisor.NewStack(client)
	if err != nil {
		m.logf("the packet stack is unavailable (%v), using the TCP tunnel", err)
		return tcptunnel.NewStack(client)
	}
	return vpnStack, nil
}

// tunnelDNS returns the resolvers the tunnel should use. The gateway names its
// own servers in the login reply; "auto" is a placeholder, not an address, and
// dialling it through net.ParseIP would silently target 0.0.0.0:53. Without a
// usable name the tunnel keeps the system resolver for everything the gateway
// did not map itself.
func (m *Manager) tunnelDNS(client clientpkg.Client) (remote, secondary string, useRemote bool) {
	remote, err := client.DNSServer()
	if err != nil || net.ParseIP(remote) == nil {
		m.logf("the gateway named no usable DNS server: remote DNS is off")
		return "", "", false
	}

	// The second name is the fallback for everything the tunnel resolver does
	// not answer itself. Leave it empty when the gateway offers only one, so
	// the resolver falls back to the system configuration instead of a
	// hard-coded public server.
	if servers, err := client.DNSServers(); err == nil && len(servers) > 1 && net.ParseIP(servers[1]) != nil {
		secondary = servers[1]
	}
	return remote, secondary, true
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
		OnResult: m.recordKeepAlive,
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
// empty list means the gateway has nothing to ask, so no login method is built;
// the client continues with the stored cookies and the portal answers.
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

	m.emit(state)
}

// setTransition replaces the state and stamps Since only when the phase changes:
// the reconnect countdown republishes the same phase every second, and a
// timestamp that moves with it would claim the phase restarted.
func (m *Manager) setTransition(state State) {
	m.mu.Lock()
	if m.state.Phase == state.Phase {
		state.Since = m.state.Since
	} else {
		state.Since = time.Now()
	}
	m.state = state
	m.mu.Unlock()

	m.emit(state)
}

func (m *Manager) emit(state State) {
	if m.options.OnState != nil {
		m.options.OnState(state)
	}
}

// recordKeepAlive publishes the last probe without disturbing the phase the
// session is in: a failed probe is evidence, not a state transition.
func (m *Manager) recordKeepAlive(result service.KeepAliveResult) {
	keepAlive := KeepAliveState{
		Target:   result.Target,
		Named:    result.Named,
		Status:   result.StatusCode,
		OK:       result.OK,
		Fallback: result.Fallback,
		At:       result.At,
	}
	if result.Err != nil {
		keepAlive.Err = result.Err.Error()
	}

	m.mu.Lock()
	state := m.state
	state.KeepAlive = keepAlive
	m.state = state
	m.mu.Unlock()

	m.emit(state)
}

// tunnelIP reports the address the live client holds, empty when there is none.
func (m *Manager) tunnelIP() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.ClientIP
}

// resources reports the last grant, so a reconnecting state does not flash an
// empty list.
func (m *Manager) resources() Resources {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Resources
}

// gatewayAddress is host:port in the form the window shows when online.
func (m *Manager) gatewayAddress() string {
	profile := m.settings().Active()
	return net.JoinHostPort(profile.ServerAddress(), strconv.Itoa(profile.Server.Port))
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
