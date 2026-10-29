// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package proxy serves SOCKS5 and HTTP proxies over an injected dial function.
//
// It deliberately does not use the dialer's own listener helpers: those take no
// context and register their cleanup inside the upstream module's internal
// packages, which this module cannot reach. Owning the listeners here also means
// a failed bind is reported instead of panicking.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/things-go/go-socks5"
)

// DialFunc opens a connection through the tunnel. The network is "tcp" or
// "udp"; addr is "host:port" and may carry a hostname, which the dialer is
// expected to resolve inside the tunnel.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// NameResolver maps a hostname to an address. A SOCKS5 client sends the
// hostname, so without one the library resolves it with the system resolver:
// an intranet name would never resolve, and a name that resolves publicly would
// connect outside the tunnel. The tunnel resolver satisfies this interface.
type NameResolver interface {
	Resolve(ctx context.Context, name string) (context.Context, net.IP, error)
}

// Options configures the two listeners. An empty bind disables that listener.
type Options struct {
	SOCKS5Bind string
	HTTPBind   string
	// Dial is required.
	Dial DialFunc
	// Resolver resolves hostnames inside the tunnel. Nil falls back to the
	// system resolver, which is only correct for a public destination.
	Resolver NameResolver
	// Logf receives listener and request diagnostics. Nil discards them.
	Logf func(format string, args ...any)
}

// Server owns the listeners for one session. Create one per connection and
// call Close when the session ends.
type Server struct {
	options Options
	logf    func(format string, args ...any)
	// transport serves plain-HTTP proxy requests. One per server, because a
	// per-request Transport never reuses a connection and leaks its idle pool.
	transport *http.Transport

	mu        sync.Mutex
	listeners []boundListener
	servers   []*http.Server
	// conns holds every proxied connection that outlives its handler: hijacked
	// CONNECT tunnels and accepted SOCKS5 connections. http.Server.Shutdown
	// ignores hijacked connections, so Close has to drop them itself.
	conns  map[net.Conn]struct{}
	closed bool

	// up and down count the bytes the proxy moved since it started. They are
	// written on every tunnel read and write, so they are atomics rather than
	// fields under mu: the data path must not contend for the lock Close and
	// the listener list use.
	up, down atomic.Uint64
}

// ErrNoListener reports that both binds were empty.
var ErrNoListener = errors.New("no proxy listener is configured")

