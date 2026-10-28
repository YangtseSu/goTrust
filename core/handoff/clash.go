// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package handoff

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// ErrNoProxy reports a snapshot whose proxy listeners are both disabled.
var ErrNoProxy = errors.New("no proxy listener is configured")

// Clash renders a fragment for a Clash/Mihomo configuration: one proxy entry,
// a selector group and the rules in first-match order. The fragment assumes
// the configuration's own fallback (usually DIRECT) and deliberately adds no
// private-range preset: a grant may be public, and a grant may sit inside a
// private range.
//
// The first rules keep goTrust's own traffic out of the tunnel. A TUN
// configuration captures the process's TLS to the gateway as well, and sending
// that back through the proxy would either loop or add a pointless hop.
// Clash returns the plain fragment a Mihomo/Clash configuration consumes: the
// lists are called proxies, proxy-groups and rules.
func (s Snapshot) Clash() (string, error) { return s.clashFragment(false) }

// ClashMerge returns the same content under the prepend-* keys Clash Verge
// Rev's Merge profile uses, so the profile can be pasted without editing.
func (s Snapshot) ClashMerge() (string, error) { return s.clashFragment(true) }

// ClashConfig returns a complete standalone Mihomo configuration. Any client
// can import it as a local profile; it carries goTrust only and assumes the
// client manages ports, DNS and TUN.
func (s Snapshot) ClashConfig() (string, error) {
	body, err := s.clashBody(false)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# goTrust standalone Mihomo config, generated %s\n", s.Generated.UTC().Format(time.RFC3339))
	if s.Profile != "" {
		fmt.Fprintf(&b, "# profile: %s\n", s.Profile)
	}
	if s.Gateway != "" {
		fmt.Fprintf(&b, "# gateway: %s\n", s.Gateway)
	}
	b.WriteString("# Import as a local profile. It carries goTrust only; a subscription\n")
	b.WriteString("# profile needs its own merge mechanism.\n")
	b.WriteString("mixed-port: 7890\nallow-lan: false\nmode: rule\nlog-level: info\n\n")
	b.WriteString(body)
	b.WriteString("  - MATCH,DIRECT\n")
	return b.String(), nil
}

func (s Snapshot) clashFragment(prepend bool) (string, error) {
	body, err := s.clashBody(prepend)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# goTrust Clash/Mihomo fragment, generated %s\n", s.Generated.UTC().Format(time.RFC3339))
	if s.Profile != "" {
		fmt.Fprintf(&b, "# profile: %s\n", s.Profile)
	}
	if s.Gateway != "" {
		fmt.Fprintf(&b, "# gateway: %s\n", s.Gateway)
	}
	if prepend {
		b.WriteString("# A Clash Verge Rev Merge profile; paste it as-is and enable it.\n")
	} else {
		b.WriteString("# Paste under proxies / proxy-groups / rules. Keep your own fallback.\n")
	}
	b.WriteString(body)
	return b.String(), nil
}

func (s Snapshot) clashBody(prepend bool) (string, error) {
	listen, proxyType := s.SOCKS5, "socks5"
	if listen == "" {
		listen, proxyType = s.HTTP, "http"
	}
	if listen == "" {
		return "", ErrNoProxy
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("proxy listener %q: %w", listen, err)
	}

	proxiesKey, groupsKey, rulesKey := "proxies", "proxy-groups", "rules"
	if prepend {
		proxiesKey, groupsKey, rulesKey = "prepend-proxies", "prepend-proxy-groups", "prepend-rules"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n", proxiesKey)
	b.WriteString("  - name: goTrust\n")
	fmt.Fprintf(&b, "    type: %s\n", proxyType)
	fmt.Fprintf(&b, "    server: %s\n", scalar(host))
	fmt.Fprintf(&b, "    port: %s\n", port)
	if proxyType == "socks5" {
		b.WriteString("    udp: true\n")
	}

	fmt.Fprintf(&b, "%s:\n", groupsKey)
	b.WriteString("  - name: goTrust-routes\n")
	b.WriteString("    type: select\n")
	b.WriteString("    proxies:\n")
	b.WriteString("      - goTrust\n")
	b.WriteString("      - DIRECT\n")

	fmt.Fprintf(&b, "%s:\n", rulesKey)
	for _, process := range []string{"gotrust", "gotrust-cli", "gotrust.exe", "gotrust-cli.exe"} {
		fmt.Fprintf(&b, "  - PROCESS-NAME,%s,DIRECT\n", process)
	}
	if rule := gatewayRule(s.gatewayHost()); rule != "" {
		fmt.Fprintf(&b, "  - %s\n", rule)
	}
	for _, domain := range s.Domains {
		fmt.Fprintf(&b, "  - DOMAIN-SUFFIX,%s,goTrust-routes\n", domain)
	}
	for _, prefix := range s.Prefixes {
		fmt.Fprintf(&b, "  - IP-CIDR,%s,goTrust-routes,no-resolve\n", prefix)
	}
	return b.String(), nil
}

func gatewayRule(host string) string {
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" {
		return ""
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.Is4() {
			return "IP-CIDR," + addr.String() + "/32,DIRECT,no-resolve"
		}
		return "IP-CIDR6," + addr.String() + "/128,DIRECT,no-resolve"
	}
	return "DOMAIN," + host + ",DIRECT"
}

// scalar quotes a host only when plain YAML would misread it (an IPv6 literal
// contains a colon). Hostnames and IPv4 addresses stay readable.
func scalar(value string) string {
	if strings.ContainsAny(value, ":#{}[]&*!|>'\"%@`") {
		return strconv.Quote(value)
	}
	return value
}
