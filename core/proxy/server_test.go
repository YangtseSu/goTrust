// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package proxy

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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

// A client may pipeline the CONNECT request and its first payload in one write.
// Those bytes are already buffered when the connection is hijacked; copying
// them with io.Copy(target, buffered) ran the reader's WriteTo, which reads the
// client until EOF and never lets the target's answer through.
func TestHTTPConnectForwardsPipelinedBytes(t *testing.T) {
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
	// A stall must fail the test instead of hanging it.
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	payload := "pipelined-payload"
	request := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n%s", echo.Addr(), echo.Addr(), payload)
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatalf("write CONNECT and payload: %v", err)
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

	echoed := make([]byte, len(payload))
	if _, err := io.ReadFull(reader, echoed); err != nil {
		t.Fatalf("read the pipelined echo: %v", err)
	}
	if string(echoed) != payload {
		t.Fatalf("echoed %q, want %q", echoed, payload)
	}
}

// startProxy binds both configured listeners on an ephemeral-free port chosen
// by the test and closes the server when the test ends.
func startProxy(t *testing.T, options Options) *Server {
	t.Helper()

	server, err := New(options)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	return server
}

// readProxyStatus reads the status line and the headers of a proxy reply and
// returns the status line.
func readProxyStatus(t *testing.T, reader *bufio.Reader) string {
	t.Helper()

	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status line: %v", err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read headers: %v", err)
		}
		if line == "\r\n" {
			return status
		}
	}
}

// socks5Connect performs the greeting and CONNECT handshake and returns the
// tunnelled connection.
func socks5Connect(t *testing.T, bind, target string) net.Conn {
	t.Helper()

	conn, err := net.Dial("tcp", bind)
	if err != nil {
		t.Fatalf("dial the proxy: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write greeting: %v", err)
	}
	greeting := make([]byte, 2)
	if _, err := io.ReadFull(conn, greeting); err != nil {
		t.Fatalf("read greeting: %v", err)
	}

	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatalf("split target address: %v", err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatalf("parse target port: %v", err)
	}
	ip := net.ParseIP(host).To4()
	if ip == nil {
		t.Fatalf("target address %s is not IPv4", host)
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
	return conn
}

// isTimeout reports whether err is a read-deadline expiry. A closed connection
// fails at once; a live one waits for the deadline.
func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// Close must drop a hijacked CONNECT tunnel: http.Server.Shutdown ignores
// hijacked connections, so an open tunnel used to keep the session's data path
// alive after the proxy was gone.
func TestCloseDropsHTTPConnectTunnels(t *testing.T) {
	echo := echoServer(t)
	server := startProxy(t, Options{HTTPBind: freePort(t), Dial: directDial})
	bind := server.Addresses()[0]

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
	if status := readProxyStatus(t, reader); !strings.Contains(status, "200") {
		t.Fatalf("status = %q, want 200", status)
	}
	if _, err := conn.Write([]byte("still open")); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	echoed := make([]byte, len("still open"))
	if _, err := io.ReadFull(reader, echoed); err != nil {
		t.Fatalf("read echo before close: %v", err)
	}

	if err := server.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("the hijacked tunnel still delivered a byte after Close")
	} else if isTimeout(err) {
		t.Fatal("the hijacked tunnel survived Close until the read deadline")
	}
}

// A SOCKS5 tunnel is accepted by the library, not hijacked from net/http, so it
// needs its own tracking to be dropped by Close.
func TestCloseDropsSOCKS5Tunnels(t *testing.T) {
	echo := echoServer(t)
	server := startProxy(t, Options{SOCKS5Bind: freePort(t), Dial: directDial})
	conn := socks5Connect(t, server.Addresses()[0], echo.Addr().String())

	if _, err := conn.Write([]byte("still open")); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	echoed := make([]byte, len("still open"))
	if _, err := io.ReadFull(conn, echoed); err != nil {
		t.Fatalf("read echo before close: %v", err)
	}

	if err := server.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("the SOCKS5 tunnel still delivered a byte after Close")
	} else if isTimeout(err) {
		t.Fatal("the SOCKS5 tunnel survived Close until the read deadline")
	}
}