// New builds the server. It does not bind anything yet.
func New(options Options) (*Server, error) {
	if options.Dial == nil {
		return nil, errors.New("proxy: Dial is required")
	}
	if options.SOCKS5Bind == "" && options.HTTPBind == "" {
		return nil, ErrNoListener
	}
	logf := options.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Server{
		options: options,
		logf:    logf,
		transport: &http.Transport{
			DialContext:           options.Dial,
			MaxIdleConns:          16,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}, nil
}

// Start binds every configured listener and serves in the background. A bind
// failure closes whatever was already bound, so a half-started server never
// outlives its error.
func (s *Server) Start(ctx context.Context) error {
	if s.options.SOCKS5Bind != "" {
		if err := s.startSOCKS5(ctx, s.options.SOCKS5Bind); err != nil {
			_ = s.Close()
			return err
		}
	}
	if s.options.HTTPBind != "" {
		if err := s.startHTTP(ctx, s.options.HTTPBind); err != nil {
			_ = s.Close()
			return err
		}
	}
	return nil
}

// Listener is one bound proxy endpoint: the protocol it speaks and the address
// it listens on, as the window shows them.
type Listener struct {
	Protocol string
	Address  string
}

// The protocols a Listener speaks. They are names, not interface text, so they
// read the same in every language.
const (
	ProtocolSOCKS5 = "SOCKS5"
	ProtocolHTTP   = "HTTP"
)

// boundListener is one listening socket and the protocol it speaks.
type boundListener struct {
	protocol string
	net.Listener
}

// Listeners reports the bound endpoints, in the order they were bound.
func (s *Server) Listeners() []Listener {
	s.mu.Lock()
	defer s.mu.Unlock()

	listeners := make([]Listener, 0, len(s.listeners))
	for _, listener := range s.listeners {
		listeners = append(listeners, Listener{
			Protocol: listener.protocol,
			Address:  listener.Addr().String(),
		})
	}
	return listeners
}

// Traffic is the number of bytes the proxy has moved in each direction since
// it started: Up is what clients sent into the tunnel, Down what came back
// through it.
type Traffic struct {
	Up   uint64
	Down uint64
}

// Traffic reports the byte counters. A closed server reports the totals it
// ended with, because the listeners stop and nothing more is counted.
func (s *Server) Traffic() Traffic {
	return Traffic{Up: s.up.Load(), Down: s.down.Load()}
}

func (s *Server) countUp(n int) {
	if n > 0 {
		s.up.Add(uint64(n))
	}
}

func (s *Server) countDown(n int) {
	if n > 0 {
		s.down.Add(uint64(n))
	}
}

// Close stops every listener and drops every open tunnel. It is safe to call
// twice.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	listeners := s.listeners
	servers := s.servers
	conns := make([]net.Conn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.listeners = nil
	s.servers = nil
	s.conns = nil
	s.mu.Unlock()

	var failures []error
	for _, listener := range listeners {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			failures = append(failures, err)
		}
	}
	// A hijacked tunnel is invisible to Shutdown; closing it here is what
	// releases the session's data path when the proxy goes away.
	for _, conn := range conns {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			failures = append(failures, err)
		}
	}
	s.transport.CloseIdleConnections()
	for _, server := range servers {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
		cancel()
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

// track registers a proxied connection. It reports false once the server is
// closed, so a connection that surfaces after Close is dropped instead of
// outliving its session.
func (s *Server) track(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	if s.conns == nil {
		s.conns = make(map[net.Conn]struct{})
	}
	s.conns[conn] = struct{}{}
	return true
}

func (s *Server) untrack(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

// trackedConn untracks itself when its handler closes it.
type trackedConn struct {
	net.Conn
	server *Server
	once   sync.Once
}

func (c *trackedConn) Close() error {
	var err error
	c.once.Do(func() {
		c.server.untrack(c.Conn)
		err = c.Conn.Close()
	})
	return err
}

// Read counts what the client sends into the tunnel: the SOCKS5 handler and
// the CONNECT tunnel both read the client connection to forward it.
func (c *trackedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.server.countUp(n)
	return n, err
}

// Write counts what the tunnel sends back to the client.
func (c *trackedConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.server.countDown(n)
	return n, err
}

// CloseWrite keeps the half-close usable through the wrapper: the SOCKS5
// server asserts it when it ends a direction.
func (c *trackedConn) CloseWrite() error {
	if closer, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return closer.CloseWrite()
	}
	return nil
}

// trackedListener registers every accepted connection with the server, so a
// SOCKS5 tunnel is dropped by Close like a hijacked CONNECT.
type trackedListener struct {
	net.Listener
	server *Server
}

func (l *trackedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if !l.server.track(conn) {
		_ = conn.Close()
		return nil, net.ErrClosed
	}
	return &trackedConn{Conn: conn, server: l.server}, nil
}

func (s *Server) startSOCKS5(ctx context.Context, bind string) error {
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return fmt.Errorf("listen for SOCKS5 on %s: %w", bind, err)
	}

	options := []socks5.Option{
		socks5.WithAuthMethods([]socks5.Authenticator{socks5.NoAuthAuthenticator{}}),
		socks5.WithDial(s.options.Dial),
	}
	if s.options.Resolver != nil {
		options = append(options, socks5.WithResolver(s.options.Resolver))
	}
	server := socks5.NewServer(options...)
	tracked := &trackedListener{Listener: listener, server: s}

	s.mu.Lock()
	s.listeners = append(s.listeners, boundListener{protocol: ProtocolSOCKS5, Listener: listener})
	s.mu.Unlock()

	s.logf("SOCKS5 listening on %s", listener.Addr())
	go func() {
		err := server.Serve(tracked)
		if err != nil && !errors.Is(err, net.ErrClosed) {
			s.logf("SOCKS5 listener stopped: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	return nil
}

func (s *Server) startHTTP(ctx context.Context, bind string) error {
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return fmt.Errorf("listen for HTTP proxy on %s: %w", bind, err)
	}

	server := &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	s.mu.Lock()
	s.listeners = append(s.listeners, boundListener{protocol: ProtocolHTTP, Listener: listener})
	s.servers = append(s.servers, server)
	s.mu.Unlock()

	s.logf("HTTP proxy listening on %s", listener.Addr())
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logf("HTTP proxy stopped: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	return nil
}

// handler accepts both proxy forms: CONNECT tunnels for HTTPS, and absolute
// URI requests for plain HTTP. Anything else is a direct hit on the proxy port,
// which is a mistake worth answering with an explanation.
func (s *Server) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			s.tunnel(w, r)
			return
		}
		if r.URL.Host == "" {
			http.Error(w, "goTrust is a forward proxy: point your application at it instead of opening this page", http.StatusBadRequest)
			return
		}
		s.forward(w, r)
	})
}

