package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/YangtseSu/goTrust/core/atrust/log"
	"github.com/YangtseSu/goTrust/core/atrust/resolve"
)

const (
	keepAliveRequestTimeout = 10 * time.Second
	keepAliveDrainLimit     = 32 << 10

	// candidateKeepAliveTimeout is the short leash a derived candidate gets: it is
	// a guess, and the round moves on to the next candidate on a miss.
	candidateKeepAliveTimeout = 3 * time.Second
)

// routedTunnel and routedDirect are what a probe reports about its own path:
// only the first is the application traffic that holds a session open.
const (
	routedTunnel = "tunnel"
	routedDirect = "direct"
)

// KeepAliveTarget is one address to probe. An HTTP target carries a URL; a TCP
// target carries an address and is probed with the exchange its port implies: a
// TLS handshake on 443, the SSH identification on 22, a bare connection
// otherwise. The two forms are exclusive.
type KeepAliveTarget struct {
	URL     string
	Address string
	Named   bool
}

// String is what the log, the state and the UI call this target.
func (t KeepAliveTarget) String() string {
	if t.Address != "" {
		return "tcp://" + t.Address
	}
	return t.URL
}

// KeepAliveDialer is the part of dial.Dialer the keep-alive needs, so a test can
// substitute its own.
type KeepAliveDialer interface {
	Dial(ctx context.Context, network, address string) (net.Conn, error)
}

// RouteReporter is what a dialer that can answer for itself implements: whether
// Dial would send this destination through the tunnel. A keep-alive probe is
// only application traffic when it crosses the tunnel, so an answer that came
// back over a direct connection is a warning, not a successful probe.
type RouteReporter interface {
	RouteFor(ctx context.Context, network, addr string) (bool, error)
}

// KeepAliveResult is the outcome of one probe. A caller that wants to show
// what the last round learned (a tray tooltip, a status line) reads this
// instead of matching the log lines the loop writes.
type KeepAliveResult struct {
	// Target is the probe's name: an HTTP URL, tcp://host:port for a TCP
	// target, or the name of the remote-DNS fallback probe.
	Target string
	// Named reports whether Target came from the user's configured list.
	Named bool
	// StatusCode is the HTTP answer, zero for a transport error, a TCP target
	// or a DNS probe.
	StatusCode int
	// OK reports whether the probe counted: a 2xx/3xx HTTP answer, an
	// established TCP connection, or a successful name lookup in the fallback.
	OK bool
	// Err is the transport or lookup error, nil when the probe got an answer.
	Err error
	// Fallback marks the remote-DNS probe, which is not application traffic
	// and does not hold a session open.
	Fallback bool
	// Routed is how the probe left: routedTunnel for a connection the dialer
	// sent through the tunnel, routedDirect for one it did not, and empty when
	// the dialer could not say. A direct probe does not keep the session open,
	// so it is never OK.
	Routed string
	// At is when the probe finished.
	At time.Time
}

// KeepAliveOptions carries what the loop needs from its caller.
type KeepAliveOptions struct {
	// Targets to probe, in preference order.
	Targets []KeepAliveTarget
	// Interval between rounds.
	Interval time.Duration
	// OnResult observes every probe, including the remote-DNS fallback. It is
	// called from the keep-alive goroutine and must not block; nil discards
	// the results.
	OnResult func(KeepAliveResult)
}

// KeepAlive keeps the session open by sending real application traffic through
// the tunnel: on this deployment a periodic session check does not extend the
// session at all, only traffic to a granted application does.
//
// An HTTP grant is probed with a plain request. Any other TCP grant is probed
// with a connection through the tunnel, followed by the exchange its port
// implies, because a socket that is opened and closed may not count as
// application traffic on every gateway.
//
// With no target at all it falls back to the older remote-DNS probe and says so,
// because a name lookup is not application traffic and does not hold the session.
func KeepAlive(ctx context.Context, resolver *resolve.Resolver, dialer KeepAliveDialer, options KeepAliveOptions) {
	report := options.OnResult
	if report == nil {
		report = func(KeepAliveResult) {}
	}

	if len(options.Targets) == 0 {
		log.Println("KeepAlive: no application target, falling back to the remote-DNS probe " +
			"(this is not application traffic and does not hold the session open; " +
			"set keepalive.targets or grant the account an HTTP or TCP application)")
		keepAliveWithRemoteDNS(ctx, resolver, options.Interval, report)
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.Dial,
			ResponseHeaderTimeout: keepAliveRequestTimeout,
			// Each probe opens its own connection: an idle tunnel connection
			// between probes buys nothing and hides routing changes, and the
			// userspace netstack drops idle connections by itself anyway.
			DisableKeepAlives: true,
		},
	}
	defer client.CloseIdleConnections()

	active := ""
	for {
		if target, ok := keepAliveRound(ctx, client, dialer, options.Targets, active, report); ok {
			active = target
		}
		if !sleep(ctx, options.Interval) {
			return
		}
	}
}

