package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A session has no UA of its own: the installation identity names it. A
// request made before that is refused instead of presenting a platform this
// package would have to guess.
func TestARequestNeedsAUserAgent(t *testing.T) {
	var seen []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("User-Agent"))
		fmt.Fprint(w, `{"code":0,"data":{"selfId":"self","deviceTrusted":false}}`)
	}))
	defer server.Close()

	session := NewSession(newTestHost(server), nil)
	session.SetPlatform("Linux")
	if _, err := session.QueryDevice(); !errors.Is(err, ErrMissingUserAgent) {
		t.Fatalf("error = %v, want ErrMissingUserAgent", err)
	}
	if len(seen) != 0 {
		t.Fatalf("a request without a UA reached the server: %v", seen)
	}

	custom := "Mozilla/5.0 (X11; Linux x86_64) aTrustTray-Test"
	session.SetUserAgent(custom)
	if _, err := session.QueryDevice(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != custom {
		t.Fatalf("user agent = %v, want %q", seen, custom)
	}

	session.SetUserAgent("")
	if session.userAgent != custom {
		t.Fatalf("an empty override replaced the UA with %q", session.userAgent)
	}
}

// newTestHost is the host form the session expects from a TLS test server.
func newTestHost(server *httptest.Server) string {
	return server.URL[len("https://"):]
}
