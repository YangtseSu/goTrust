// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"encoding/json"
	"path"
	"testing"
)

const (
	testDeviceID = "0123456789ABCDEF0123456789ABCDEF"
	testMAC      = "02:00:5e:10:00:01"
)

func TestSeedDeviceIDAddsTheInstallID(t *testing.T) {
	seeded, err := seedIdentity([]byte(`{"cookies":[{"name":"sid","value":"x"}]}`), testDeviceID, []string{testMAC})
	if err != nil {
		t.Fatalf("seedIdentity() error = %v", err)
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(seeded, &record); err != nil {
		t.Fatalf("parse the seeded session: %v", err)
	}
	var deviceID string
	if err := json.Unmarshal(record["device_id"], &deviceID); err != nil {
		t.Fatalf("device id: %v", err)
	}
	if deviceID != testDeviceID {
		t.Fatalf("device id = %q, want %q", deviceID, testDeviceID)
	}
	if _, ok := record["cookies"]; !ok {
		t.Fatal("the stored cookies were dropped")
	}
	var macs []string
	if err := json.Unmarshal(record["mac_addresses"], &macs); err != nil {
		t.Fatalf("mac addresses: %v", err)
	}
	if len(macs) != 1 || macs[0] != testMAC {
		t.Fatalf("mac addresses = %v, want the install's", macs)
	}
}

func TestSeedDeviceIDReplacesAnEarlierID(t *testing.T) {
	seeded, err := seedIdentity([]byte(`{"device_id":"OLDPROFILEID"}`), testDeviceID, []string{testMAC})
	if err != nil {
		t.Fatalf("seedDeviceID() error = %v", err)
	}
	var record struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(seeded, &record); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if record.DeviceID != testDeviceID {
		t.Fatalf("device id = %q, want the install's", record.DeviceID)
	}
}

// A configuration without an identity leaves the stored session alone, and a
// corrupt file is an error rather than a silent replacement.
func TestSeedDeviceIDWithoutAnIdentity(t *testing.T) {
	original := []byte(`{"device_id":"PROFILEID"}`)
	seeded, err := seedIdentity(original, "", nil)
	if err != nil {
		t.Fatalf("seedIdentity() error = %v", err)
	}
	if string(seeded) != string(original) {
		t.Fatalf("stored session = %s, want it untouched", seeded)
	}
	if _, err := seedIdentity([]byte("not json"), testDeviceID, nil); err == nil {
		t.Fatal("a corrupt stored session was accepted")
	}
}

// A profile that already reported a hardware address keeps it: the portal
// bound that endpoint to the value, so replacing it would invent a new device.
func TestSeedIdentityKeepsAStoredAddress(t *testing.T) {
	seeded, err := seedIdentity([]byte(`{"mac_addresses":["02:00:5e:10:00:09"]}`), testDeviceID, []string{testMAC})
	if err != nil {
		t.Fatalf("seedIdentity() error = %v", err)
	}
	var record struct {
		MACAddresses []string `json:"mac_addresses"`
	}
	if err := json.Unmarshal(seeded, &record); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(record.MACAddresses) != 1 || record.MACAddresses[0] != "02:00:5e:10:00:09" {
		t.Fatalf("mac addresses = %v, want the stored one", record.MACAddresses)
	}
}

// The L3 environment names this client, not the upstream library's default.
func TestClientProcessNamesTheRunningBinary(t *testing.T) {
	process := clientProcess()
	if process.Name != "goTrust" {
		t.Fatalf("name = %q", process.Name)
	}
	if process.Path == "" || !path.IsAbs(process.Path) {
		t.Fatalf("path = %q, want the running binary's absolute path", process.Path)
	}
}
