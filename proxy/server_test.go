package proxy

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// echoServer is the stand-in for an intranet service: whatever a client writes
// comes back with a fixed prefix.
func echoServer(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener
}

// directDial dials without a tunnel, which is what the fake dialer must do to
// reach the local echo server.
func directDial(ctx context.Context, network, addr string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, addr)
}

func freePort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe for a free port: %v", err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func TestSOCKS5TunnelsBytes(t *testing.T) {
	echo := echoServer(t)
	bind := freePort(t)

	server, err := New(Options{SOCKS5Bind: bind, Dial: directDial})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = server.Close() }()

	conn, err := net.Dial("tcp", bind)
	if err != nil {
		t.Fatalf("dial the proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Greeting: one method, "no authentication".
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write greeting: %v", err)
	}
	greeting := make([]byte, 2)
	if _, err := io.ReadFull(conn, greeting); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if greeting[0] != 0x05 || greeting[1] != 0x00 {
		t.Fatalf("greeting = %v, want [5 0]", greeting)
	}

	// CONNECT to the echo server by address.
	host, portText, err := net.SplitHostPort(echo.Addr().String())
	if err != nil {
		t.Fatalf("split echo address: %v", err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatalf("parse echo port: %v", err)
	}
	ip := net.ParseIP(host).To4()
	if ip == nil {
		t.Fatalf("echo address %s is not IPv4", host)
	}

	request := []byte{0x05, 0x01, 0x00, 0x01}
	request = append(request, ip...)
	request = binary.BigEndian.AppendUint16(request, uint16(port))
	if _, err := conn.Write(request); err != nil {
		t.Fatalf("write connect: %v", err)
	}

	reply := make([]byte, 10)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read connect reply: %v", err)
	}
	if reply[1] != 0x00 {
		t.Fatalf("connect reply code = %#x, want 0", reply[1])
	}

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	response := make([]byte, 5)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if string(response) != "hello" {
		t.Fatalf("echoed %q, want hello", response)
	}
}

func TestHTTPConnectTunnelsBytes(t *testing.T) {
	echo := echoServer(t)
	bind := freePort(t)

	server, err := New(Options{HTTPBind: bind, Dial: directDial})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = server.Close() }()

	conn, err := net.Dial("tcp", bind)
	if err != nil {
		t.Fatalf("dial the proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	request := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", echo.Addr(), echo.Addr())
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status line: %v", err)
	}
	if !strings.Contains(status, "200") {
		t.Fatalf("status = %q, want 200", status)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read headers: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	if _, err := conn.Write([]byte("tunnelled")); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	echoed := make([]byte, len("tunnelled"))
	if _, err := io.ReadFull(reader, echoed); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if string(echoed) != "tunnelled" {
		t.Fatalf("echoed %q, want tunnelled", echoed)
	}
}

func TestHTTPForwardProxiesPlainRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("upstream saw path %q, want /status", r.URL.Path)
		}
		fmt.Fprint(w, "intranet ok")
	}))
	defer upstream.Close()

	bind := freePort(t)
	server, err := New(Options{HTTPBind: bind, Dial: directDial})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = server.Close() }()

	proxyURL, err := url.Parse("http://" + bind)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}

	response, err := client.Get(upstream.URL + "/status")
	if err != nil {
		t.Fatalf("get through the proxy: %v", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "intranet ok" {
		t.Fatalf("body = %q, want intranet ok", body)
	}
}

// A SOCKS5 client sends the hostname, so the resolver must be the tunnel's, not
// the system's: otherwise an intranet name never resolves and a public name
// leaves the tunnel. This test proves the injected resolver is the one asked.
func TestSOCKS5ResolvesHostnameThroughTheInjectedResolver(t *testing.T) {
	echo := echoServer(t)
	bind := freePort(t)

	_, portText, err := net.SplitHostPort(echo.Addr().String())
	if err != nil {
		t.Fatalf("split echo address: %v", err)
	}

	var asked []string
	resolver := resolverFunc(func(ctx context.Context, name string) (context.Context, net.IP, error) {
		asked = append(asked, name)
		return ctx, net.ParseIP("127.0.0.1"), nil
	})

	server, err := New(Options{SOCKS5Bind: bind, Dial: directDial, Resolver: resolver})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = server.Close() }()

	conn, err := net.Dial("tcp", bind)
	if err != nil {
		t.Fatalf("dial the proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write greeting: %v", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read greeting: %v", err)
	}

	// CONNECT with a domain name, which is what a browser sends.
	host := "intranet.example"
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	request := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	request = append(request, host...)
	request = binary.BigEndian.AppendUint16(request, uint16(port))
	if _, err := conn.Write(request); err != nil {
		t.Fatalf("write connect: %v", err)
	}

	connectReply := make([]byte, 10)
	if _, err := io.ReadFull(conn, connectReply); err != nil {
		t.Fatalf("read connect reply: %v", err)
	}
	if connectReply[1] != 0x00 {
		t.Fatalf("connect reply code = %#x, want 0", connectReply[1])
	}
	if len(asked) != 1 || asked[0] != host {
		t.Fatalf("resolver saw %v, want [%s]", asked, host)
	}
}

type resolverFunc func(ctx context.Context, name string) (context.Context, net.IP, error)

func (f resolverFunc) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	return f(ctx, name)
}

func TestDirectHitExplainsItself(t *testing.T) {
	bind := freePort(t)
	server, err := New(Options{HTTPBind: bind, Dial: directDial})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = server.Close() }()

	response, err := http.Get("http://" + bind + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func TestCloseStopsTheListener(t *testing.T) {
	bind := freePort(t)
	server, err := New(Options{SOCKS5Bind: bind, HTTPBind: "", Dial: directDial})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if addresses := server.Addresses(); len(addresses) != 1 {
		t.Fatalf("addresses = %v, want one", addresses)
	}

	if err := server.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if addresses := server.Addresses(); len(addresses) != 0 {
		t.Fatalf("addresses after close = %v, want none", addresses)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}

	conn, err := net.DialTimeout("tcp", bind, time.Second)
	if err == nil {
		_ = conn.Close()
		t.Fatal("dial succeeded after close")
	}
}

func TestBothBindsEmptyIsRejected(t *testing.T) {
	if _, err := New(Options{Dial: directDial}); err != ErrNoListener {
		t.Fatalf("err = %v, want ErrNoListener", err)
	}
}

func TestBindFailureClosesWhatWasBound(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy a port: %v", err)
	}
	defer taken.Close()

	// SOCKS5 binds first, then the HTTP bind collides with the occupied port.
	server, err := New(Options{
		SOCKS5Bind: freePort(t),
		HTTPBind:   taken.Addr().String(),
		Dial:       directDial,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	err = server.Start(context.Background())
	if err == nil {
		t.Fatal("start succeeded on an occupied port")
	}
	if addresses := server.Addresses(); len(addresses) != 0 {
		t.Fatalf("addresses = %v, want none after a failed start", addresses)
	}
}

// Both listeners must stop when the session context ends. A login attempt's
// deadline is not that context: the app passes one that lives with the session.
func TestContextCancellationClosesBothListeners(t *testing.T) {
	socks5Bind := freePort(t)
	httpBind := freePort(t)

	server, err := New(Options{SOCKS5Bind: socks5Bind, HTTPBind: httpBind, Dial: directDial})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer func() { _ = server.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	for _, address := range []string{socks5Bind, httpBind} {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			t.Fatalf("dial %s before cancel: %v", address, err)
		}
		_ = conn.Close()
	}

	cancel()

	for _, address := range []string{socks5Bind, httpBind} {
		deadline := time.Now().Add(2 * time.Second)
		for {
			conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if err != nil {
				break
			}
			_ = conn.Close()
			if time.Now().After(deadline) {
				t.Fatalf("%s still accepts connections after the context ended", address)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
