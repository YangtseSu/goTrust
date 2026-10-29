// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package handoff

import (
	"encoding/json"
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

// Clash returns the plain fragment a Mihomo/Clash configuration consumes: the
// lists are called proxies, proxy-groups and rules. It is meant to be merged
// into an existing configuration, before its MATCH rule.
func (s Snapshot) Clash() (string, error) {
	body, err := s.clashBody()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	s.writeHeader(&b, "Clash/Mihomo fragment", "# ")
	b.WriteString("# Paste under proxies / proxy-groups / rules. Keep your own fallback.\n")
	b.WriteString(body)
	return b.String(), nil
}

// ClashConfig returns a complete standalone Mihomo configuration. Any client
// can import it as a local profile; it carries goTrust only and assumes the
// client manages ports, DNS and TUN.
func (s Snapshot) ClashConfig() (string, error) {
	body, err := s.clashBody()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	s.writeHeader(&b, "standalone Mihomo config", "# ")
	b.WriteString("# Import as a local profile. It carries goTrust only; a subscription\n")
	b.WriteString("# profile needs its own merge mechanism.\n")
	b.WriteString("mixed-port: 7890\nallow-lan: false\nmode: rule\nlog-level: info\n\n")
	b.WriteString(body)
	b.WriteString("  - MATCH,DIRECT\n")
	return b.String(), nil
}

// ClashScript returns a Clash Verge Rev Script. Verge's Merge config is a deep
// merge that replaces lists, so a fragment cannot be appended there; the
// Script enhancement receives the whole configuration and prepends the proxy,
// the group and the rules without losing the subscription's own entries.
func (s Snapshot) ClashScript() (string, error) {
	proxyType, host, port, udp, err := s.clashProxy()
	if err != nil {
		return "", err
	}
	proxy := map[string]any{
		"name":   "goTrust",
		"type":   proxyType,
		"server": host,
		"port":   port,
	}
	if udp {
		proxy["udp"] = true
	}
	group := map[string]any{
		"name":    "goTrust-routes",
		"type":    "select",
		"proxies": []string{"goTrust", "DIRECT"},
	}
	proxyJSON, err := json.Marshal(proxy)
	if err != nil {
		return "", err
	}
	groupJSON, err := json.Marshal(group)
	if err != nil {
		return "", err
	}
	rulesJSON, err := json.Marshal(s.clashRules())
	if err != nil {
		return "", err
	}

	var b strings.Builder
	s.writeHeader(&b, "Clash Verge Rev script", "// ")
	b.WriteString("// Paste into Profiles → Global Extend Script (Script), save and enable it.\n")
	b.WriteString("function main(config, profileName) {\n")
	fmt.Fprintf(&b, "  var proxy = %s;\n", proxyJSON)
	fmt.Fprintf(&b, "  var group = %s;\n", groupJSON)
	fmt.Fprintf(&b, "  var trustRules = %s;\n", rulesJSON)
	b.WriteString("  config.proxies = (config.proxies || []).filter(function (item) { return item.name !== proxy.name; });\n")
	b.WriteString("  config.proxies.unshift(proxy);\n")
	b.WriteString("  config[\"proxy-groups\"] = (config[\"proxy-groups\"] || []).filter(function (item) { return item.name !== group.name; });\n")
	b.WriteString("  config[\"proxy-groups\"].unshift(group);\n")
	b.WriteString("  config.rules = (config.rules || []).filter(function (rule) { return trustRules.indexOf(rule) < 0; });\n")
	b.WriteString("  config.rules = trustRules.concat(config.rules);\n")
	b.WriteString("  return config;\n}\n")
	return b.String(), nil
}

func (s Snapshot) writeHeader(b *strings.Builder, kind, comment string) {
	fmt.Fprintf(b, "%sgoTrust %s, generated %s\n", comment, kind, s.Generated.UTC().Format(time.RFC3339))
	if s.Profile != "" {
		fmt.Fprintf(b, "%sprofile: %s\n", comment, s.Profile)
	}
	if s.Gateway != "" {
		fmt.Fprintf(b, "%sgateway: %s\n", comment, s.Gateway)
	}
}

func (s Snapshot) clashProxy() (proxyType, host string, port int, udp bool, err error) {
	listen, proxyType := s.SOCKS5, "socks5"
	if listen == "" {
		listen, proxyType = s.HTTP, "http"
	}
	if listen == "" {
		return "", "", 0, false, ErrNoProxy
	}
	host, portText, err := net.SplitHostPort(listen)
	if err != nil {
		return "", "", 0, false, fmt.Errorf("proxy listener %q: %w", listen, err)
	}
	number, err := strconv.Atoi(portText)
	if err != nil {
		return "", "", 0, false, fmt.Errorf("proxy listener %q: %w", listen, err)
	}
	return proxyType, host, number, proxyType == "socks5", nil
}

// clashRules is the ordered rule set every rendering shares: goTrust's own
// traffic and the gateway stay direct, then the granted suffixes and prefixes.
func (s Snapshot) clashRules() []string {
	rules := make([]string, 0, 4+len(s.Domains)+len(s.Prefixes)+1)
	for _, process := range []string{"gotrust", "gotrust-cli", "gotrust.exe", "gotrust-cli.exe"} {
		rules = append(rules, "PROCESS-NAME,"+process+",DIRECT")
	}
	if rule := gatewayRule(s.gatewayHost()); rule != "" {
		rules = append(rules, rule)
	}
	for _, domain := range s.Domains {
		rules = append(rules, "DOMAIN-SUFFIX,"+domain+",goTrust-routes")
	}
	for _, prefix := range s.Prefixes {
		rules = append(rules, "IP-CIDR,"+prefix.String()+",goTrust-routes,no-resolve")
	}
	return rules
}

func (s Snapshot) clashBody() (string, error) {
	proxyType, host, port, udp, err := s.clashProxy()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("proxies:\n")
	b.WriteString("  - name: goTrust\n")
	fmt.Fprintf(&b, "    type: %s\n", proxyType)
	fmt.Fprintf(&b, "    server: %s\n", scalar(host))
	fmt.Fprintf(&b, "    port: %d\n", port)
	if udp {
		b.WriteString("    udp: true\n")
	}

	b.WriteString("proxy-groups:\n")
	b.WriteString("  - name: goTrust-routes\n")
	b.WriteString("    type: select\n")
	b.WriteString("    proxies:\n")
	b.WriteString("      - goTrust\n")
	b.WriteString("      - DIRECT\n")

	b.WriteString("rules:\n")
	for _, rule := range s.clashRules() {
		fmt.Fprintf(&b, "  - %s\n", rule)
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
