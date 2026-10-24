// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"inet.af/netaddr"

	clientpkg "github.com/mythologyli/zju-connect/client"
	"github.com/mythologyli/zju-connect/resolve"
	"github.com/mythologyli/zju-connect/stack/gvisor"
	"github.com/mythologyli/zju-connect/stack/tcptunnel"

	"github.com/YangtseSu/goTrust/config"
)

// A manager that was never logged in must refuse to dial instead of handing
// back a dialer that does not exist: the proxy layer starts before the session
// is up far more often than it looks.
func TestDialRefusesWhileIdle(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})

	_, err := manager.Dial(context.Background(), "tcp", "example.com:80")
	if !errors.Is(err, ErrNotOnline) {
		t.Fatalf("err = %v, want ErrNotOnline", err)
	}
}

func TestInitialStateIsIdle(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})

	state := manager.State()
	if state.Phase != PhaseIdle {
		t.Fatalf("phase = %q, want %q", state.Phase, PhaseIdle)
	}
	if state.Since.IsZero() {
		t.Fatal("the initial state has no timestamp")
	}
}

// Invalid settings must fail before anything is dialled, and must leave the
// manager in the failed phase with the reason attached.
func TestLoginRejectsInvalidConfig(t *testing.T) {
	empty := config.Default() // no server address, no username

	manager := New(Options{Config: func() config.Config { return empty }})
	err := manager.Login(context.Background())
	if err == nil {
		t.Fatal("login succeeded with an empty server address")
	}

	state := manager.State()
	if state.Phase != PhaseFailed {
		t.Fatalf("phase = %q, want %q", state.Phase, PhaseFailed)
	}
	if state.Detail == "" {
		t.Fatal("the failed state carries no reason")
	}
}

// A second login while one is in flight must be refused rather than silently
// replacing the first: a full login rotates the device token.
func TestLoginRefusesWhileOnline(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})
	manager.set(State{Phase: PhaseOnline, Detail: "test", Since: time.Now()})

	err := manager.Login(context.Background())
	if err == nil {
		t.Fatal("a second login was accepted while online")
	}
}

// State changes reach the observer, which is what keeps the window honest.
func TestStateChangesAreReported(t *testing.T) {
	var seen []Phase
	manager := New(Options{
		Config: func() config.Config { return config.Default() },
		OnState: func(state State) {
			seen = append(seen, state.Phase)
		},
	})

	manager.set(State{Phase: PhaseConnecting, Detail: "x", Since: time.Now()})
	manager.set(State{Phase: PhaseOnline, Detail: "y", Since: time.Now()})

	if len(seen) != 2 || seen[0] != PhaseConnecting || seen[1] != PhaseOnline {
		t.Fatalf("observed phases = %v", seen)
	}
}

// The settings are read at login time, not captured at construction: the
// settings window can switch profiles after the manager exists.
func TestSettingsAndProfileAreReadAtLoginTime(t *testing.T) {
	cfg := config.Default()
	manager := New(Options{Config: func() config.Config { return cfg }})

	if got := manager.settings().Active().Auth.Username; got != "" {
		t.Fatalf("username = %q, want empty", got)
	}

	cfg.Profiles[0].Auth.Username = "second"
	if got := manager.settings().Active().Auth.Username; got != "second" {
		t.Fatalf("username = %q, want second", got)
	}
}

// Logout is idempotent: the UI calls it on quit even when nothing connected.
func TestLogoutIsSafeWhenIdle(t *testing.T) {
	manager := New(Options{Config: func() config.Config { return config.Default() }})

	manager.Logout()
	manager.Logout()

	if state := manager.State(); state.Phase != PhaseIdle {
		t.Fatalf("phase = %q, want %q", state.Phase, PhaseIdle)
	}
}

// Keep-alive targets come from the configuration first and from the granted
// resources second, so a user-specified target always wins placement.
func TestKeepAliveTargetsMergeNamedAndDerived(t *testing.T) {
	profile := config.Default().Active()
	profile.Keep.Targets = []string{"http://192.0.2.5/health"}

	targets, _ := keepAliveTargets(profile, nil)
	if len(targets) != 1 {
		t.Fatalf("targets = %v, want one", targets)
	}
	if targets[0].URL != "http://192.0.2.5/health" || !targets[0].Named {
		t.Fatalf("target = %+v, want the named target", targets[0])
	}

	empty := config.Default().Active()
	empty.Keep.Targets = nil
	targets, _ = keepAliveTargets(empty, nil)
	if len(targets) != 0 {
		t.Fatalf("targets = %v, want none without a config or resources", targets)
	}
}

