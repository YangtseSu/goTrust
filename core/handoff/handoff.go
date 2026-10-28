// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package handoff turns one session's grant into the artifacts an external
// program can consume: a PAC script for browsers and a rule fragment for the
// Clash/Mihomo family. It holds no state and opens no socket; the front-ends
// own the endpoint that serves the live PAC.
//
// The grant is the authority. A rendered rule only says where a destination
// should go; the session's dialer still decides whether a destination enters
// the tunnel, and dials everything else directly.
package handoff

import (
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
)

// Snapshot is what one grant means to an external program: the proxy
// endpoints, the destinations that belong to the tunnel, and enough context
// for a header. It is a value, so a renderer cannot change the session.
type Snapshot struct {
	Generated time.Time
	Profile   string
	Gateway   string
	TunnelIP  string
	// Online is false while no data path can be used; the PAC then answers
	// DIRECT for everything.
	Online   bool
	SOCKS5   string
	HTTP     string
	Prefixes []netip.Prefix
	Domains  []string
}

// FromState builds a snapshot from the live session. A reconnecting session
// may still hold a usable data path, so it keeps the grant routed; a REFUSED
// dial is safer than leaking an intranet destination around the tunnel.
func FromState(cfg config.Config, state session.State) Snapshot {
	gateway := gatewayOf(cfg)
	// Detail carries the gateway address only while online; during a login it
	// is a progress line, and during a reconnect it is the reason.
	if state.Phase == session.PhaseOnline {
		if detail := strings.TrimSpace(state.Detail); detail != "" {
			if _, _, err := net.SplitHostPort(detail); err == nil {
				gateway = detail
			}
		}
	}
	online := state.Phase == session.PhaseOnline || state.Phase == session.PhaseReconnecting
	return build(cfg, cfg.ActiveProfile, gateway, state.ClientIP, online, state.Resources)
}

// FromCache builds a snapshot from the grant cached by the last login. It is
// used by the one-shot CLI commands, which have no live session and only need
// the rules.
func FromCache(cfg config.Config, cached session.ResourceSnapshot) Snapshot {
	gateway := strings.TrimSpace(cached.Gateway)
	if gateway == "" {
		gateway = gatewayOf(cfg)
	}
	profile := cached.Profile
	if profile == "" {
		profile = cfg.ActiveProfile
	}
	return build(cfg, profile, gateway, cached.TunnelIP, true, cached.Resources)
}

func build(cfg config.Config, profile, gateway, tunnelIP string, online bool, resources session.Resources) Snapshot {
	socks5, httpBind := cfg.ProxyAddresses()
	return Snapshot{
		Generated: time.Now(),
		Profile:   profile,
		Gateway:   gateway,
		TunnelIP:  tunnelIP,
		Online:    online,
		SOCKS5:    strings.TrimSpace(socks5),
		HTTP:      strings.TrimSpace(httpBind),
		Prefixes:  grantedPrefixes(resources.IPs),
		Domains:   grantedDomains(resources),
	}
}

// gatewayHost returns the host part of the snapshot's gateway address.
func (s Snapshot) gatewayHost() string {
	host, _, err := net.SplitHostPort(s.Gateway)
	if err != nil {
		return strings.TrimSpace(s.Gateway)
	}
	return host
}

func gatewayOf(cfg config.Config) string {
	profile := cfg.Active()
	host := strings.TrimSpace(profile.ServerAddress())
	if host == "" {
		return ""
	}
	port := profile.Server.Port
	if port == 0 {
		port = 443
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// grantedDomains returns the label-suffix set an external program may route:
// the names the gateway granted explicitly and the names it resolved itself.
// App.WebDomains is a portal launch hint, not a grant, and is never included.
func grantedDomains(resources session.Resources) []string {
	seen := make(map[string]struct{}, len(resources.Domains)+len(resources.DNS))
	for _, rule := range resources.Domains {
		addDomain(seen, rule.Name)
	}
	for _, rule := range resources.DNS {
		addDomain(seen, rule.Name)
	}
	domains := make([]string, 0, len(seen))
	for domain := range seen {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	return domains
}

func addDomain(seen map[string]struct{}, name string) {
	if domain := normalizeDomain(name); domain != "" {
		seen[domain] = struct{}{}
	}
}

// normalizeDomain turns a grant name into a suffix. A wildcard becomes a bare
// suffix because matching is done on label boundaries, and an address is not a
// name.
func normalizeDomain(name string) string {
	domain := strings.ToLower(strings.TrimSpace(name))
	domain = strings.TrimSuffix(domain, ".")
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimPrefix(domain, ".")
	if domain == "" || strings.ContainsAny(domain, " /\\*") {
		return ""
	}
	if _, err := netip.ParseAddr(domain); err == nil {
		return ""
	}
	return domain
}