// keepAliveRound tries each target once, in preference order, and returns the
// first one that answered. A target that fails is not retried: the next round is
// the retry, and logging one honest line per attempt is all the caller needs.
func keepAliveRound(
	ctx context.Context,
	client *http.Client,
	dialer KeepAliveDialer,
	targets []KeepAliveTarget,
	active string,
	report func(KeepAliveResult),
) (string, bool) {
	for _, target := range keepAliveOrder(targets, active) {
		timeout := candidateKeepAliveTimeout
		if target.Named {
			timeout = keepAliveRequestTimeout
		}
		if keepAliveAttempt(ctx, client, dialer, target, timeout, report) {
			return target.String(), true
		}
	}
	return "", false
}

func keepAliveAttempt(
	ctx context.Context,
	client *http.Client,
	dialer KeepAliveDialer,
	target KeepAliveTarget,
	timeout time.Duration,
	report func(KeepAliveResult),
) bool {
	if target.Address != "" {
		return tcpKeepAliveAttempt(ctx, dialer, target, timeout, report)
	}
	return httpKeepAliveAttempt(ctx, client, dialer, target, timeout, report)
}

func httpKeepAliveAttempt(
	ctx context.Context,
	client *http.Client,
	dialer KeepAliveDialer,
	target KeepAliveTarget,
	timeout time.Duration,
	report func(KeepAliveResult),
) bool {
	result := KeepAliveResult{Target: target.String(), Named: target.Named}
	finish := func() {
		result.At = time.Now()
		report(result)
	}

	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target.URL, nil)
	if err != nil {
		result.Err = err
		finish()
		log.Printf("KeepAlive: target=%s err=%v", target.URL, err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Routed = probeRoute(ctx, dialer, target)
		result.Err = err
		finish()
		log.Printf("KeepAlive: target=%s routed=%s err=%v", target.URL, orUnknown(result.Routed), err)
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, keepAliveDrainLimit+1))

	// Any HTTP answer is an application request the gateway carried, so it
	// renews the session whatever the status says: a 403 from an application
	// that wants a login is still traffic through the tunnel. The status stays
	// in the result, because it is what a status line shows.
	result.StatusCode = resp.StatusCode
	result.Routed = probeRoute(ctx, dialer, target)

	if result.Routed == routedDirect {
		finish()
		log.Printf("KeepAlive: target=%s status=%d routed=direct; the answer did not cross the tunnel, so the session is not renewed",
			target.URL, resp.StatusCode)
		return false
	}

	result.OK = true
	finish()
	log.Printf("KeepAlive: OK status=%d routed=%s target=%s", resp.StatusCode, orUnknown(result.Routed), target.URL)
	return true
}

// probeRoute asks the dialer where an HTTP probe went. A dialer that cannot
// answer leaves it empty: a plain HTTP client used in a test carries no route,
// and claiming either path would be a guess.
func probeRoute(ctx context.Context, dialer KeepAliveDialer, target KeepAliveTarget) string {
	targetURL, err := url.Parse(target.URL)
	if err != nil {
		return ""
	}
	port := targetURL.Port()
	if port == "" {
		port = "80"
		if targetURL.Scheme == "https" {
			port = "443"
		}
	}
	return routeOf(ctx, dialer, net.JoinHostPort(targetURL.Hostname(), port))
}

// routeOf is the route query both kinds of probe share.
func routeOf(ctx context.Context, dialer KeepAliveDialer, address string) string {
	reporter, ok := dialer.(RouteReporter)
	if !ok {
		return ""
	}
	if routed, err := reporter.RouteFor(ctx, "tcp", address); err == nil && routed {
		return routedTunnel
	}
	return routedDirect
}

// orUnknown prints the route for a log line.
func orUnknown(routed string) string {
	if routed == "" {
		return "unknown"
	}
	return routed
}

