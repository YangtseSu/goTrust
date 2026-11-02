package atrust

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YangtseSu/goTrust/core/atrust/client/atrust/auth"
)

// loginStub is the smallest portal a login completes against: authConfig,
// reportEnv, authCheck, clientResource and onlineInfo. It records the
// mac_addresses each reportEnv carried, which is the value under test.
type loginStub struct {
	server      *httptest.Server
	key         *rsa.PrivateKey
	reportedMAC [][]string
}

func newLoginStub(t *testing.T) *loginStub {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	stub := &loginStub{key: key}
	stub.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/passport/v1/public/authConfig":
			fmt.Fprintf(w, `{"code":0,"data":{"isLogin":0,"csrfToken":"csrf","pubKey":"%s","pubKeyExp":"%d","antiReplayRand":"rand",`+
				`"authServerInfoList":[{"loginDomain":"local","authType":"auth/psw"}]}}`,
				stub.key.PublicKey.N.Text(16), stub.key.PublicKey.E)
		case "/passport/v1/auth/psw":
			fmt.Fprint(w, `{"code":0,"data":{"ticket":"ticket","nextService":"auth/authCheck","antiReplayRand":"rand-2"}}`)
		case "/controller/v1/public/reportEnv":
			var payload struct {
				Env struct {
					Endpoint struct {
						MACAddresses []string `json:"mac_addresses"`
					} `json:"endpoint"`
				} `json:"env"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			stub.reportedMAC = append(stub.reportedMAC, payload.Env.Endpoint.MACAddresses)
			fmt.Fprint(w, `{"code":0}`)
		case "/passport/v1/auth/authCheck":
			fmt.Fprint(w, `{"code":0,"data":{"nextService":""}}`)
		case "/controller/v1/user/clientResource":
			fmt.Fprint(w, `{"code":0,"data":{"appList":{"data":{"appInfo":[],"config":{"nodeGroupConf":{"majorNodeGroup":{"id":"group"},"nodeGroupList":[]}}}},"sdpPolicy":{"data":{}}}}`)
		case "/passport/v1/user/onlineInfo":
			fmt.Fprint(w, `{"code":0,"data":{"isOnline":true,"username":"student"}}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

// loginAgainstStub runs one login through the client's own session
// establishment and returns the client data that would be stored.
func loginAgainstStub(t *testing.T, stub *loginStub, c *Client, clientData auth.ClientAuthData) auth.ClientAuthData {
	t.Helper()

	sess := auth.NewSession(strings.TrimPrefix(stub.server.URL, "https://"), nil)
	options := SetupOptions{
		LoginMethod: auth.PasswordLogin{Username: "student", Password: "secret", Domain: "local"},
	}
	if _, err := c.establishSession(context.Background(), sess, options, &clientData, nil); err != nil {
		t.Fatalf("establishSession() error = %v", err)
	}
	return clientData
}

// TestLoginReportsTheCallersAddress: the hardware address is decided by the
// caller and reported as given. The protocol layer has no card of its own to
// read, and inventing a value would name a device the portal never sees.
func TestLoginReportsTheCallersAddress(t *testing.T) {
	stub := newLoginStub(t)
	c := NewClient(ClientOptions{Session: SessionOptions{
		MACAddresses: []string{"00:1a:2b:3c:4d:5e", "00:1a:2b:3c:4d:5f"},
	}})
	t.Cleanup(c.Close)

	loginAgainstStub(t, stub, c, auth.ClientAuthData{})
	if len(stub.reportedMAC) != 1 {
		t.Fatalf("reportEnv calls = %d, want one", len(stub.reportedMAC))
	}
	want := []string{"00:1a:2b:3c:4d:5e", "00:1a:2b:3c:4d:5f"}
	if got := stub.reportedMAC[0]; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("reported = %v, want %v", got, want)
	}
}

// TestLoginReportsEveryCallerAddressOnALaterLogin: the address list is not
// frozen into the stored session, so a login that starts from a record written
// by an older build reports what the caller passes now rather than the value in
// the file.
func TestLoginReportsWhatTheCallerPassesOnALaterLogin(t *testing.T) {
	stub := newLoginStub(t)
	c := NewClient(ClientOptions{Session: SessionOptions{MACAddresses: []string{"00:1a:2b:3c:4d:5e"}}})
	t.Cleanup(c.Close)

	// A record from a build that still stored an address with the device id.
	stored := auth.ClientAuthData{
		DeviceID:  "0123456789abcdef0123456789abcdef",
		CSRFToken: "csrf",
	}
	loginAgainstStub(t, stub, c, stored)

	second := NewClient(ClientOptions{Session: SessionOptions{MACAddresses: []string{"00:1a:2b:3c:4d:5f"}}})
	t.Cleanup(second.Close)
	loginAgainstStub(t, stub, second, stored)
	if len(stub.reportedMAC) != 2 {
		t.Fatalf("reportEnv calls = %d, want two", len(stub.reportedMAC))
	}
	if got := stub.reportedMAC[1]; len(got) != 1 || got[0] != "00:1a:2b:3c:4d:5f" {
		t.Fatalf("second report = %v, want the caller's address", got)
	}
}

// TestLoginWithoutAnAddressReportsNone: a caller with nothing to report sends
// an empty list. The layer does not fill the gap, so a front-end that cannot
// read a card produces a visible gap at the portal instead of a device that
// does not exist.
func TestLoginWithoutAnAddressReportsNone(t *testing.T) {
	stub := newLoginStub(t)
	c := NewClient(ClientOptions{})
	t.Cleanup(c.Close)

	loginAgainstStub(t, stub, c, auth.ClientAuthData{})
	if len(stub.reportedMAC) != 1 {
		t.Fatalf("reportEnv calls = %d, want one", len(stub.reportedMAC))
	}
	if got := stub.reportedMAC[0]; len(got) != 0 {
		t.Fatalf("reported = %v, want none", got)
	}
}

// TestStoredAddressIsNotWrittenBack: the address is not part of the session
// material, so a record that carried one from an older build is cleaned as the
// login writes the record back.
func TestStoredAddressIsNotWrittenBack(t *testing.T) {
	stub := newLoginStub(t)
	c := NewClient(ClientOptions{Session: SessionOptions{MACAddresses: []string{"00:1a:2b:3c:4d:5e"}}})
	t.Cleanup(c.Close)

	encoded, err := json.Marshal(auth.ClientAuthData{DeviceID: "0123456789abcdef0123456789abcdef"})
	if err != nil {
		t.Fatal(err)
	}
	loginAgainstStub(t, stub, c, auth.ClientAuthData{})
	if strings.Contains(string(encoded), "mac_addresses") {
		t.Fatalf("the record still has an address field: %s", encoded)
	}
}
