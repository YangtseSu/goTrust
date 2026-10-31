package service

import (
	"bytes"
	"context"
	"errors"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/atrust/resolve"
)

// fakeDialer stands in for the tunnel. The keep-alive only opens connections
// through it, so a test can make it fail the way the gateway does.
type fakeDialer struct {
	dial func(ctx context.Context, network, address string) (net.Conn, error)
}

func (d fakeDialer) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	if d.dial == nil {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	return d.dial(ctx, network, address)
}

func keepAliveClientFor(dialer KeepAliveDialer) *http.Client {
	return &http.Client{
		Transport: &http.Transport{DialContext: dialer.Dial, DisableKeepAlives: true},
	}
}

// TestKeepAliveRoundUsesTheFirstAnsweringTarget: each target is tried once, in
// preference order; a failed target is not retried, because the next round is
// the retry.
func TestKeepAliveRoundUsesTheFirstAnsweringTarget(t *testing.T) {
	var firstRequests atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		firstRequests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer first.Close()

	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer second.Close()

	results := make([]KeepAliveResult, 0, 2)
	target, ok := keepAliveRound(context.Background(), keepAliveClientFor(fakeDialer{}), fakeDialer{}, []KeepAliveTarget{
		{URL: first.URL, Named: true},
		{URL: second.URL, Named: true},
	}, "", func(result KeepAliveResult) {
		results = append(results, result)
	})
	if !ok || target != second.URL {
		t.Fatalf("round = (%q, %t), want (%q, true)", target, ok, second.URL)
	}
	if got := firstRequests.Load(); got != 1 {
		t.Fatalf("first target requests = %d, want 1 (no within-round retry)", got)
	}

	// Every attempt is reported, not only the one that won, so a status line
	// can say why it moved to the next target.
	if len(results) != 2 {
		t.Fatalf("results = %+v, want one per attempt", results)
	}
	if results[0].Target != first.URL || results[0].OK || results[0].StatusCode != http.StatusInternalServerError {
		t.Fatalf("first result = %+v, want a reported 500", results[0])
	}
	if results[1].Target != second.URL || !results[1].OK || results[1].StatusCode != http.StatusOK {
		t.Fatalf("second result = %+v, want a reported 200", results[1])
	}
	if results[0].At.IsZero() || results[1].At.Before(results[0].At) {
		t.Fatalf("result timestamps = (%v, %v), want increasing", results[0].At, results[1].At)
	}
}

func TestKeepAliveRoundReportsWhenNoTargetAnswers(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var results []KeepAliveResult
	target, ok := keepAliveRound(context.Background(), keepAliveClientFor(fakeDialer{}), fakeDialer{}, []KeepAliveTarget{
		{URL: server.URL, Named: true},
	}, "", func(result KeepAliveResult) {
		results = append(results, result)
	})
	if ok || target != "" {
		t.Fatalf("round = (%q, %t), want no target", target, ok)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
	if len(results) != 1 || results[0].OK || results[0].StatusCode != http.StatusNotFound {
		t.Fatalf("results = %+v, want one reported 404", results)
	}
}

// TestKeepAliveOpensAFreshConnectionPerProbe pins the deliberate choice of one
// connection per probe: an idle tunnel connection between probes buys nothing,
// hides routing changes and is dropped by the userspace netstack anyway.
func TestKeepAliveOpensAFreshConnectionPerProbe(t *testing.T) {
	type connIDKey struct{}

	var nextID atomic.Int64
	ids := make(chan int64, 4)

	server := httptest.NewUnstartedServer(nil)
	server.Config.ConnContext = func(ctx context.Context, _ net.Conn) context.Context {
		return context.WithValue(ctx, connIDKey{}, nextID.Add(1))
	}
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := r.Context().Value(connIDKey{}).(int64); ok {
			select {
			case ids <- id:
			default:
			}
		}
		_, _ = w.Write([]byte("ok"))
	})
	server.Start()
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		KeepAlive(ctx, nil, fakeDialer{}, KeepAliveOptions{
			Targets:  []KeepAliveTarget{{URL: server.URL, Named: true}},
			Interval: 20 * time.Millisecond,
		})
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	seen := make([]int64, 0, 2)
	for len(seen) < 2 {
		select {
		case id := <-ids:
			seen = append(seen, id)
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d keepalive requests arrived", len(seen))
		}
	}
	if seen[0] == seen[1] {
		t.Fatalf("both probes arrived on connection %d; each probe must open its own", seen[0])
	}
}

