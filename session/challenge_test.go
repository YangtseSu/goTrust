// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"strings"
	"testing"
	"time"
)

// The callback has to be the portal's own endpoint with the ticket (CAS) or
// code (OAuth2); anything else would fail inside the client with a less
// helpful message.
func TestValidateCallbackURL(t *testing.T) {
	cases := []struct {
		name    string
		kind    ExternalKind
		raw     string
		wantErr string
	}{
		{
			name: "CAS ticket",
			kind: ExternalCAS,
			raw:  "https://vpn.example.com/passport/v1/auth/cas?sfDomain=local&ticket=ST-123",
		},
		{
			name: "OAuth2 code",
			kind: ExternalOAuth2,
			raw:  "https://vpn.example.com/passport/v1/auth/httpsOauth2?code=abc&state=null",
		},
		{
			name:    "CAS without a ticket",
			kind:    ExternalCAS,
			raw:     "https://vpn.example.com/passport/v1/auth/cas?sfDomain=local",
			wantErr: "ticket",
		},
		{
			name:    "OAuth2 code at the CAS path",
			kind:    ExternalOAuth2,
			raw:     "https://vpn.example.com/passport/v1/auth/cas?code=abc",
			wantErr: "httpsOauth2",
		},
		{
			name:    "plain http",
			kind:    ExternalCAS,
			raw:     "http://vpn.example.com/passport/v1/auth/cas?ticket=ST-1",
			wantErr: "https",
		},
		{
			name:    "not a URL",
			kind:    ExternalCAS,
			raw:     "://nope",
			wantErr: "invalid",
		},
		{
			name:    "unknown kind",
			kind:    ExternalKind("saml"),
			raw:     "https://vpn.example.com/passport/v1/auth/cas?ticket=ST-1",
			wantErr: "unsupported",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateCallbackURL(testCase.kind, testCase.raw)
			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("validate: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("an invalid callback was accepted")
			}
			if !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err, testCase.wantErr)
			}
		})
	}
}

// A stored TOTP secret is turned into the current code with the standard
// algorithm; the RFC 6238 vector is the reference for the truncation.
func TestGenerateTOTPCode(t *testing.T) {
	// Base32 of "12345678901234567890".
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	code, err := GenerateTOTPCode(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if code != "287082" {
		t.Fatalf("code = %q, want 287082", code)
	}

	if _, err := GenerateTOTPCode("", time.Now()); err == nil {
		t.Fatal("an empty secret produced a code")
	}
	if _, err := GenerateTOTPCode("not base32!", time.Now()); err == nil {
		t.Fatal("a malformed secret produced a code")
	}
}
