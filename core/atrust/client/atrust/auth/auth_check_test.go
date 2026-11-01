package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The recorded answer of a strict gateway: the account may log in but the
// installation is not trusted yet, so the portal names the bind step next to a
// non-zero code.
const trustDeviceAuthCheckAnswer = `{"code":75500503,"message":"The client is not installed, ` +
	`or login in web-only mode is performed. No trusted endpoint can be added.",` +
	`"data":{"currentService":"auth/authCheck","type":"trustDevice",` +
	`"nextService":"auth/bindAuthDevice",` +
	`"nextServiceList":[{"authId":"-1","authType":"auth/bindAuthDevice"}]}}`

func TestAuthCheckFollowsNextServiceOnNonZeroCode(t *testing.T) {
	var bindCalls int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/passport/v1/auth/authCheck":
			fmt.Fprint(w, trustDeviceAuthCheckAnswer)
		case "/passport/v1/auth/bindAuthDevice":
			bindCalls++
			fmt.Fprint(w, `{"code":0,"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	session := newTLSTestSession(server)
	session.deviceID = "0123456789abcdef0123456789abcdef"
	if err := session.continueAuth(authStep{Service: "auth/authCheck"}); err != nil {
		t.Fatalf("continueAuth() error = %v, want the bind step to run", err)
	}
	if bindCalls != 1 {
		t.Fatalf("bindAuthDevice calls = %d, want 1", bindCalls)
	}
}

func TestAuthCheckNonZeroCodeWithoutStepFails(t *testing.T) {
	for _, service := range []string{"", "auth/sms/tokenExchange", "auth/token"} {
		t.Run("service="+service, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/passport/v1/auth/authCheck" {
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
				fmt.Fprintf(w, `{"code":75500503,"message":"the client is not installed","data":{"nextService":%q}}`, service)
			}))
			defer server.Close()

			_, err := newTLSTestSession(server).authCheck()
			if err == nil {
				t.Fatal("authCheck() error = nil, want the portal code")
			}
			message := err.Error()
			if !strings.Contains(message, "75500503") || !strings.Contains(message, "the client is not installed") {
				t.Fatalf("authCheck() error = %q, want the portal code and message", message)
			}
		})
	}
}

func TestAuthCheckKeepsZeroCodeStep(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":0,"data":{"nextService":"auth/totp","nextServiceList":[{"authId":"totp-id","authType":"auth/totp"}]}}`)
	}))
	defer server.Close()

	step, err := newTLSTestSession(server).authCheck()
	if err != nil {
		t.Fatal(err)
	}
	if step.Service != "auth/totp" || step.AuthID != "totp-id" {
		t.Fatalf("step = %+v", step)
	}
}

func TestAuthCheckCapturesPasswordChangeOnNonZeroCode(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":75500402,"message":"the password expired",`+
			`"data":{"nextService":"auth/modifyPwd","modifyPwdCode":75500402,`+
			`"modifyPwdMessage":"expired","username":"student","domain":"local"}}`)
	}))
	defer server.Close()

	session := newTLSTestSession(server)
	step, err := session.authCheck()
	if err != nil {
		t.Fatal(err)
	}
	if step.Service != "auth/modifyPwd" {
		t.Fatalf("step = %+v, want the password change step", step)
	}
	challenge := session.pendingPasswordChange()
	if challenge.Code != 75500402 || challenge.Username != "student" || challenge.Domain != "local" {
		t.Fatalf("challenge = %+v", challenge)
	}
}

// TestLoginReachesTheBindStepThroughTheLoop pins the recorded strict-gateway
// sequence end to end: authCheck answers 75500503 with the bind step, the bind
// endpoint completes it, and the login walks on to onlineInfo.
func TestLoginReachesTheBindStepThroughTheLoop(t *testing.T) {
	var bindCalls int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/passport/v1/public/authConfig":
			fmt.Fprint(w, `{"code":0,"data":{"isLogin":0,"csrfToken":"csrf","ticket":"ticket","authServerInfoList":[]}}`)
		case "/controller/v1/public/reportEnv":
			fmt.Fprint(w, `{"code":0}`)
		case "/passport/v1/auth/authCheck":
			fmt.Fprint(w, trustDeviceAuthCheckAnswer)
		case "/passport/v1/auth/bindAuthDevice":
			bindCalls++
			fmt.Fprint(w, `{"code":0,"data":{"nextService":""}}`)
		case "/passport/v1/user/onlineInfo":
			fmt.Fprint(w, `{"code":0,"data":{"isOnline":true,"username":"student"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	session := newTLSTestSession(server)
	session.ticket = "ticket"
	if err := session.continueAuth(authStep{Service: "auth/authCheck"}); err != nil {
		t.Fatalf("continueAuth() error = %v", err)
	}
	if bindCalls != 1 {
		t.Fatalf("bindAuthDevice calls = %d, want 1", bindCalls)
	}
}
