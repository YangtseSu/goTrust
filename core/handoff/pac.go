// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package handoff

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// PAC renders the snapshot as a proxy auto-config script. Only literal IPv4
// destinations are tested against a prefix: isInNet on a hostname would ask
// the browser's resolver, which is exactly the resolver an intranet name may
// not reach. Names are matched as suffixes instead and travel to the proxy,
// where the session resolver answers them inside the tunnel.
//
// PAC has no UDP notion, so a UDP destination of a proxied host bypasses the
// proxy. The script says so in its header.
func (s Snapshot) PAC() string {
	proxy := s.directive()
	var b strings.Builder
	fmt.Fprintf(&b, "// goTrust PAC, generated %s\n", s.Generated.UTC().Format(time.RFC3339))
	if s.Profile != "" {
		fmt.Fprintf(&b, "// profile: %s\n", s.Profile)
	}
	if s.Gateway != "" {
		fmt.Fprintf(&b, "// gateway: %s\n", s.Gateway)
	}
	b.WriteString("// Only the granted destinations are proxied; everything else is DIRECT.\n")
	b.WriteString("// PAC cannot carry UDP; QUIC and WebRTC bypass the proxy.\n")

	if proxy == "" || !s.Online {
		b.WriteString("function FindProxyForURL(url, host) {\n")
		b.WriteString("  return \"DIRECT\";\n")
		b.WriteString("}\n")
		return b.String()
	}

	fmt.Fprintf(&b, "var goTrustProxy = %s;\n", strconv.Quote(proxy))
	b.WriteString("function FindProxyForURL(url, host) {\n")
	b.WriteString("  host = host.toLowerCase();\n")
	b.WriteString("  if (host === \"localhost\" || host === \"127.0.0.1\" || host === \"::1\") return \"DIRECT\";\n")
	if gateway := strings.ToLower(s.gatewayHost()); gateway != "" && gateway != "0.0.0.0" {
		fmt.Fprintf(&b, "  if (host === %s) return \"DIRECT\";\n", strconv.Quote(gateway))
	}
	if len(s.Prefixes) > 0 {
		b.WriteString("  if (/^(\\d{1,3}\\.){3}\\d{1,3}$/.test(host)) {\n")
		for _, prefix := range s.Prefixes {
			network, mask := prefixNetMask(prefix)
			fmt.Fprintf(&b, "    if (isInNet(host, %s, %s)) return goTrustProxy;\n",
				strconv.Quote(network), strconv.Quote(mask))
		}
		b.WriteString("    return \"DIRECT\";\n")
		b.WriteString("  }\n")
	}
	for _, domain := range s.Domains {
		fmt.Fprintf(&b, "  if (host === %s || dnsDomainIs(host, %s)) return goTrustProxy;\n",
			strconv.Quote(domain), strconv.Quote("."+domain))
	}
	b.WriteString("  return \"DIRECT\";\n")
	b.WriteString("}\n")
	return b.String()
}

// directive is the PAC result for a granted destination. The HTTP proxy is
// preferred because CONNECT carries the hostname; SOCKS5 is the fallback when
// only that listener is enabled.
func (s Snapshot) directive() string {
	if s.HTTP != "" {
		return "PROXY " + s.HTTP
	}
	if s.SOCKS5 != "" {
		return "SOCKS5 " + s.SOCKS5
	}
	return ""
}

func prefixNetMask(prefix netip.Prefix) (string, string) {
	prefix = prefix.Masked()
	mask := net.CIDRMask(prefix.Bits(), 32)
	return prefix.Addr().String(), net.IP(mask).String()
}