func TestJoinTargets(t *testing.T) {
	if got := joinTargets(nil); got != "" {
		t.Fatalf("join(nil) = %q", got)
	}
	if got := joinTargets([]string{"a", "b"}); got != "a,b" {
		t.Fatalf("join = %q, want a,b", got)
	}
}

// A first login has no config directory yet: writing the stored session must
// create it, so a fresh install keeps its ticket.
func TestWriteClientDataCreatesAPrivateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "client_data.json")
	if err := writeClientData(path, []byte(`{"sid":"x"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0o600 {
		t.Fatalf("client data mode = %o, want 600", mode)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("client data directory mode = %o, want 700", mode)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != `{"sid":"x"}` {
		t.Fatalf("data = %q, %v", raw, err)
	}

	// The write goes through a temporary file: nothing may be left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read the directory: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != filepath.Base(path) {
			t.Fatalf("leftover file after the write: %s", entry.Name())
		}
	}

	// A rewrite replaces the file instead of truncating the ticket in place.
	if err := writeClientData(path, []byte(`{"sid":"y"}`)); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != `{"sid":"y"}` {
		t.Fatalf("rewritten data = %q, %v", raw, err)
	}
}

// stubClient is the part of the upstream client the data-path choice reads. A
// real login cannot be made in a test, so the decision itself is exercised
// through this.
type stubClient struct {
	ip         net.IP
	dnsServer  string
	dnsServers []string
}

func (c stubClient) IP() (net.IP, error) {
	if c.ip == nil {
		return nil, errors.New("IP not available")
	}
	return c.ip, nil
}

func (c stubClient) IPSet() (*netaddr.IPSet, error) { return nil, errors.New("IP set not available") }

func (c stubClient) IPResources() ([]clientpkg.IPResource, error) { return nil, nil }

func (c stubClient) DomainResources() (clientpkg.DomainResources, error) { return nil, nil }

func (c stubClient) DNSResource() (map[string][]net.IP, error) { return nil, nil }

func (c stubClient) DNSServer() (string, error) {
	if c.dnsServer == "" {
		return "", errors.New("DNS server not available")
	}
	return c.dnsServer, nil
}

func (c stubClient) DNSServers() ([]string, error) {
	if len(c.dnsServers) == 0 {
		return nil, errors.New("DNS servers not available")
	}
	return c.dnsServers, nil
}

func (c stubClient) CanUseTCPTunnel() bool { return true }

func (c stubClient) DialTCP(context.Context, *net.TCPAddr) (net.Conn, error) {
	return nil, errors.New("not used in this test")
}

func (c stubClient) NewL3Conn() (io.ReadWriteCloser, error) {
	return nil, errors.New("not used in this test")
}

// A gateway that assigns an address can carry UDP, which remote DNS needs, so
// the packet stack has to win over the TCP tunnel there. The upstream
// CanUseTCPTunnel reports true unconditionally and must not decide this.
func TestNewStackPrefersThePacketStackWhenAnAddressExists(t *testing.T) {
	manager := New(Options{})

	vpnStack, err := manager.newStack(stubClient{ip: net.ParseIP("192.0.2.10")})
	if err != nil {
		t.Fatalf("new stack: %v", err)
	}
	if _, ok := vpnStack.(*gvisor.Stack); !ok {
		t.Fatalf("stack = %T, want the packet stack", vpnStack)
	}
}

// A TCP-only gateway assigns no address, so the netstack cannot come up: the
// TCP tunnel is the only usable path and must not fail the login.
func TestNewStackFallsBackToTheTCPTunnelWithoutAnAddress(t *testing.T) {
	manager := New(Options{})

	vpnStack, err := manager.newStack(stubClient{})
	if err != nil {
		t.Fatalf("new stack: %v", err)
	}
	if _, ok := vpnStack.(*tcptunnel.Stack); !ok {
		t.Fatalf("stack = %T, want the TCP tunnel", vpnStack)
	}
}

// The gateway names its own resolvers; both have to reach the resolver as
// addresses. The login reply can carry the literal "auto", which net.ParseIP
// turns into an unspecified address instead of an error.
func TestTunnelDNSStopsAtTheLiteralAuto(t *testing.T) {
	manager := New(Options{})

	remote, secondary, useRemote := manager.tunnelDNS(stubClient{
		dnsServer:  "auto",
		dnsServers: []string{"auto", "auto"},
	})
	if remote != "" || secondary != "" || useRemote {
		t.Fatalf("remote = %q, secondary = %q, useRemote = %v, want no remote DNS", remote, secondary, useRemote)
	}
}

func TestTunnelDNSTakesBothServersFromTheGateway(t *testing.T) {
	manager := New(Options{})

	remote, secondary, useRemote := manager.tunnelDNS(stubClient{
		dnsServer:  "192.0.2.53",
		dnsServers: []string{"192.0.2.53", "203.0.113.53"},
	})
	if !useRemote || remote != "192.0.2.53" || secondary != "203.0.113.53" {
		t.Fatalf("remote = %q, secondary = %q, useRemote = %v, want both gateway servers", remote, secondary, useRemote)
	}
}

// One named server is enough for the tunnel; the second name stays empty so
// the resolver keeps the system configuration for everything else instead of a
// hard-coded public server.
func TestTunnelDNSLeavesTheSecondaryToTheSystem(t *testing.T) {
	manager := New(Options{})

	remote, secondary, useRemote := manager.tunnelDNS(stubClient{
		dnsServer:  "192.0.2.53",
		dnsServers: []string{"192.0.2.53"},
	})
	if !useRemote || remote != "192.0.2.53" || secondary != "" {
		t.Fatalf("remote = %q, secondary = %q, useRemote = %v, want the gateway server only", remote, secondary, useRemote)
	}
}

// stubTunnelDialer stands in for the upstream dialer: Dial must reach it only
// while the session still owns a data path.
type stubTunnelDialer struct {
	calls int
}

func (d *stubTunnelDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	d.calls++
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}

// A restored session stays "restoring" until real traffic confirms it. Refusing
// to dial there would stop the only traffic that can confirm it, so a
// reconnecting session that still holds a dialer has to serve.
func TestDialServesAReconnectingSessionThatStillHasATunnel(t *testing.T) {
	manager := New(Options{})
	dialer := &stubTunnelDialer{}
	manager.dialer = dialer
	manager.set(State{Phase: PhaseReconnecting, Detail: "正在恢复会话", Since: time.Now()})

	conn, err := manager.Dial(context.Background(), "tcp", "192.0.2.10:443")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()
	if dialer.calls != 1 {
		t.Fatalf("dialer calls = %d, want 1", dialer.calls)
	}
}

// The reconnect ladder tears the dead tunnel down before its first attempt.
// That state is also PhaseReconnecting, and it must still refuse.
func TestDialRefusesReconnectingWithoutATunnel(t *testing.T) {
	manager := New(Options{})
	manager.set(State{Phase: PhaseReconnecting, Detail: "重连中", Since: time.Now()})

	if _, err := manager.Dial(context.Background(), "tcp", "192.0.2.10:443"); !errors.Is(err, ErrNotOnline) {
		t.Fatalf("err = %v, want ErrNotOnline", err)
	}
}

// resolverFor builds a tunnel resolver that answers one name from the gateway's
// DNS resource list, so a lookup needs no stack or network.
func resolverFor(host string, ip net.IP) *resolve.Resolver {
	return resolve.NewResolver(
		nil, "192.0.2.53", "", 3600,
		nil,
		map[string][]net.IP{host: {ip}},
		true,
	)
}

// The proxy outlives an auto-reconnect, and the reconnect replaces the resolver
// together with the stack it was built on. A manager that resolved through the
// resolver it held at login would answer from the dead stack after every retry.
func TestResolveFollowsTheReconnectedSession(t *testing.T) {
	manager := New(Options{})
	manager.resolver = resolverFor("intranet.example.com", net.ParseIP("192.0.2.7"))

	_, ip, err := manager.Resolve(context.Background(), "intranet.example.com")
	if err != nil || !ip.Equal(net.ParseIP("192.0.2.7")) {
		t.Fatalf("ip = %v, err = %v, want the first session's answer", ip, err)
	}

	manager.resolver = resolverFor("intranet.example.com", net.ParseIP("192.0.2.8"))

	_, ip, err = manager.Resolve(context.Background(), "intranet.example.com")
	if err != nil || !ip.Equal(net.ParseIP("192.0.2.8")) {
		t.Fatalf("ip = %v, err = %v, want the reconnected session's answer", ip, err)
	}
}

func TestResolveRefusesWithoutASession(t *testing.T) {
	manager := New(Options{})

	if _, _, err := manager.Resolve(context.Background(), "intranet.example.com"); !errors.Is(err, ErrNotOnline) {
		t.Fatalf("err = %v, want ErrNotOnline", err)
	}
}
