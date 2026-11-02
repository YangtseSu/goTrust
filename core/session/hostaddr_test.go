// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"net"
	"testing"
)

func mustHardwareAddr(t *testing.T, text string) net.HardwareAddr {
	t.Helper()
	address, err := net.ParseMAC(text)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	return address
}

// TestSelectReportedAddressesKeepsPhysicalCards: the endpoint is a machine, so
// a card the kernel created is not reported while a real one is present.
func TestSelectReportedAddressesKeepsPhysicalCards(t *testing.T) {
	cards := []netCard{
		{name: "wlan0", hardware: mustHardwareAddr(t, "00:1a:2b:3c:4d:5f"), hasAddress: true},
		{name: "docker0", hardware: mustHardwareAddr(t, "02:42:ac:11:00:02"), hasAddress: true, virtual: true},
		{name: "veth1a2b", hardware: mustHardwareAddr(t, "8a:1c:9d:00:11:22"), hasAddress: true, virtual: true},
	}
	got := selectReportedAddresses(cards)
	if len(got) != 1 || got[0] != "00:1a:2b:3c:4d:5f" {
		t.Fatalf("reported = %v, want the physical card alone", got)
	}
}

// TestSelectReportedAddressesDropsWhatCannotCarryTheEndpoint: a loopback, a
// tunnel, a card with no address and a card with no vendor address are not the
// machine's endpoint.
func TestSelectReportedAddressesDropsWhatCannotCarryTheEndpoint(t *testing.T) {
	cards := []netCard{
		{name: "lo", flags: net.FlagLoopback, hardware: mustHardwareAddr(t, "00:00:00:00:00:00"), hasAddress: true},
		{name: "tun0", hardware: mustHardwareAddr(t, "0a:1b:2c:3d:4e:5f"), hasAddress: true},
		{name: "eth9", hardware: mustHardwareAddr(t, "00:1a:2b:3c:4d:60")},
		{name: "gretap0", hardware: mustHardwareAddr(t, "00:00:00:00:00:00"), hasAddress: true},
		{name: "eth0", hardware: mustHardwareAddr(t, "00:1a:2b:3c:4d:5e"), hasAddress: true},
	}
	got := selectReportedAddresses(cards)
	if len(got) != 1 || got[0] != "00:1a:2b:3c:4d:5e" {
		t.Fatalf("reported = %v, want only the card that carries an address", got)
	}
}

// TestSelectReportedAddressesReportsVirtualCardsWhenThereIsNothingElse: a host
// whose only links are virtual still has to name an endpoint, so the list goes
// out rather than staying empty.
func TestSelectReportedAddressesReportsVirtualCardsWhenThereIsNothingElse(t *testing.T) {
	cards := []netCard{
		{name: "Meta", hardware: mustHardwareAddr(t, "8e:4d:00:1a:2b:3c"), hasAddress: true, virtual: true},
		{name: "br0", hardware: mustHardwareAddr(t, "8e:4d:00:1a:2b:3d"), hasAddress: true, virtual: true},
	}
	got := selectReportedAddresses(cards)
	if len(got) != 2 {
		t.Fatalf("reported = %v, want both virtual cards", got)
	}
}

// TestSelectReportedAddressesDedupes: one card under two names is one endpoint.
func TestSelectReportedAddressesDedupes(t *testing.T) {
	cards := []netCard{
		{name: "enp99s0", hardware: mustHardwareAddr(t, "00:1a:2b:3c:4d:5e"), hasAddress: true},
		{name: "enx001a2b3c4d5e", hardware: mustHardwareAddr(t, "00:1a:2b:3c:4d:5e"), hasAddress: true},
	}
	got := selectReportedAddresses(cards)
	if len(got) != 1 {
		t.Fatalf("reported = %v, want one entry", got)
	}
}

// TestSelectReportedAddressesWithNoCards: a host that presents no usable card
// reports nothing instead of inventing one.
func TestSelectReportedAddressesWithNoCards(t *testing.T) {
	if got := selectReportedAddresses(nil); len(got) != 0 {
		t.Fatalf("reported = %v, want none", got)
	}
}

// TestReportedMACAddressesPrefersTheConfiguredValue: a value in the settings is
// the endpoint, exactly as typed, and is never replaced by the host's cards.
func TestReportedMACAddressesPrefersTheConfiguredValue(t *testing.T) {
	got := reportedMACAddresses([]string{" 00:1a:2b:3c:4d:5e ", "", "00:1a:2b:3c:4d:5f"})
	want := []string{"00:1a:2b:3c:4d:5e", "00:1a:2b:3c:4d:5f"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("reported = %v, want %v", got, want)
	}
}

// TestReportedMACAddressesReportsTheHostWhenUnset: an entry list of blanks is
// the same as no entry at all, so the machine is described by its own cards.
func TestReportedMACAddressesReportsTheHostWhenUnset(t *testing.T) {
	host := hostMACAddresses()
	for _, configured := range [][]string{nil, {}, {"", "  "}} {
		got := reportedMACAddresses(configured)
		if len(got) != len(host) {
			t.Fatalf("configured %v produced %v, want the host's %v", configured, got, host)
		}
		for index := range host {
			if got[index] != host[index] {
				t.Fatalf("configured %v produced %v, want the host's %v", configured, got, host)
			}
		}
	}
}
