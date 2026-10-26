// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/config"
)

func testMethods() []AuthMethod {
	return []AuthMethod{
		{Name: "密码登录", Type: "auth/psw", Domain: "local"},
		{Name: "密码登录", Type: "auth/psw", Domain: "Radius"},
	}
}

// The only offered method needs no question; a stored pair wins only when the
// gateway still offers it; several methods without a stored pair are a choice.
func TestResolveAuthMethod(t *testing.T) {
	only := []AuthMethod{{Name: "CAS", Type: "auth/cas", Domain: "local", URL: "https://sso.example.com/login"}}

	cases := []struct {
		name     string
		methods  []AuthMethod
		authType string
		domain   string
		want     AuthMethod
		errIs    error
		errHas   string
	}{
		{
			name:    "single method is automatic",
			methods: only,
			want:    only[0],
		},
		{
			name:     "stored pair wins",
			methods:  testMethods(),
			authType: "auth/psw",
			domain:   "Radius",
			want:     testMethods()[1],
		},
		{
			name:    "several methods need a choice",
			methods: testMethods(),
			errIs:   ErrAuthMethodChoice,
			errHas:  "auth/psw · Radius",
		},
		{
			name:     "a stored pair the gateway dropped is an error with the list",
			methods:  only,
			authType: "auth/psw",
			domain:   "local",
			errHas:   "auth/cas · local",
		},
		{
			name:     "a stored domain that moved is not silently replaced",
			methods:  testMethods(),
			authType: "auth/psw",
			domain:   "local2",
			errHas:   "auth/psw · local",
		},
		{
			name:    "an empty list cannot be resolved",
			methods: nil,
			errHas:  "did not offer",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveAuthMethod(testCase.methods, testCase.authType, testCase.domain)
			switch {
			case testCase.errIs != nil:
				if !errors.Is(err, testCase.errIs) {
					t.Fatalf("err = %v, want %v", err, testCase.errIs)
				}
			case testCase.errHas != "":
				if err == nil {
					t.Fatalf("resolved %+v, want an error", got)
				}
			default:
				if err != nil {
					t.Fatalf("resolve: %v", err)
				}
				if got != testCase.want {
					t.Fatalf("method = %+v, want %+v", got, testCase.want)
				}
			}
			if testCase.errHas != "" && !strings.Contains(err.Error(), testCase.errHas) {
				t.Fatalf("error = %q, want it to mention %q", err, testCase.errHas)
			}
		})
	}
}

// Discovery reads the same authConfig answer a login does, drops the duplicate
// entry gateways repeat and ignores an entry without a type.
func TestDiscoverAuthMethodsParsesTheGatewayAnswer(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/passport/v1/public/authConfig" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{
			"code": 0,
			"data": {
				"isLogin": 0,
				"csrfToken": "csrf",
				"authServerInfoList": [
					{"loginDomain":"local","authId":"1","authType":"auth/psw","authName":"密码登录"},
					{"loginDomain":"Radius","authId":"2","authType":"auth/psw","authName":"密码登录"},
					{"loginDomain":"local","authId":"3","authType":"auth/cas","authName":"统一认证","loginUrl":"https://sso.example.com/login"},
					{"loginDomain":"local","authId":"1","authType":"auth/psw","authName":"密码登录"},
					{"loginDomain":"local","authId":"4","authType":"","authName":"broken"}
				]
			}
		}`))
	}))
	defer server.Close()

	methods, err := discoverAuthMethods(context.Background(), strings.TrimPrefix(server.URL, "https://"), testDial(server))
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(methods) != 3 {
		t.Fatalf("methods = %+v, want three", methods)
	}
	if methods[0] != (AuthMethod{Name: "密码登录", Type: "auth/psw", Domain: "local"}) {
		t.Fatalf("first method = %+v", methods[0])
	}
	if methods[1].Domain != "Radius" {
		t.Fatalf("second method = %+v", methods[1])
	}
	if methods[2].URL != "https://sso.example.com/login" || methods[2].Type != "auth/cas" {
		t.Fatalf("third method = %+v", methods[2])
	}
}

// A gateway that considers the client logged in answers with an empty list;
// that is not an error, because the stored session then does the login.
func TestDiscoverAuthMethodsAcceptsAnEmptyList(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":{"isLogin":1,"csrfToken":"csrf"}}`))
	}))
	defer server.Close()

	methods, err := discoverAuthMethods(context.Background(), strings.TrimPrefix(server.URL, "https://"), testDial(server))
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("methods = %+v, want none", methods)
	}
}

// Cancelling the login attempt must stop the discovery request instead of
// leaving it to time out on its own.
func TestDiscoverAuthMethodsHonoursTheContext(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-blocked
	}))
	defer server.Close()
	defer close(blocked)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	if _, err := discoverAuthMethods(ctx, strings.TrimPrefix(server.URL, "https://"), testDial(server)); err == nil {
		t.Fatal("a cancelled discovery returned a method list")
	}
}

func TestAuthHostOmitsThePortAt443(t *testing.T) {
	if got := authHost("vpn.example.com", 443); got != "vpn.example.com" {
		t.Fatalf("host = %q", got)
	}
	if got := authHost("vpn.example.com", 8443); got != "vpn.example.com:8443" {
		t.Fatalf("host = %q", got)
	}
	if got := authHost("2001:db8::1", 443); got != "[2001:db8::1]" {
		t.Fatalf("host = %q", got)
	}
	if got := authHost("2001:db8::1", 8443); got != "[2001:db8::1]:8443" {
		t.Fatalf("host = %q", got)
	}
}

// The primary login type has to be one this build can complete.
func TestNewLoginMethodRejectsUnsupportedPrimaryTypes(t *testing.T) {
	auth := config.AuthConfig{Username: "alice", Password: "secret"}
	if method, err := newLoginMethod(auth, AuthMethod{Type: "auth/psw", Domain: "local"}); err != nil || method == nil {
		t.Fatalf("password login: %v, %v", method, err)
	}
	if _, err := newLoginMethod(auth, AuthMethod{Type: "auth/smsCheckCode", Domain: "local"}); err == nil {
		t.Fatal("a primary SMS login was accepted without a phone number to send it to")
	}
}

func testDial(server *httptest.Server) func(context.Context, string, string) (net.Conn, error) {
	address := server.Listener.Addr().String()
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		dialer := &net.Dialer{}
		return dialer.DialContext(ctx, network, address)
	}
}
