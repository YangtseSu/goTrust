// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package handoff

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
)

func testConfig() config.Config {
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	cfg.Profiles[0].Server.Port = 443
	return cfg
}

func testResources() session.Resources {
	return session.Resources{
		IPs: []session.IPRule{
			{From: "192.0.2.0", To: "192.0.2.255", PortMin: 80, PortMax: 80, Protocol: "tcp"},
			{From: "198.51.100.1", To: "198.51.100.1", PortMin: 443, PortMax: 443, Protocol: "tcp"},
		},
		Domains: []session.DomainRule{
			{Name: "*.Corp.Example.", PortMin: 443, PortMax: 443, Protocol: "tcp"},
			{Name: "corp.example", PortMin: 80, PortMax: 80, Protocol: "tcp"},
		},
		DNS: []session.DNSRule{
			{Name: "app.corp.example", IPs: []string{"192.0.2.10"}},
			{Name: "10.0.0.1"},
		},
		Apps: []session.App{{
			ID:         "app",
			WebDomains: []string{"portal.example"},
		}},
	}
}

func TestGrantedPrefixesMergeAndSplit(t *testing.T) {
	rules := []session.IPRule{
		{From: "192.0.2.0", To: "192.0.2.127"},
		{From: "192.0.2.128", To: "192.0.2.255"},
		{From: "203.0.113.4", To: "203.0.113.0"},   // reversed
		{From: "203.0.113.0", To: "203.0.113.255"}, // duplicate and wider
		{From: "198.51.100.1", To: ""},
		{From: "not-an-ip", To: "192.0.2.1"},
		{From: "2001:db8::1", To: "2001:db8::2"},
	}
	got := grantedPrefixes(rules)
	want := []string{
		"192.0.2.0/24",
		"198.51.100.1/32",
		"203.0.113.0/24",
	}
	// Sorted by address, not by rule order.
	if len(got) != len(want) {
		t.Fatalf("prefixes = %v, want %v", got, want)
	}
	for index, prefix := range want {
		if got[index] != netip.MustParsePrefix(prefix) {
			t.Fatalf("prefix %d = %s, want %s (all %v)", index, got[index], prefix, got)
		}
	}
}

func TestGrantedPrefixesFullRange(t *testing.T) {
	got := grantedPrefixes([]session.IPRule{{From: "0.0.0.0", To: "255.255.255.255"}})
	if len(got) != 1 || got[0].String() != "0.0.0.0/0" {
		t.Fatalf("full range = %v, want 0.0.0.0/0", got)
	}
}

func TestGrantedDomains(t *testing.T) {
	got := grantedDomains(testResources())
	want := []string{"app.corp.example", "corp.example"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("domains = %v, want %v", got, want)
	}
	for _, domain := range got {
		if domain == "portal.example" {
			t.Fatal("App.WebDomains must never become a routing rule")
		}
		if domain == "10.0.0.1" {
			t.Fatal("an address must not become a domain suffix")
		}
	}
}

func TestFromStateBuildsTheGrant(t *testing.T) {
	cfg := testConfig()
	state := session.State{
		Phase:     session.PhaseOnline,
		Detail:    "vpn.example.com:443",
		ClientIP:  "192.0.2.99",
		Resources: testResources(),
	}
	snapshot := FromState(cfg, state)
	if !snapshot.Online || snapshot.SOCKS5 != "127.0.0.1:1080" || snapshot.HTTP != "127.0.0.1:1081" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if snapshot.Profile != config.DefaultProfileName || snapshot.Gateway != "vpn.example.com:443" {
		t.Fatalf("identity = %q / %q", snapshot.Profile, snapshot.Gateway)
	}
	if len(snapshot.Prefixes) != 2 || len(snapshot.Domains) != 2 {
		t.Fatalf("grant = %d prefixes / %d domains", len(snapshot.Prefixes), len(snapshot.Domains))
	}
}

func TestFromStateOffline(t *testing.T) {
	cfg := testConfig()
	snapshot := FromState(cfg, session.State{Phase: session.PhaseIdle})
	if snapshot.Online {
		t.Fatal("an idle session must render as offline")
	}
}

func TestPACOnline(t *testing.T) {
	cfg := testConfig()
	snapshot := FromState(cfg, session.State{Phase: session.PhaseOnline, Resources: testResources()})
	snapshot.Generated = time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	snapshot.Gateway = "vpn.example.com:443"

	pac := snapshot.PAC()
	for _, want := range []string{
		"PROXY 127.0.0.1:1081",
		`isInNet(host, "192.0.2.0", "255.255.255.0")`,
		`isInNet(host, "198.51.100.1", "255.255.255.255")`,
		`dnsDomainIs(host, ".corp.example")`,
		`host === "vpn.example.com"`,
		"PAC cannot carry UDP",
	} {
		if !strings.Contains(pac, want) {
			t.Fatalf("PAC is missing %q:\n%s", want, pac)
		}
	}
	if strings.Contains(pac, "DIRECT\";\n}\n") && !strings.Contains(pac, "isInNet") {
		t.Fatal("an online PAC must not be direct-only")
	}
}

