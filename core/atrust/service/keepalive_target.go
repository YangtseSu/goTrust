package service

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/YangtseSu/goTrust/core/atrust/client"
)

const (
	// maxDerivedCandidates bounds one discovery round. A derived candidate is a
	// guess, so it is probed on a short leash and only a handful of them.
	maxDerivedCandidates = 4
	// The well-known ports a grant is probed on, in preference order. Port 80
	// answers a plain HTTP request, 443 completes a TLS handshake and 22 sends
	// its SSH identification; any other port gets a connection only.
	derivedHTTPPort = 80
	derivedTLSPort  = 443
	derivedSSHPort  = 22
)

// NamedKeepAliveTargets validates the comma-separated list of configured
// keep-alive targets. A named target is what a human chose, so it is probed
// with a generous timeout.
func NamedKeepAliveTargets(value string) ([]KeepAliveTarget, []string) {
	var targets []KeepAliveTarget
	var warnings []string
	seen := make(map[string]bool)

	for _, raw := range strings.Split(value, ",") {
		target := strings.TrimSpace(raw)
		if target == "" {
			continue
		}

		parsed, err := url.Parse(target)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			warnings = append(warnings, fmt.Sprintf(
				"keep-alive target %q is not a http:// or https:// URL and is ignored", raw))
			continue
		}
		if parsed.Scheme == "https" {
			warnings = append(warnings, fmt.Sprintf(
				"keep-alive target %s: the probe verifies the certificate, so an intranet certificate "+
					"from a private CA fails; prefer a plain http:// address", target))
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		targets = append(targets, KeepAliveTarget{URL: target, Named: true})
	}

	return targets, warnings
}

// DeriveKeepAliveTargets turns the granted resources into probe candidates. It
// never invents an address and only uses ports the grant covers: 80 is probed
// with a plain HTTP request, 443 with a TLS handshake, 22 with the SSH
// identification, and any other lowest port with a connection through the
// tunnel.
func DeriveKeepAliveTargets(ipResources []client.IPResource) ([]KeepAliveTarget, []string) {
	var targets []KeepAliveTarget
	var notes []string
	seen := make(map[string]bool)

	for _, resource := range ipResources {
		if len(targets) >= maxDerivedCandidates {
			break
		}
		if !resourceForwardsProtocol(resource) {
			continue
		}
		address, ok := resourceAddress(resource)
		if !ok {
			// IPv6 destinations are dialed directly, so they prove nothing.
			continue
		}
		if maximum, ok := resourceMaximumAddress(resource); ok && maximum != address {
			notes = append(notes, fmt.Sprintf(
				"resource %s-%s covers a range, probing %s only", address, maximum, address))
		}

		for _, port := range resourceProbePorts(resource) {
			if len(targets) >= maxDerivedCandidates {
				break
			}
			target := derivedTarget(address, port)
			if seen[target.String()] {
				continue
			}
			seen[target.String()] = true
			targets = append(targets, target)
		}
	}

	if len(targets) == 0 {
		notes = append(notes, "no TCP application target could be derived from the granted resources")
	}

	return targets, notes
}

// resourceForwardsProtocol reports whether the tunnel forwards a resource's
// protocol at all.
func resourceForwardsProtocol(resource client.IPResource) bool {
	return resource.Protocol == "tcp" || resource.Protocol == "all"
}

// resourceAddress returns the address a resource is probed at.
func resourceAddress(resource client.IPResource) (netip.Addr, bool) {
	address, ok := netip.AddrFromSlice(resource.IPMin)
	if !ok {
		return netip.Addr{}, false
	}
	address = address.Unmap()
	if address.Is6() || !address.Is4() {
		return netip.Addr{}, false
	}
	return address, true
}

// resourceMaximumAddress returns the last address of a resource, which is the
// first one again for a single address.
func resourceMaximumAddress(resource client.IPResource) (netip.Addr, bool) {
	maximum := resource.IPMax
	if maximum == nil {
		maximum = resource.IPMin
	}
	address, ok := netip.AddrFromSlice(maximum)
	if !ok {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

// resourceProbePorts returns the granted ports worth probing, in preference
// order. A grant that covers no well-known port is still worth one connection
// at its lowest port.
func resourceProbePorts(resource client.IPResource) []int {
	var ports []int
	for _, port := range []int{derivedHTTPPort, derivedTLSPort, derivedSSHPort} {
		if resource.PortMin <= port && port <= resource.PortMax {
			ports = append(ports, port)
		}
	}
	if len(ports) == 0 && resource.PortMin > 0 && resource.PortMin <= resource.PortMax {
		ports = append(ports, resource.PortMin)
	}
	return ports
}

// derivedTarget names the probe for one granted address and port. Only port 80
// is an HTTP probe; the rest are TCP probes with the exchange their port
// implies.
func derivedTarget(address netip.Addr, port int) KeepAliveTarget {
	if port == derivedHTTPPort {
		return KeepAliveTarget{URL: fmt.Sprintf("http://%s/", address)}
	}
	return KeepAliveTarget{Address: net.JoinHostPort(address.String(), strconv.Itoa(port))}
}

// keepAliveOrder puts the last target that answered first, then the named
// targets in the order the user gave them, then the derived candidates.
func keepAliveOrder(targets []KeepAliveTarget, active string) []KeepAliveTarget {
	ordered := make([]KeepAliveTarget, 0, len(targets))
	if active != "" {
		for _, target := range targets {
			if target.String() == active {
				ordered = append(ordered, target)
			}
		}
	}
	for _, named := range []bool{true, false} {
		for _, target := range targets {
			if target.Named != named || target.String() == active {
				continue
			}
			ordered = append(ordered, target)
		}
	}
	return ordered
}
