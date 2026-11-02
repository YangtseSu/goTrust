// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"net"
	"os"
	"path/filepath"
	"strings"
)

// virtualNetDir holds one entry per network interface the kernel created
// itself: bridges, bonds, vlans, tunnels, veth pairs and container links all
// appear here, while a card with a vendor address does not.
var virtualNetDir = "/sys/devices/virtual/net"

// reportedMACAddresses returns what reportEnv carries for an account. A value
// the user set is reported exactly as written, so a gateway that already
// recorded an endpoint this host cannot present still recognizes the machine;
// otherwise the machine's own cards are reported.
func reportedMACAddresses(configured []string) []string {
	if kept := nonEmpty(configured); len(kept) > 0 {
		return kept
	}
	return hostMACAddresses()
}

// nonEmpty drops blank entries without changing the rest: a trailing comma in a
// hand-edited list is a typo, not a request for an empty address.
func nonEmpty(addresses []string) []string {
	kept := make([]string, 0, len(addresses))
	for _, address := range addresses {
		trimmed := strings.TrimSpace(address)
		if trimmed == "" {
			continue
		}
		kept = append(kept, trimmed)
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// hostMACAddresses returns the hardware addresses the machine presents when the
// account names none: what the kernel says its cards are.
func hostMACAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		// A host that cannot enumerate its interfaces reports none: an invented
		// address would name a device that does not exist.
		return nil
	}
	cards := make([]netCard, 0, len(interfaces))
	for _, iface := range interfaces {
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		cards = append(cards, netCard{
			name:       iface.Name,
			flags:      iface.Flags,
			hardware:   iface.HardwareAddr,
			hasAddress: hasIPAddress(addresses),
			virtual:    isVirtualInterface(iface.Name),
		})
	}
	return selectReportedAddresses(cards)
}

// netCard is what the selection looks at, separated from net.Interface so the
// rules can be exercised without the host's real cards.
type netCard struct {
	name       string
	flags      net.Flags
	hardware   net.HardwareAddr
	hasAddress bool
	// virtual marks an interface the kernel created rather than a card: a
	// bridge, bond, vlan, tunnel or container link.
	virtual bool
}

// selectReportedAddresses decides which cards the endpoint is reported as.
//
// An interface has to carry an address (a card without one is not carrying
// traffic), must not be a loopback or a tunnel, and must have a vendor address
// rather than the all-zero one. Virtual interfaces are dropped when at least one
// real card is left, because a container bridge or a mesh tunnel is not the
// machine; when every card is virtual the full list goes out, since an empty
// report would leave the portal nothing to bind the endpoint to.
//
// The result is stable across launches: it comes from the kernel, not from a
// stored copy.
func selectReportedAddresses(cards []netCard) []string {
	physical := make([]string, 0, len(cards))
	all := make([]string, 0, len(cards))
	for _, card := range cards {
		if !cardReportable(card) {
			continue
		}
		address := card.hardware.String()
		all = append(all, address)
		if !card.virtual {
			physical = append(physical, address)
		}
	}
	if len(physical) > 0 {
		return dedupe(physical)
	}
	if len(all) > 0 {
		return dedupe(all)
	}
	return nil
}

// cardReportable reports whether the interface could be the endpoint's.
func cardReportable(card netCard) bool {
	if card.flags&net.FlagLoopback != 0 {
		return false
	}
	if len(card.hardware) != 6 || isZeroHardwareAddr(card.hardware) {
		return false
	}
	if !card.hasAddress {
		return false
	}
	return !isTunnelName(card.name)
}

// isTunnelName reports a point-to-point interface by the prefix the kernel
// gives it. Such an interface carries a link address but is not the card the
// machine reaches the gateway with while it is disconnected.
func isTunnelName(name string) bool {
	return strings.HasPrefix(name, "tun") || strings.HasPrefix(name, "utun")
}

// hasIPAddress reports whether the interface carries an address a gateway could
// answer on.
func hasIPAddress(addresses []net.Addr) bool {
	for _, address := range addresses {
		if _, _, err := net.ParseCIDR(address.String()); err == nil {
			return true
		}
	}
	return false
}

// isZeroHardwareAddr reports the address of an interface that has none. A
// tunnel and a down card both present it, and neither is the machine's card.
func isZeroHardwareAddr(address net.HardwareAddr) bool {
	for _, octet := range address {
		if octet != 0 {
			return false
		}
	}
	return true
}

// isVirtualInterface reports whether the kernel created the interface rather
// than naming a card.
func isVirtualInterface(name string) bool {
	if name == "" {
		return true
	}
	_, err := os.Stat(filepath.Join(virtualNetDir, name))
	return err == nil
}

// dedupe keeps the first occurrence of each address: a bond and its slave, or
// two names for one card, would otherwise report the same value twice.
func dedupe(addresses []string) []string {
	seen := make(map[string]bool, len(addresses))
	unique := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if seen[address] {
			continue
		}
		seen[address] = true
		unique = append(unique, address)
	}
	return unique
}