// TestKeepAliveFallsBackToRemoteDNSWithALabel: a name lookup is not application
// traffic, and the fallback line has to say so.
func TestKeepAliveFallsBackToRemoteDNSWithALabel(t *testing.T) {
	oldOutput := stdlog.Writer()
	var output bytes.Buffer
	stdlog.SetOutput(&output)
	t.Cleanup(func() { stdlog.SetOutput(oldOutput) })

	// A zero resolver has no remote resolvers, so the fallback logs and returns.
	var results []KeepAliveResult
	KeepAlive(context.Background(), &resolve.Resolver{}, fakeDialer{}, KeepAliveOptions{
		OnResult: func(result KeepAliveResult) { results = append(results, result) },
	})

	got := output.String()
	if !strings.Contains(got, "not application traffic") {
		t.Fatalf("fallback log does not label the probe: %q", got)
	}
	if !strings.Contains(got, "No remote resolver available") {
		t.Fatalf("fallback log does not explain the missing resolver: %q", got)
	}
	if len(results) != 1 || results[0].OK || !results[0].Fallback || results[0].Err == nil {
		t.Fatalf("fallback results = %+v, want one labelled failure", results)
	}
}

func TestKeepAliveOrderPutsTheWorkingTargetFirst(t *testing.T) {
	targets := []KeepAliveTarget{
		{URL: "http://a.example/", Named: true},
		{URL: "http://b.example/", Named: true},
		{URL: "http://c.example/"},
		{URL: "http://d.example/"},
	}

	ordered := keepAliveOrder(targets, "http://c.example/")
	want := []string{"http://c.example/", "http://a.example/", "http://b.example/", "http://d.example/"}
	for i, target := range ordered {
		if target.URL != want[i] {
			t.Fatalf("order = %v, want %v", urls(ordered), want)
		}
	}
}

func urls(targets []KeepAliveTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.String())
	}
	return out
}

// acceptLoop serves one connection at a time until the listener is closed.
func acceptLoop(listener net.Listener, handle func(net.Conn)) {
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handle(conn)
		}
	}()
}

// TestTCPKeepAliveAttempt: the probe counts once the tunnel carries a
// connection to the granted address. The exchange the port implies is best
// effort, because a service that does not speak it must not look like a dead
// session.
func TestTCPKeepAliveAttempt(t *testing.T) {
	t.Run("a non-TLS server on 443 still counts as connected", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = listener.Close() }()
		acceptLoop(listener, func(conn net.Conn) { _ = conn.Close() })

		var results []KeepAliveResult
		dialer := fakeDialer{dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, listener.Addr().String())
		}}
		if !tcpKeepAliveAttempt(context.Background(), dialer, KeepAliveTarget{Address: "127.0.0.1:443"}, time.Second, func(result KeepAliveResult) {
			results = append(results, result)
		}) {
			t.Fatalf("probe = false, want the established connection to count (results: %+v)", results)
		}
		if len(results) != 1 || !results[0].OK || results[0].Target != "tcp://127.0.0.1:443" || results[0].Err != nil {
			t.Fatalf("results = %+v, want one OK connection to the granted address", results)
		}
	})

	t.Run("the TLS handshake on 443 completes", func(t *testing.T) {
		server := httptest.NewTLSServer(http.NotFoundHandler())
		defer server.Close()

		dialer := fakeDialer{dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}}
		var results []KeepAliveResult
		if !tcpKeepAliveAttempt(context.Background(), dialer, KeepAliveTarget{Address: "127.0.0.1:443"}, time.Second, func(result KeepAliveResult) {
			results = append(results, result)
		}) {
			t.Fatalf("probe = false, want a completed handshake")
		}
		if len(results) != 1 || !results[0].OK {
			t.Fatalf("results = %+v, want one OK probe", results)
		}
	})

	t.Run("the SSH identification on 22 is read", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = listener.Close() }()
		acceptLoop(listener, func(conn net.Conn) {
			defer func() { _ = conn.Close() }()
			_, _ = conn.Write([]byte("SSH-2.0-OpenSSH_test\r\n"))
		})

		var results []KeepAliveResult
		dialer := fakeDialer{dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, listener.Addr().String())
		}}
		if !tcpKeepAliveAttempt(context.Background(), dialer, KeepAliveTarget{Address: "127.0.0.1:22"}, time.Second, func(result KeepAliveResult) {
			results = append(results, result)
		}) {
			t.Fatalf("probe = false, want the SSH banner read")
		}
		if len(results) != 1 || !results[0].OK {
			t.Fatalf("results = %+v, want one OK probe", results)
		}
	})

	t.Run("a failed connection is reported", func(t *testing.T) {
		dialer := fakeDialer{dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("tcp tunnel authentication failed: invalid SID")
		}}
		var results []KeepAliveResult
		if tcpKeepAliveAttempt(context.Background(), dialer, KeepAliveTarget{Address: "127.0.0.1:22"}, time.Second, func(result KeepAliveResult) {
			results = append(results, result)
		}) {
			t.Fatal("probe = true, want the dial error")
		}
		if len(results) != 1 || results[0].OK || results[0].Err == nil {
			t.Fatalf("results = %+v, want one reported failure", results)
		}
	})
}
