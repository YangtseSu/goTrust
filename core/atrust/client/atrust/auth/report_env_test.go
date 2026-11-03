package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// reportEnvStub is the portal as a login sees it for the two calls this file
// pins: authConfig, which carries the anti-MITM data the signature is derived
// from, and reportEnv itself.
type reportEnvStub struct {
	server *httptest.Server
	// authConfigBody overrides the answer, so a test can present a gateway that
	// sends no anti-MITM data.
	authConfigBody string
	requests       int
	sigHeader      string
	macAddresses   []string
	ticket         string
}

func newReportEnvStub(t *testing.T) *reportEnvStub {
	t.Helper()

	stub := &reportEnvStub{}
	stub.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/passport/v1/public/authConfig":
			if stub.authConfigBody != "" {
				fmt.Fprint(w, stub.authConfigBody)
				return
			}
			fmt.Fprintf(w, `{"code":0,"data":{"isLogin":0,"csrfToken":"csrf","ticket":"ticket",`+
				`"authServerInfoList":[{"loginDomain":"local","authType":"auth/psw"}],`+
				`"antiMITMAttackData":%s}}`, signedAntiMITMJSON(t, 1, "", ""))
		case "/controller/v1/public/reportEnv":
			stub.requests++
			stub.sigHeader = r.Header.Get("X-Request-Sig")
			var payload struct {
				Ticket string `json:"ticket"`
				Env    struct {
					Endpoint struct {
						MACAddresses []string `json:"mac_addresses"`
					} `json:"endpoint"`
				} `json:"env"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			stub.macAddresses = payload.Env.Endpoint.MACAddresses
			stub.ticket = payload.Ticket
			fmt.Fprint(w, `{"code":0}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func TestReportEnvCarriesTheStoredMACAndRequestSignature(t *testing.T) {
	stub := newReportEnvStub(t)

	session := newTLSTestSession(stub.server)
	session.deviceID = "0123456789abcdef0123456789abcdef"
	session.macAddresses = []string{"02:00:5e:10:00:01"}
	if _, _, err := session.authConfig(false, true); err != nil {
		t.Fatal(err)
	}
	if err := session.reportEnv(); err != nil {
		t.Fatal(err)
	}
	if stub.requests != 1 {
		t.Fatalf("reportEnv calls = %d, want 1", stub.requests)
	}
	if len(stub.macAddresses) != 1 || stub.macAddresses[0] != "02:00:5e:10:00:01" {
		t.Fatalf("mac_addresses = %v, want the stored address", stub.macAddresses)
	}
	if stub.ticket != "ticket" {
		t.Fatalf("ticket = %q, want the one authConfig issued", stub.ticket)
	}

	// The header is the HMAC of the path with its query followed by the body,
	// keyed by the anti-MITM answer's derived key.
	key := antiMITMSignatureKey(antiMITMAttackData{
		DevicePubKeyMod: "A1B2C3",
		DevicePubKeyExp: "10001",
		Challenge:       "challenge-vector",
	})
	want := antiMITMHMAC(key, []byte("/controller/v1/public/reportEnv?"+
		url.Values{
			"clientType": {"SDPClient"},
			"platform":   {"Linux"},
			"lang":       {"en-US"},
		}.Encode()+reportEnvBody(t, "0123456789abcdef0123456789abcdef", "ticket", []string{"02:00:5e:10:00:01"})))
	if stub.sigHeader != want {
		t.Fatalf("X-Request-Sig = %q, want %q", stub.sigHeader, want)
	}
}

// reportEnvBody rebuilds the body the session sends. The portal signs exactly
// these bytes, so the test has to name them rather than float a field.
func reportEnvBody(t *testing.T, deviceID, ticket string, macs []string) string {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"ticket":   ticket,
		"deviceId": deviceID,
		"env": map[string]interface{}{
			"endpoint": map[string]interface{}{
				"device_id": deviceID,
				"device": map[string]interface{}{
					"type": "goTrust",
				},
				"mac_addresses": macs,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestReportEnvSignatureUsesTheAntiMITMKey(t *testing.T) {
	stub := newReportEnvStub(t)

	session := newTLSTestSession(stub.server)
	session.deviceID = "device"
	session.macAddresses = []string{"02:00:5e:10:00:02"}
	if _, _, err := session.authConfig(false, true); err != nil {
		t.Fatal(err)
	}
	if len(session.signKey) == 0 {
		t.Fatal("authConfig did not derive a request signature key")
	}
	if err := session.reportEnv(); err != nil {
		t.Fatal(err)
	}
	if stub.sigHeader == "" {
		t.Fatal("reportEnv did not sign the request")
	}
	// The key is the one the anti-MITM challenge derives, not an unrelated
	// random value: the same data reproduces the same header.
	want := antiMITMHMAC(session.signKey, []byte("/controller/v1/public/reportEnv?"+
		url.Values{
			"clientType": {"SDPClient"},
			"platform":   {"Linux"},
			"lang":       {"en-US"},
		}.Encode()+reportEnvBody(t, "device", "ticket", []string{"02:00:5e:10:00:02"})))
	if stub.sigHeader != want {
		t.Fatalf("X-Request-Sig = %q, want %q", stub.sigHeader, want)
	}
}
