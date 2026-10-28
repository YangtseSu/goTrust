// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package handoff

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/session"
)

func TestServerServesTheLivePAC(t *testing.T) {
	cfg := testConfig()
	snapshot := FromState(cfg, session.State{Phase: session.PhaseOnline, Resources: testResources()})
	server := New("127.0.0.1:0", func() Snapshot { return snapshot }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	if !strings.HasSuffix(server.URL(), "/proxy.pac") {
		t.Fatalf("URL = %q", server.URL())
	}
	response, err := http.Get(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); got != "application/x-ns-proxy-autoconfig" {
		t.Fatalf("content type = %q", got)
	}
	if !strings.Contains(string(body), "isInNet") {
		t.Fatalf("PAC = %s", body)
	}

	missing, err := http.Get(strings.TrimSuffix(server.URL(), "/proxy.pac") + "/")
	if err != nil {
		t.Fatal(err)
	}
	missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown path status = %d", missing.StatusCode)
	}

	post, err := http.Post(server.URL(), "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	post.Body.Close()
	if post.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", post.StatusCode)
	}
}

func TestServerOfflineServesDirect(t *testing.T) {
	server := New("127.0.0.1:0", func() Snapshot { return Snapshot{} }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	response, err := http.Get(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if strings.Contains(string(body), "isInNet") || !strings.Contains(string(body), "DIRECT") {
		t.Fatalf("offline PAC = %s", body)
	}
}

func TestServerRejectsAnEmptyBind(t *testing.T) {
	server := New("", func() Snapshot { return Snapshot{} }, nil)
	if err := server.Start(context.Background()); err == nil {
		t.Fatal("an empty bind was accepted")
	}
}

func TestServerCloseIsIdempotent(t *testing.T) {
	server := New("127.0.0.1:0", func() Snapshot { return Snapshot{} }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
}
