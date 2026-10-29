// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"net"
	"strconv"
	"strings"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/locales"
)

// ProfileProblem names the first field a login needs and the profile does not
// have. An empty result means the profile can be used for a login.
//
// config.Profile.Validate is the authority and speaks English, because
// core/config is a library boundary; the window shows this sentence instead,
// and the connect path refuses with it. TestProfileProblemMatchesValidate keeps
// the two lists identical.
func ProfileProblem(profile config.Profile) string {
	switch {
	case profile.ServerAddress() == "":
		return locales.ProblemAddress()
	case profile.Server.Port <= 0 || profile.Server.Port > 65535:
		return locales.ProblemPort()
	case profile.Auth.Username == "":
		return locales.ProblemUsername()
	default:
		return ""
	}
}

// defaultGatewayPort is the port a gateway address without one uses: aTrust
// gateways listen on 443, and the settings file has always defaulted to it.
const defaultGatewayPort = 443

// ParseGateway reads the account page's one gateway field. It accepts what an
// administrator or a browser hands over: host:port, https://host:port, a bare
// host (port 443), a trailing slash, and IPv6 in brackets. Any other scheme is
// refused rather than stored as part of the host. An empty text is an account
// without a gateway yet: the address is empty and the port is zero, so a
// caller keeps the port the profile already stores.
func ParseGateway(text string) (address string, port int, problem string) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return "", 0, ""
	}
	if scheme, rest, found := strings.Cut(raw, "://"); found {
		if !strings.EqualFold(scheme, "https") {
			return "", 0, locales.ProblemGatewayScheme()
		}
		raw = rest
	}
	// A path or a trailing slash is a paste artefact, not part of the gateway.
	if cut := strings.IndexByte(raw, '/'); cut >= 0 {
		raw = raw[:cut]
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, locales.ProblemAddress()
	}

	var host, portText string
	hasPort := false
	switch {
	case strings.HasPrefix(raw, "["):
		end := strings.IndexByte(raw, ']')
		if end < 0 {
			return "", 0, locales.ProblemAddress()
		}
		host = raw[1:end]
		if rest := raw[end+1:]; rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return "", 0, locales.ProblemPort()
			}
			portText, hasPort = rest[1:], true
		}
	case strings.Count(raw, ":") == 0:
		host = raw
	case strings.Count(raw, ":") == 1:
		host, portText, _ = strings.Cut(raw, ":")
		hasPort = true
	default:
		// Several colons without brackets are an IPv6 address typed without a
		// port; with a port it would have been ambiguous, so it has to parse
		// as one or the port is the part that is wrong.
		if net.ParseIP(raw) == nil {
			return "", 0, locales.ProblemPort()
		}
		host = raw
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", 0, locales.ProblemAddress()
	}
	if !hasPort {
		return host, defaultGatewayPort, ""
	}
	number, err := strconv.Atoi(strings.TrimSpace(portText))
	if err != nil || number < 1 || number > 65535 {
		return "", 0, locales.ProblemPort()
	}
	return host, number, ""
}

// gatewayText is the field text for a profile: the address and port as one
// host:port, with IPv6 bracketed so the same text parses back. A profile
// without a gateway has an empty field.
func gatewayText(profile config.Profile) string {
	address := strings.TrimSpace(profile.Server.Address)
	if address == "" {
		return ""
	}
	port := profile.Server.Port
	if port <= 0 {
		port = defaultGatewayPort
	}
	if strings.Contains(address, ":") && !strings.HasPrefix(address, "[") && net.ParseIP(address) != nil {
		address = "[" + address + "]"
	}
	return address + ":" + strconv.Itoa(port)
}
