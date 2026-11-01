package atrust

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net"
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

func TestLoginReportsOneGeneratedMACAndReusesIt(t *testing.T) {
	stub := newLoginStub(t)
	c := NewClient(ClientOptions{})
	t.Cleanup(c.Close)

	stored := loginAgainstStub(t, stub, c, auth.ClientAuthData{})
	if len(stored.MACAddresses) != 1 {
		t.Fatalf("stored mac_addresses = %v, want exactly one", stored.MACAddresses)
	}
	if len(stub.reportedMAC) != 1 || stub.reportedMAC[0][0] != stored.MACAddresses[0] {
		t.Fatalf("reported = %v, stored = %v", stub.reportedMAC, stored.MACAddresses)
	}
	address, err := net.ParseMAC(stored.MACAddresses[0])
	if err != nil {
		t.Fatalf("generated MAC %q: %v", stored.MACAddresses[0], err)
	}
	if address[0]&0x02 == 0 || address[0]&0x01 != 0 {
		t.Fatalf("generated MAC %q is not locally administered and unicast", address)
	}

	// A second login reads the record back instead of generating: the portal
	// binds the trusted endpoint to this value.
	second := loginAgainstStub(t, stub, c, stored)
	if len(second.MACAddresses) != 1 || second.MACAddresses[0] != stored.MACAddresses[0] {
		t.Fatalf("second login mac_addresses = %v, want %v", second.MACAddresses, stored.MACAddresses)
	}
	if len(stub.reportedMAC) != 2 || stub.reportedMAC[1][0] != stored.MACAddresses[0] {
		t.Fatalf("second report = %v", stub.reportedMAC)
	}
}

// TestOlderClientDataIsUpgradedOnce: a record written before the field existed
// gains exactly one address, and the login that wrote it back is the last one
// that generates.
func TestOlderClientDataIsUpgradedOnce(t *testing.T) {
	stub := newLoginStub(t)
	c := NewClient(ClientOptions{})
	t.Cleanup(c.Close)

	// What an older version stored: a device id and nothing else.
	older := auth.ClientAuthData{DeviceID: "0123456789abcdef0123456789abcdef"}
	upgraded := loginAgainstStub(t, stub, c, older)
	if upgraded.DeviceID != older.DeviceID {
		t.Fatalf("device id = %q, want %q", upgraded.DeviceID, older.DeviceID)
	}
	if len(upgraded.MACAddresses) != 1 {
		t.Fatalf("upgraded mac_addresses = %v", upgraded.MACAddresses)
	}

	// The record as it reached the file, read back, is what the next login uses.
	encoded, err := json.Marshal(upgraded)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded auth.ClientAuthData
	if err := json.Unmarshal(encoded, &reloaded); err != nil {
		t.Fatal(err)
	}
	if len(reloaded.MACAddresses) != 1 || reloaded.MACAddresses[0] != upgraded.MACAddresses[0] {
		t.Fatalf("reloaded mac_addresses = %v, want %v", reloaded.MACAddresses, upgraded.MACAddresses)
	}

	next := NewClient(ClientOptions{})
	t.Cleanup(next.Close)
	again := loginAgainstStub(t, stub, next, reloaded)
	if len(again.MACAddresses) != 1 || again.MACAddresses[0] != upgraded.MACAddresses[0] {
		t.Fatalf("second login mac_addresses = %v, want %v", again.MACAddresses, upgraded.MACAddresses)
	}
}

func TestEnsureMACAddressesDecidesOnce(t *testing.T) {
	if got := ensureMACAddresses([]string{"02:00:5e:10:00:01"}, []string{"02:00:5e:10:00:02"}); len(got) != 1 || got[0] != "02:00:5e:10:00:01" {
		t.Fatalf("stored must win: %v", got)
	}
	if got := ensureMACAddresses(nil, []string{"02:00:5e:10:00:02"}); len(got) != 1 || got[0] != "02:00:5e:10:00:02" {
		t.Fatalf("configured must be used when nothing is stored: %v", got)
	}
	first := ensureMACAddresses(nil, nil)
	if len(first) != 1 {
		t.Fatalf("generated = %v", first)
	}
	if _, err := net.ParseMAC(first[0]); err != nil {
		t.Fatalf("generated MAC %q: %v", first[0], err)
	}
}