// tunnel serves CONNECT: the client keeps its TLS session end to end and this
// side only moves bytes.
func (s *Server) tunnel(w http.ResponseWriter, r *http.Request) {
	target, err := s.options.Dial(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = target.Close()
		http.Error(w, "hijacking is unavailable", http.StatusInternalServerError)
		return
	}
	rawClient, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = target.Close()
		return
	}
	if !s.track(rawClient) {
		_ = rawClient.Close()
		_ = target.Close()
		return
	}
	client := &trackedConn{Conn: rawClient, server: s}
	defer client.Close()
	defer target.Close()

	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	// The hijacked connection may already hold bytes the client sent; they
	// belong to the tunnel and must be forwarded before the raw copy. Copying
	// from the buffered readwriter would run its WriteTo, which reads the
	// client until EOF and blocks the other direction: a CONNECT with a
	// pipelined first payload then hung. Drain the buffer instead.
	if n := buffered.Reader.Buffered(); n > 0 {
		pending := make([]byte, n)
		if _, err := io.ReadFull(buffered.Reader, pending); err != nil {
			return
		}
		// These bytes left the client before the hijack; the tunnel's own
		// counter never saw them.
		s.countUp(len(pending))
		if _, err := target.Write(pending); err != nil {
			return
		}
	}

	// Each direction ends with a half-close, and the handler waits for both.
	// Returning after the first one truncated whatever the other direction
	// still had to deliver, which broke a client that half-closes after its
	// request and then reads the response.
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(target, client); closeWrite(target); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, target); closeWrite(client); done <- struct{}{} }()
	<-done
	<-done
}

// closeWrite half-closes a connection that supports it, so its peer sees EOF
// while the opposite direction keeps draining.
func closeWrite(conn net.Conn) {
	if closer, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = closer.CloseWrite()
	}
}

// forward serves a plain-HTTP proxy request: absolute URI in, same URI out over
// the tunnel.
func (s *Server) forward(w http.ResponseWriter, r *http.Request) {
	outbound := r.Clone(r.Context())
	outbound.RequestURI = ""
	stripHopByHop(outbound.Header)
	if outbound.Body != nil {
		outbound.Body = countingReader{reader: outbound.Body, count: s.countUp}
	}

	response, err := s.transport.RoundTrip(outbound)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()

	// Hop-by-hop headers are meaningless across a proxy and must not be copied.
	for name, values := range response.Header {
		if isHopByHop(name) {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	stream(countingWriter{ResponseWriter: w, count: s.countDown}, response.Body)
}

// countingReader counts the request body a plain-HTTP exchange sends toward
// the origin. Close follows the transport's own close of the request body.
type countingReader struct {
	reader io.Reader
	count  func(int)
}

func (r countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.count(n)
	return n, err
}

func (r countingReader) Close() error {
	if closer, ok := r.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// countingWriter counts the response body a plain-HTTP exchange sends back to
// the client. Unwrap keeps the response's own Flush reachable through it.
type countingWriter struct {
	http.ResponseWriter
	count func(int)
}

func (w countingWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.count(n)
	return n, err
}

// Unwrap exposes the wrapped writer to http.ResponseController, which is how
// stream's Flush still reaches the connection through the wrapper. Embedding
// the interface does not promote Flush: it is not one of its methods.
func (w countingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// stream copies a response body and flushes every chunk. Without a flush a
// streamed response (event stream, chunked download, long poll) sits in the
// proxy's buffer until the origin closes, so a live stream looks dead.
func stream(w http.ResponseWriter, body io.Reader) {
	controller := http.NewResponseController(w)
	buffer := make([]byte, 32*1024)
	for {
		n, err := body.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				return
			}
			_ = controller.Flush()
		}
		if err != nil {
			return
		}
	}
}

// stripHopByHop removes the headers that describe one hop only, including the
// names a Connection header lists. A proxied request must not carry the
// client's proxy credentials or connection token to the origin.
func stripHopByHop(header http.Header) {
	for _, value := range header.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			if name := strings.TrimSpace(token); name != "" {
				header.Del(name)
			}
		}
	}
	for name := range header {
		if isHopByHop(name) {
			header.Del(name)
		}
	}
}

func isHopByHop(name string) bool {
	switch strings.ToLower(name) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate",
		"proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	}
	return false
}
