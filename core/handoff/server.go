// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package handoff

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
)

// Source returns the snapshot a request should be answered from. The PAC is
// generated per request so a reconnect or a new grant needs no restart.
type Source func() Snapshot

// Server serves the PAC on one loopback listener. It holds no session state:
// the source does.
type Server struct {
	bind   string
	source Source
	logf   func(string, ...any)

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
}

// New builds the server. Start performs the bind.
func New(bind string, source Source, logf func(string, ...any)) *Server {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Server{bind: strings.TrimSpace(bind), source: source, logf: logf}
}

// Start binds the listener and serves until Close or ctx is done.
func (s *Server) Start(ctx context.Context) error {
	if s.bind == "" {
		return errors.New("handoff: no listen address")
	}
	listener, err := net.Listen("tcp", s.bind)
	if err != nil {
		return fmt.Errorf("listen for the PAC on %s: %w", s.bind, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/proxy.pac", s.servePAC)
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.mu.Lock()
	s.listener = listener
	s.server = server
	s.mu.Unlock()

	s.logf("PAC listening on %s", listener.Addr())
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			s.logf("PAC listener stopped: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()
	return nil
}

// URL is the address a browser or an extension should use. Empty before Start.
func (s *Server) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return ""
	}
	return "http://" + s.listener.Addr().String() + "/proxy.pac"
}

// Close stops the listener. It is safe to call more than once.
func (s *Server) Close() error {
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.listener = nil
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Close()
}

func (s *Server) servePAC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "the PAC endpoint is read-only", http.StatusMethodNotAllowed)
		return
	}
	snapshot := Snapshot{}
	if s.source != nil {
		snapshot = s.source()
	}
	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.WriteString(w, snapshot.PAC())
}