// Hop-by-hop headers describe one connection, so the proxy must not hand the
// client's proxy credentials or the names a Connection header lists to the
// origin.
func TestHTTPForwardStripsHopByHopRequestHeaders(t *testing.T) {
	seen := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		fmt.Fprint(w, "ok")
	}))
	defer upstream.Close()

	bind := freePort(t)
	startProxy(t, Options{HTTPBind: bind, Dial: directDial})

	conn, err := net.Dial("tcp", bind)
	if err != nil {
		t.Fatalf("dial the proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}
	request := fmt.Sprintf("GET %s/echo HTTP/1.1\r\nHost: %s\r\n"+
		"Proxy-Authorization: Basic c2VjcmV0\r\n"+
		"Connection: X-Hop\r\n"+
		"X-Hop: drop-me\r\n"+
		"Keep-Alive: timeout=5\r\n"+
		"X-Kept: yes\r\n\r\n", upstream.URL, target.Host)
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatalf("write request: %v", err)
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer response.Body.Close()
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}

	forwarded := <-seen
	for _, name := range []string{"Proxy-Authorization", "X-Hop", "Keep-Alive", "Connection"} {
		if value := forwarded.Get(name); value != "" {
			t.Errorf("origin saw %s: %q, want it stripped", name, value)
		}
	}
	if forwarded.Get("X-Kept") != "yes" {
		t.Errorf("origin saw X-Kept = %q, want yes", forwarded.Get("X-Kept"))
	}
}

// A streamed response must reach the client while the origin still holds it
// open; buffering until the origin closes turns a live stream into a dead one.
func TestHTTPForwardStreamsWithoutWaitingForTheOrigin(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "first\n")
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "second\n")
	}))
	// The upstream handler waits for the release, so release it before the
	// server's own Close waits for the handler.
	defer upstream.Close()
	defer unblock()

	server := startProxy(t, Options{HTTPBind: freePort(t), Dial: directDial})
	conn, err := net.Dial("tcp", server.Addresses()[0])
	if err != nil {
		t.Fatalf("dial the proxy: %v", err)
	}
	defer conn.Close()

	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}
	request := fmt.Sprintf("GET %s/stream HTTP/1.1\r\nHost: %s\r\n\r\n", upstream.URL, target.Host)
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatalf("write request: %v", err)
	}

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	first := make([]byte, len("first\n"))
	if _, err := io.ReadFull(response.Body, first); err != nil {
		t.Fatalf("read the first event while the origin is open: %v", err)
	}
	if string(first) != "first\n" {
		t.Fatalf("first event = %q, want %q", first, "first\n")
	}

	unblock()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	second := make([]byte, len("second\n"))
	if _, err := io.ReadFull(response.Body, second); err != nil {
		t.Fatalf("read the second event: %v", err)
	}
	if string(second) != "second\n" {
		t.Fatalf("second event = %q, want %q", second, "second\n")
	}
}

// A client that half-closes after its request must still receive the reply.
// The tunnel used to return as soon as the client→target copy ended, closing
// the connection before the origin could answer.
func TestHTTPConnectHalfCloseKeepsTheResponseDirection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		request, _ := io.ReadAll(conn)
		_, _ = conn.Write(append([]byte("answered:"), request...))
	}()

	server := startProxy(t, Options{HTTPBind: freePort(t), Dial: directDial})
	conn, err := net.Dial("tcp", server.Addresses()[0])
	if err != nil {
		t.Fatalf("dial the proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	request := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", listener.Addr(), listener.Addr())
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	reader := bufio.NewReader(conn)
	if status := readProxyStatus(t, reader); !strings.Contains(status, "200") {
		t.Fatalf("status = %q, want 200", status)
	}

	if _, err := io.WriteString(conn, "question"); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		t.Fatalf("connection is %T, want *net.TCPConn", conn)
	}
	if err := tcpConn.CloseWrite(); err != nil {
		t.Fatalf("half-close: %v", err)
	}

	answer := make([]byte, len("answered:question"))
	if _, err := io.ReadFull(reader, answer); err != nil {
		t.Fatalf("read the answer after half-close: %v", err)
	}
	if string(answer) != "answered:question" {
		t.Fatalf("answer = %q, want %q", answer, "answered:question")
	}
}

// One Transport per server keeps the upstream connection alive between
// requests; a per-request Transport dialled a new one every time.
func TestHTTPForwardReusesTheUpstreamConnection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer upstream.Close()

	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	var mu sync.Mutex
	dials := 0
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == target.Host {
			mu.Lock()
			dials++
			mu.Unlock()
		}
		return directDial(ctx, network, addr)
	}

	bind := freePort(t)
	startProxy(t, Options{HTTPBind: bind, Dial: dial})
	proxyURL, err := url.Parse("http://" + bind)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}

	for i := 0; i < 2; i++ {
		response, err := client.Get(upstream.URL + "/reuse")
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			t.Fatalf("request %d body: %v", i+1, err)
		}
		response.Body.Close()
	}

	mu.Lock()
	defer mu.Unlock()
	if dials != 1 {
		t.Fatalf("upstream dials = %d, want 1 (the connection was not reused)", dials)
	}
}
