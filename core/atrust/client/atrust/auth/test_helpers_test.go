package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testUserAgent is the identity a test session presents. A bare session has
// none: the protocol layer refuses its first request on purpose.
const testUserAgent = "goTrust test client"

// newTestSession builds a session with the identity a caller would give it.
// The name and argument list match NewSession so a test that needed no
// identity before still reads the same.
func newTestSession(host string, writer io.Writer) *Session {
	session := NewSession(host, writer)
	session.SetUserAgent(testUserAgent)
	session.SetPlatform("Linux")
	return session
}

func newTestSessionContext(ctx context.Context, host string, writer io.Writer) *Session {
	session := NewSessionContext(ctx, host, writer)
	session.SetUserAgent(testUserAgent)
	session.SetPlatform("Linux")
	return session
}

func newTLSTestSession(server *httptest.Server) *Session {
	return newTestSession(strings.TrimPrefix(server.URL, "https://"), nil)
}

func TestNewSessionUsesTLSKeyLogWriter(t *testing.T) {
	writer := new(strings.Builder)
	session := NewSession("vpn.example.com", writer)
	transport, ok := session.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", session.client.Transport)
	}
	if transport.TLSClientConfig.KeyLogWriter != writer {
		t.Fatal("session transport did not preserve the TLS key-log writer")
	}
}
