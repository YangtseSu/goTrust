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

	mu        sync.Mutex
	listeners []net.Listener
	servers   []*http.Server
	socks     *socks5.Server
	closed    bool
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
	return &Server{options: options, logf: logf}, nil
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

// Addresses reports the bound listeners, in the form the UI shows the user.
func (s *Server) Addresses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	addresses := make([]string, 0, len(s.listeners))
	for _, listener := range s.listeners {
		addresses = append(addresses, listener.Addr().String())
	}
	return addresses
}

// Close stops every listener. It is safe to call twice.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	listeners := s.listeners
	servers := s.servers
	s.listeners = nil
	s.servers = nil
	s.mu.Unlock()

	var failures []error
	for _, server := range servers {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
		cancel()
	}
	for _, listener := range listeners {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
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

	s.mu.Lock()
	s.listeners = append(s.listeners, listener)
	s.socks = server
	s.mu.Unlock()

	s.logf("SOCKS5 listening on %s", listener.Addr())
	go func() {
		err := server.Serve(listener)
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
	s.listeners = append(s.listeners, listener)
	s.servers = append(s.servers, server)
	s.mu.Unlock()

	s.logf("HTTP proxy listening on %s", listener.Addr())
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logf("HTTP proxy stopped: %v", err)
		}
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
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = target.Close()
		return
	}
	defer client.Close()
	defer target.Close()

	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	// The hijacked connection may already hold bytes the client sent; they
	// belong to the tunnel and must be forwarded before the raw copy.
	if buffered.Reader.Buffered() > 0 {
		if _, err := io.Copy(target, buffered); err != nil {
			return
		}
	}

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(target, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, target); done <- struct{}{} }()
	<-done
}

// forward serves a plain-HTTP proxy request: absolute URI in, same URI out over
// the tunnel.
func (s *Server) forward(w http.ResponseWriter, r *http.Request) {
	outbound := r.Clone(r.Context())
	outbound.RequestURI = ""

	transport := &http.Transport{
		DialContext:           s.options.Dial,
		DisableKeepAlives:     false,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	defer transport.CloseIdleConnections()

	response, err := transport.RoundTrip(outbound)
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
	_, _ = io.Copy(w, response.Body)
}

func isHopByHop(name string) bool {
	switch strings.ToLower(name) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate",
		"proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	}
	return false
}
