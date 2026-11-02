// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"encoding/json"
	"path"
	"testing"

	"github.com/YangtseSu/goTrust/core/config"
)

const testDeviceID = "0123456789ABCDEF0123456789ABCDEF"

func TestSeedDeviceIDAddsTheInstallID(t *testing.T) {
	seeded, err := seedIdentity([]byte(`{"cookies":[{"name":"sid","value":"x"}]}`), testDeviceID)
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
}

func TestSeedDeviceIDReplacesAnEarlierID(t *testing.T) {
	seeded, err := seedIdentity([]byte(`{"device_id":"OLDPROFILEID"}`), testDeviceID)
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
	seeded, err := seedIdentity(original, "")
	if err != nil {
		t.Fatalf("seedIdentity() error = %v", err)
	}
	if string(seeded) != string(original) {
		t.Fatalf("stored session = %s, want it untouched", seeded)
	}
	if _, err := seedIdentity([]byte("not json"), testDeviceID); err == nil {
		t.Fatal("a corrupt stored session was accepted")
	}
}

// The hardware address belongs to the machine, not to the record: an older one
// that still carries a value loses it, so the next login reads the real cards
// instead of reporting whatever was frozen into the file.
func TestSeedIdentityDropsAStoredAddress(t *testing.T) {
	seeded, err := seedIdentity([]byte(`{"device_id":"OLD","mac_addresses":["02:00:5e:10:00:09"]}`), testDeviceID)
	if err != nil {
		t.Fatalf("seedIdentity() error = %v", err)
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(seeded, &record); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, present := record["mac_addresses"]; present {
		t.Fatalf("mac_addresses survived: %s", seeded)
	}
	var deviceID string
	if err := json.Unmarshal(record["device_id"], &deviceID); err != nil {
		t.Fatalf("device id: %v", err)
	}
	if deviceID != testDeviceID {
		t.Fatalf("device id = %q, want the configured one", deviceID)
	}
}

// A stored session without an identity gains the device id and nothing else:
// the address is decided at login, not stored.
func TestSeedIdentityFillsAStoredSessionWithoutAnIdentity(t *testing.T) {
	seeded, err := seedIdentity([]byte(`{"cookies":[]}`), testDeviceID)
	if err != nil {
		t.Fatalf("seedIdentity() error = %v", err)
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(seeded, &record); err != nil {
		t.Fatalf("parse: %v", err)
	}
	var deviceID string
	if err := json.Unmarshal(record["device_id"], &deviceID); err != nil {
		t.Fatalf("device id: %v", err)
	}
	if deviceID != testDeviceID {
		t.Fatalf("device id = %q, want %q", deviceID, testDeviceID)
	}
	if _, present := record["mac_addresses"]; present {
		t.Fatalf("a hardware address was written into the record: %s", seeded)
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

// TestPrepareStoredIdentityDropsAnotherDevicesSession: the stored sign key and
// ticket were minted with the recorded device, so a session that presents a
// different one would come up and have every authorization rejected. They are
// dropped instead, and the login mints a fresh pair.
func TestPrepareStoredIdentityDropsAnotherDevicesSession(t *testing.T) {
	stored := []byte(`{"device_id":"OLDDEVICE","sign_key":"key","sid_ticket":"ticket","cookies":[{"name":"sid","value":"x"}],"csrf_token":"csrf"}`)
	prepared, err := prepareStoredIdentity(stored, config.IdentityConfig{DeviceID: testDeviceID})
	if err != nil {
		t.Fatalf("prepareStoredIdentity() error = %v", err)
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(prepared, &record); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, gone := range []string{"sign_key", "sid_ticket", "cookies"} {
		if _, present := record[gone]; present {
			t.Fatalf("%s survived a device change", gone)
		}
	}
	var deviceID string
	if err := json.Unmarshal(record["device_id"], &deviceID); err != nil {
		t.Fatalf("device id: %v", err)
	}
	if deviceID != testDeviceID {
		t.Fatalf("device id = %q, want the configured one", deviceID)
	}
	if _, present := record["csrf_token"]; !present {
		t.Fatal("material the new device can still use was dropped")
	}
}

// A record that already names the configured device keeps its session: that is
// the normal start, and dropping the key would turn every launch into a login.
func TestPrepareStoredIdentityKeepsTheMatchingSession(t *testing.T) {
	stored := []byte(`{"device_id":"` + testDeviceID + `","sign_key":"key","sid_ticket":"ticket"}`)
	prepared, err := prepareStoredIdentity(stored, config.IdentityConfig{DeviceID: testDeviceID})
	if err != nil {
		t.Fatalf("prepareStoredIdentity() error = %v", err)
	}
	var record struct {
		SignKey   string `json:"sign_key"`
		SIDTicket string `json:"sid_ticket"`
	}
	if err := json.Unmarshal(prepared, &record); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if record.SignKey != "key" || record.SIDTicket != "ticket" {
		t.Fatalf("the stored session was dropped: %+v", record)
	}
}
