// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/locales"
)

// completeProfile is a profile a login can be built from.
func completeProfile() config.Profile {
	profile := config.NewProfile(config.DefaultProfileName)
	profile.Server.Address = "vpn.example.com"
	profile.Server.Port = 443
	profile.Auth.Username = "alice"
	return profile
}

// The window's sentence and the library's validation must agree on when a
// profile is usable: the first is shown, the second is the authority.
func TestProfileProblemMatchesValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*config.Profile)
	}{
		{"complete", func(*config.Profile) {}},
		{"no address", func(p *config.Profile) { p.Server.Address = "" }},
		{"blank address", func(p *config.Profile) { p.Server.Address = "   " }},
		{"no username", func(p *config.Profile) { p.Auth.Username = "" }},
		{"port zero", func(p *config.Profile) { p.Server.Port = 0 }},
		{"port too large", func(p *config.Profile) { p.Server.Port = 70000 }},
	}

	for _, tc := range cases {
		profile := completeProfile()
		tc.mutate(&profile)

		problem := ProfileProblem(profile)
		err := profile.Validate()
		if (problem == "") != (err == nil) {
			t.Fatalf("%s: ProfileProblem = %q, Validate = %v", tc.name, problem, err)
		}
		if problem != "" && !strings.HasPrefix(problem, "请填写") && !strings.HasPrefix(problem, "端口") {
			t.Fatalf("%s: %q is not a sentence the window can show", tc.name, problem)
		}
	}
}

// The gateway field accepts what an administrator or a browser hands over and
// splits it into the address and port the settings file stores.
func TestParseGateway(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		address string
		port    int
		problem string
	}{
		{"empty", "", "", 0, ""},
		{"host and port", "vpn.example.com:8443", "vpn.example.com", 8443, ""},
		{"bare host", "vpn.example.com", "vpn.example.com", 443, ""},
		{"padded", "  vpn.example.com:8443  ", "vpn.example.com", 8443, ""},
		{"https url", "https://vpn.example.com:8443", "vpn.example.com", 8443, ""},
		{"https url with a path", "https://vpn.example.com/portal", "vpn.example.com", 443, ""},
		{"uppercase scheme", "HTTPS://vpn.example.com", "vpn.example.com", 443, ""},
		{"trailing slash", "vpn.example.com:8443/", "vpn.example.com", 8443, ""},
		{"highest port", "vpn.example.com:65535", "vpn.example.com", 65535, ""},
		{"ipv6 bracketed", "[2001:db8::1]:8443", "2001:db8::1", 8443, ""},
		{"ipv6 bracketed without a port", "[2001:db8::1]", "2001:db8::1", 443, ""},
		{"ipv6 without brackets", "2001:db8::1", "2001:db8::1", 443, ""},
		{"ipv6 url", "https://[2001:db8::1]:443/", "2001:db8::1", 443, ""},
		{"other scheme", "http://vpn.example.com", "", 0, locales.ProblemGatewayScheme()},
		{"scheme only", "https://", "", 0, locales.ProblemAddress()},
		{"empty host", ":8443", "", 0, locales.ProblemAddress()},
		{"empty port", "vpn.example.com:", "", 0, locales.ProblemPort()},
		{"port zero", "vpn.example.com:0", "", 0, locales.ProblemPort()},
		{"port too large", "vpn.example.com:65536", "", 0, locales.ProblemPort()},
		{"text port", "vpn.example.com:https", "", 0, locales.ProblemPort()},
		{"unclosed bracket", "[2001:db8::1", "", 0, locales.ProblemAddress()},
		{"trailing bracket text", "[2001:db8::1]x", "", 0, locales.ProblemPort()},
		{"extra colon", "vpn.example.com:443:80", "", 0, locales.ProblemPort()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			address, port, problem := ParseGateway(tc.text)
			if address != tc.address || port != tc.port || problem != tc.problem {
				t.Fatalf("ParseGateway(%q) = %q, %d, %q; want %q, %d, %q",
					tc.text, address, port, problem, tc.address, tc.port, tc.problem)
			}
		})
	}
}

// The field text and the parser are inverses, IPv6 included, so saving and
// reopening the page shows the same gateway.
func TestGatewayTextRoundTrips(t *testing.T) {
	cases := []struct {
		address string
		port    int
		want    string
	}{
		{"", 443, ""},
		{"vpn.example.com", 443, "vpn.example.com:443"},
		{"vpn.example.com", 8443, "vpn.example.com:8443"},
		{"2001:db8::1", 443, "[2001:db8::1]:443"},
	}
	for _, tc := range cases {
		profile := config.NewProfile("round trip")
		profile.Server.Address = tc.address
		profile.Server.Port = tc.port

		text := gatewayText(profile)
		if text != tc.want {
			t.Fatalf("gatewayText(%q, %d) = %q, want %q", tc.address, tc.port, text, tc.want)
		}
		address, port, problem := ParseGateway(text)
		if problem != "" {
			t.Fatalf("ParseGateway(%q) refused the field text: %q", text, problem)
		}
		if tc.address == "" {
			if address != "" || port != 0 {
				t.Fatalf("ParseGateway(%q) = %q, %d, want the empty gateway", text, address, port)
			}
			continue
		}
		if address != tc.address || port != tc.port {
			t.Fatalf("ParseGateway(%q) = %q, %d, want %q, %d", text, address, port, tc.address, tc.port)
		}
	}
}

// The account page is the first-run form now: one gateway field, then the
// login, with no profile name and no separate port field.
func TestAccountFormOrder(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	_, tester := newShell(t, state, SettingsActions{}, PageAccount)
	texts := tester.Texts()

	order := []string{
		locales.ConnectSection(),
		locales.FieldGateway(),
		locales.FieldUsername(),
		locales.FieldPassword(),
		locales.AuthMethod(),
		locales.LoginDomain(),
		locales.SecuritySection(),
		locales.ChangePassword(),
	}
	last := -1
	for _, label := range order {
		found := -1
		for index, text := range texts {
			if text == label {
				found = index
				break
			}
		}
		if found < 0 {
			t.Fatalf("%q is missing from 账户; texts: %v", label, texts)
		}
		if found <= last {
			t.Fatalf("%q is out of order; texts: %v", label, texts)
		}
		last = found
	}

	if !tester.HasText(locales.StorePassword()) {
		t.Fatalf("保存密码 is missing from the password row; texts: %v", texts)
	}
	for _, gone := range []string{locales.NameField(), "端口"} {
		if tester.HasText(gone) {
			t.Fatalf("账户 still shows %q; texts: %v", gone, texts)
		}
	}
}

// 保存密码 edits the profile's own flag, which a new profile starts with set.
func TestAccountPasswordRowFollowsTheProfile(t *testing.T) {
	state := NewSettingsState(config.Default(), "/tmp/logs")
	if !state.Draft.Profiles[0].Auth.StorePass {
		t.Fatal("a new profile does not store its password by default")
	}
	state.Draft.Profiles[0].Auth.StorePass = false

	_, tester := newShell(t, state, SettingsActions{}, PageAccount)
	if err := tester.Click(locales.StorePassword()); err != nil {
		t.Fatalf("click 保存密码: %v", err)
	}
	if !state.Draft.Profiles[0].Auth.StorePass {
		t.Fatal("the check box did not write the profile's flag")
	}
}