// tcpKeepAliveAttempt opens a connection to a granted address through the
// tunnel. The connection itself is the traffic that holds the session; the
// application exchange that follows is best effort, because a service that
// requires a client certificate, or a non-TLS daemon on 443, must not turn a
// live tunnel into a failed probe.
func tcpKeepAliveAttempt(
	ctx context.Context,
	dialer KeepAliveDialer,
	target KeepAliveTarget,
	timeout time.Duration,
	report func(KeepAliveResult),
) bool {
	result := KeepAliveResult{Target: target.String(), Named: target.Named}
	finish := func() {
		result.At = time.Now()
		report(result)
	}

	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := dialer.Dial(attemptCtx, "tcp", target.Address)
	if err != nil {
		result.Routed = routeOf(ctx, dialer, target.Address)
		result.Err = err
		finish()
		log.Printf("KeepAlive: target=%s routed=%s err=%v", target.String(), orUnknown(result.Routed), err)
		return false
	}
	defer func() { _ = conn.Close() }()

	if err := applicationExchange(attemptCtx, conn, target.Address); err != nil {
		log.Printf("KeepAlive: target=%s connected; application exchange: %v", target.String(), err)
	}

	result.Routed = routeOf(ctx, dialer, target.Address)
	if result.Routed == routedDirect {
		finish()
		log.Printf("KeepAlive: target=%s connected directly, not through the tunnel; the session is not renewed",
			target.String())
		return false
	}

	result.OK = true
	finish()
	log.Printf("KeepAlive: OK tcp routed=%s target=%s", orUnknown(result.Routed), target.String())
	return true
}

// applicationExchange sends or reads the first bytes of the protocol the port
// implies. It proves the service speaks that protocol, never who it is, and it
// is not what decides the probe.
func applicationExchange(ctx context.Context, conn net.Conn, address string) error {
	_, portText, err := net.SplitHostPort(address)
	if err != nil {
		return nil
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	switch port {
	case derivedTLSPort:
		// The certificate is irrelevant to a liveness probe: the handshake only
		// has to reach the server and come back.
		return tls.Client(conn, &tls.Config{InsecureSkipVerify: true}).HandshakeContext(ctx)
	case derivedSSHPort:
		return readSSHIdentification(conn)
	}
	return nil
}

// readSSHIdentification reads the server's identification string, which RFC
// 4253 requires it to send as soon as the connection is established.
func readSSHIdentification(conn net.Conn) error {
	const prefix = "SSH-"
	buf := make([]byte, 64)
	var banner []byte
	for len(banner) < len(prefix) {
		n, err := conn.Read(buf)
		banner = append(banner, buf[:n]...)
		if err != nil {
			return err
		}
	}
	if !bytes.HasPrefix(banner, []byte(prefix)) {
		return fmt.Errorf("the server answered %q instead of an SSH identification", banner)
	}
	return nil
}

func sleep(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// keepAliveWithRemoteDNS is the fallback when no application target exists. It
// keeps the historical behaviour and its log lines; it is deliberately labelled
// as not being application traffic.
func keepAliveWithRemoteDNS(
	ctx context.Context,
	resolver *resolve.Resolver,
	interval time.Duration,
	report func(KeepAliveResult),
) {
	reportFallback := func(protocol string, err error) {
		report(KeepAliveResult{
			Target:   "www.baidu.com (" + protocol + ")",
			OK:       err == nil,
			Err:      err,
			Fallback: true,
			At:       time.Now(),
		})
	}

	remoteUDPResolver, err := resolver.RemoteUDPResolver()
	if err != nil {
		log.Printf("KeepAlive: %s", err)
	}

	remoteTCPResolver, err := resolver.RemoteTCPResolver()
	if err != nil {
		log.Printf("KeepAlive: %s", err)
	}

	if remoteUDPResolver == nil && remoteTCPResolver == nil {
		log.Printf("KeepAlive: No remote resolver available")
		report(KeepAliveResult{
			Target:   "remote DNS",
			Err:      errors.New("no remote resolver available"),
			Fallback: true,
			At:       time.Now(),
		})
		return
	}

	for {
		useTCP := false
		requestCtx, cancel := context.WithTimeout(ctx, keepAliveRequestTimeout)

		if remoteUDPResolver != nil {
			_, err := remoteUDPResolver.LookupIP(requestCtx, "ip4", "www.baidu.com")
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					log.DebugPrintf("KeepAlive using UDP error: %s", err)
				}
				useTCP = true
			} else {
				log.Printf("KeepAlive using UDP: OK")
			}
			reportFallback("UDP", err)
		}

		if useTCP && remoteTCPResolver != nil {
			_, err := remoteTCPResolver.LookupIP(requestCtx, "ip4", "www.baidu.com")
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					log.Printf("KeepAlive using TCP error: %s", err)
				}
			} else {
				log.Printf("KeepAlive using TCP: OK")
			}
			reportFallback("TCP", err)
		}
		cancel()

		if !sleep(ctx, interval) {
			return
		}
	}
}