func TestPACOfflineIsDirect(t *testing.T) {
	cfg := testConfig()
	snapshot := FromState(cfg, session.State{Phase: session.PhaseFailed, Resources: testResources()})
	pac := snapshot.PAC()
	if strings.Contains(pac, "isInNet") || strings.Contains(pac, "goTrustProxy") {
		t.Fatalf("an offline PAC must be direct-only:\n%s", pac)
	}
	if !strings.Contains(pac, "return \"DIRECT\";") {
		t.Fatalf("offline PAC = %s", pac)
	}
}

func TestPACFallsBackToSOCKS5(t *testing.T) {
	cfg := testConfig()
	cfg.Proxy.HTTPBind = ""
	snapshot := FromState(cfg, session.State{Phase: session.PhaseOnline, Resources: testResources()})
	if !strings.Contains(snapshot.PAC(), "SOCKS5 127.0.0.1:1080") {
		t.Fatalf("PAC did not fall back to SOCKS5:\n%s", snapshot.PAC())
	}
}

func TestClashFragmentOrder(t *testing.T) {
	cfg := testConfig()
	snapshot := FromState(cfg, session.State{Phase: session.PhaseOnline, Resources: testResources()})
	fragment, err := snapshot.Clash()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"type: socks5",
		"udp: true",
		"- name: goTrust-routes",
		"- PROCESS-NAME,gotrust,DIRECT",
		"- PROCESS-NAME,gotrust-cli,DIRECT",
		"- DOMAIN,vpn.example.com,DIRECT",
		"- DOMAIN-SUFFIX,corp.example,goTrust-routes",
		"- IP-CIDR,192.0.2.0/24,goTrust-routes,no-resolve",
	} {
		if !strings.Contains(fragment, want) {
			t.Fatalf("fragment is missing %q:\n%s", want, fragment)
		}
	}
	index := func(text string) int { return strings.Index(fragment, text) }
	if !(index("PROCESS-NAME") < index("DOMAIN,vpn.example.com") &&
		index("DOMAIN,vpn.example.com") < index("DOMAIN-SUFFIX") &&
		index("DOMAIN-SUFFIX") < index("IP-CIDR")) {
		t.Fatalf("rule order is wrong:\n%s", fragment)
	}
	if strings.Contains(fragment, "IP-CIDR,10.0.0.0/8") {
		t.Fatal("the fragment must not add a private-range preset")
	}
}

func TestClashHTTPOnly(t *testing.T) {
	cfg := testConfig()
	cfg.Proxy.SOCKS5Bind = ""
	snapshot := FromState(cfg, session.State{Phase: session.PhaseOnline})
	fragment, err := snapshot.Clash()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fragment, "type: http") || strings.Contains(fragment, "udp: true") {
		t.Fatalf("HTTP-only fragment:\n%s", fragment)
	}
}

func TestClashWithoutListenersFails(t *testing.T) {
	cfg := testConfig()
	cfg.Proxy.SOCKS5Bind = ""
	cfg.Proxy.HTTPBind = ""
	snapshot := FromState(cfg, session.State{Phase: session.PhaseOnline})
	if _, err := snapshot.Clash(); !errors.Is(err, ErrNoProxy) {
		t.Fatalf("error = %v, want ErrNoProxy", err)
	}
}

func TestDomainNormalisation(t *testing.T) {
	cases := map[string]string{
		"*.Corp.Example.": "corp.example",
		".corp.example":   "corp.example",
		"  host.Example ": "host.example",
		"10.0.0.1":        "",
		"":                "",
		"*":               "",
	}
	for input, want := range cases {
		if got := normalizeDomain(input); got != want {
			t.Errorf("normalizeDomain(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFromStateIgnoresProgressDetail(t *testing.T) {
	cfg := testConfig()
	snapshot := FromState(cfg, session.State{
		Phase:  session.PhaseConnecting,
		Detail: "discovering the authentication methods",
	})
	if snapshot.Gateway != "vpn.example.com:443" {
		t.Fatalf("gateway = %q, want the configured address", snapshot.Gateway)
	}
	if snapshot.Online {
		t.Fatal("a connecting session must not be rendered as online")
	}
}
