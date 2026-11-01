package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YangtseSu/goTrust/core/atrust/client/authchallenge"
)

// bindStub answers the bind endpoint with the recorded shapes: the first answer
// says whether the account has room, the second completes the step.
type bindStub struct {
	server   *httptest.Server
	answers  []string
	payloads []struct {
		UnbindList string `json:"unbindList"`
		Reason     string `json:"reason"`
	}
}

func newBindStub(t *testing.T, answers ...string) *bindStub {
	t.Helper()

	stub := &bindStub{answers: answers}
	stub.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/passport/v1/auth/bindAuthDevice" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var payload struct {
			UnbindList string `json:"unbindList"`
			Reason     string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		stub.payloads = append(stub.payloads, payload)
		if len(stub.payloads) > len(stub.answers) {
			t.Errorf("unexpected extra bind request")
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, stub.answers[len(stub.payloads)-1])
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func TestBindAuthDeviceWithoutLimitFollowsTheStep(t *testing.T) {
	stub := newBindStub(t, `{"code":0,"data":{"nextService":"auth/authCheck","nextServiceList":[{"authId":"-1","authType":"auth/authCheck"}]}}`)

	session := newTLSTestSession(stub.server)
	session.challengeHandler = authchallenge.HandlerFuncs{
		Code: func(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
			t.Fatalf("no device list was needed, but the client asked: %+v", challenge)
			return authchallenge.CodeResponse{}, nil
		},
	}
	step, err := session.bindAuthDevice()
	if err != nil {
		t.Fatal(err)
	}
	if step.Service != "auth/authCheck" {
		t.Fatalf("step = %+v", step)
	}
	if len(stub.payloads) != 1 {
		t.Fatalf("bind requests = %d, want 1", len(stub.payloads))
	}
	if stub.payloads[0].UnbindList != "[]" || stub.payloads[0].Reason != "" {
		t.Fatalf("first payload = %+v, want an empty unbind list", stub.payloads[0])
	}
}

func TestBindAuthDeviceUnbindsTheChosenDevice(t *testing.T) {
	stub := newBindStub(t,
		`{"code":0,"data":{"bindStatus":1,"curBindNum":2,"bindNumLimit":2,"trustedDeviceList":[`+
			`{"id":"device-a","deviceName":"laptop","os":"linux","bindTime":"2026-10-01 09:00"},`+
			`{"id":"device-b","deviceName":"desktop","os":"windows","bindTime":"2026-10-02 10:00"}]}}`,
		`{"code":0,"data":{"nextService":"auth/enhancedConfirm"}}`,
	)

	session := newTLSTestSession(stub.server)
	session.challengeHandler = authchallenge.HandlerFuncs{
		Code: func(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
			if challenge.Kind != authchallenge.CodeDevice {
				t.Fatalf("challenge kind = %q", challenge.Kind)
			}
			for _, want := range []string{"1) laptop", "2) desktop", "os=windows", "bound=2026-10-02 10:00"} {
				if !strings.Contains(challenge.Message, want) {
					t.Fatalf("message = %q, want %q", challenge.Message, want)
				}
			}
			// Two bound devices against a limit of two leaves room for one
			// more only after exactly one is unbound.
			return authchallenge.CodeResponse{Code: "2"}, nil
		},
	}
	step, err := session.bindAuthDevice()
	if err != nil {
		t.Fatal(err)
	}
	if step.Service != "auth/enhancedConfirm" {
		t.Fatalf("step = %+v", step)
	}
	if len(stub.payloads) != 2 {
		t.Fatalf("bind requests = %d, want 2", len(stub.payloads))
	}
	if got := stub.payloads[1].UnbindList; got != `["device-b"]` {
		t.Fatalf("unbindList = %s, want the chosen device", got)
	}
}

func TestBindAuthDeviceAsksForEnoughDevices(t *testing.T) {
	// Three bound against a limit of two: two have to go before this one fits.
	stub := newBindStub(t,
		`{"code":0,"data":{"bindStatus":1,"curBindNum":3,"bindNumLimit":2,"trustedDeviceList":[`+
			`{"id":"a","deviceName":"one"},{"id":"b","deviceName":"two"},{"id":"c","deviceName":"three"}]}}`,
		`{"code":0,"data":{"nextService":"auth/enhancedDone"}}`,
	)

	session := newTLSTestSession(stub.server)
	asked := ""
	session.challengeHandler = authchallenge.HandlerFuncs{
		Code: func(challenge authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
			asked = challenge.Message
			return authchallenge.CodeResponse{Code: "1, 3"}, nil
		},
	}
	if _, err := session.bindAuthDevice(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asked, "Unbind 1, 2") {
		t.Fatalf("message = %q, want it to name two devices", asked)
	}
	if got := stub.payloads[1].UnbindList; got != `["a","c"]` {
		t.Fatalf("unbindList = %s", got)
	}
}

func TestBindAuthDeviceRejectsAnswersThatDoNotFit(t *testing.T) {
	const listed = `{"code":0,"data":{"bindStatus":1,"curBindNum":1,"bindNumLimit":1,"trustedDeviceList":[` +
		`{"id":"a","deviceName":"one"},{"id":"b","deviceName":"two"}]}}`

	cases := map[string]struct {
		answer string
		want   string
	}{
		"too few":      {"", "choose exactly 1 trusted device number"},
		"too many":     {"1,2", "choose exactly 1 trusted device number"},
		"out of range": {"9", "no trusted device numbered"},
		"not a number": {"laptop", "no trusted device numbered"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stub := newBindStub(t, listed)
			session := newTLSTestSession(stub.server)
			session.challengeHandler = authchallenge.HandlerFuncs{
				Code: func(authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
					return authchallenge.CodeResponse{Code: tc.answer}, nil
				},
			}
			_, err := session.bindAuthDevice()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if len(stub.payloads) != 1 {
				t.Fatalf("bind requests = %d, want only the first", len(stub.payloads))
			}
		})
	}
}

func TestBindAuthDeviceRejectsARepeatedDevice(t *testing.T) {
	stub := newBindStub(t, `{"code":0,"data":{"bindStatus":1,"curBindNum":3,"bindNumLimit":2,"trustedDeviceList":[`+
		`{"id":"a","deviceName":"one"},{"id":"b","deviceName":"two"},{"id":"c","deviceName":"three"}]}}`)

	session := newTLSTestSession(stub.server)
	session.challengeHandler = authchallenge.HandlerFuncs{
		Code: func(authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
			return authchallenge.CodeResponse{Code: "2,2"}, nil
		},
	}
	_, err := session.bindAuthDevice()
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("error = %v, want the repeated number reported", err)
	}
}

func TestBindAuthDeviceWithoutAListFails(t *testing.T) {
	stub := newBindStub(t, `{"code":0,"data":{"bindStatus":1,"curBindNum":2,"bindNumLimit":1}}`)

	session := newTLSTestSession(stub.server)
	session.challengeHandler = authchallenge.HandlerFuncs{
		Code: func(authchallenge.CodeChallenge) (authchallenge.CodeResponse, error) {
			t.Fatalf("nothing could be chosen, but the client asked")
			return authchallenge.CodeResponse{}, nil
		},
	}
	_, err := session.bindAuthDevice()
	if err == nil || !strings.Contains(err.Error(), "listed no device to unbind") {
		t.Fatalf("error = %v", err)
	}
}
