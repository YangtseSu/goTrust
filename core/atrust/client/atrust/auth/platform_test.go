package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPlatformQueryFollowsTheSession: the platform= value lives on the
// session, so every request names the platform the caller derived from the
// account identity rather than a constant of this package.
func TestPlatformQueryFollowsTheSession(t *testing.T) {
	var seen []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query().Get("platform"))
		fmt.Fprint(w, `{"code":0,"data":{"selfId":"self","deviceTrusted":false}}`)
	}))
	defer server.Close()

	session := newTLSTestSession(server)
	want := []string{"Linux", "Windows", "Darwin"}
	for _, platform := range want {
		session.SetPlatform(platform)
		if _, err := session.QueryDevice(); err != nil {
			t.Fatalf("QueryDevice() with platform %q: %v", platform, err)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("platform queries = %v, want %v", seen, want)
	}
	for index := range want {
		if seen[index] != want[index] {
			t.Fatalf("query %d platform = %q, want %q", index, seen[index], want[index])
		}
	}
}
