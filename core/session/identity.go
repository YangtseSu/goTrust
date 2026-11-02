// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/YangtseSu/goTrust/core/config"
)

// prepareStoredIdentity returns the stored session as the account presents it.
//
// The record is authoritative: the sign key and the sidTicket in it were minted
// with the device id beside them, so a session that presents a different device
// brings the tunnel up and has every authorization through it rejected. The
// configured identity therefore only fills a record that has no device yet.
//
// The one exception is a device id the account asks for on purpose: the record
// belongs to another device then, and keeping its key and ticket would produce
// exactly that dead tunnel, so they are dropped and the login mints a new pair.
func prepareStoredIdentity(data []byte, identity config.IdentityConfig) ([]byte, error) {
	record := struct {
		DeviceID string `json:"device_id"`
	}{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("parse the stored session: %w", err)
		}
	}
	if record.DeviceID != "" && identity.DeviceID != "" && !strings.EqualFold(record.DeviceID, identity.DeviceID) {
		data, err := dropStoredCredentials(data)
		if err != nil {
			return nil, err
		}
		log.Printf("The stored session belongs to device %s; logging in as %s needs a fresh one",
			record.DeviceID, identity.DeviceID)
		return seedIdentity(data, identity.DeviceID)
	}
	return seedIdentity(data, identity.DeviceID)
}

// dropStoredCredentials removes the material only the previous device could use.
func dropStoredCredentials(data []byte) ([]byte, error) {
	record := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse the stored session: %w", err)
	}
	delete(record, "sign_key")
	delete(record, "sid_ticket")
	delete(record, "cookies")
	return json.Marshal(record)
}

// seedIdentity makes the stored session carry the account's device id.
//
// The hardware address is not part of the record: it belongs to the machine and
// is read at every login, so a stored copy could only go stale when a card is
// added or replaced. A record written by an earlier version still has one, and
// it is dropped here rather than carried forward.
func seedIdentity(data []byte, deviceID string) ([]byte, error) {
	record := make(map[string]json.RawMessage)
	if len(data) > 0 {
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("parse the stored session: %w", err)
		}
	}
	_, hadAddress := record["mac_addresses"]
	if deviceID == "" && !hadAddress {
		// Nothing to add and nothing stale to drop: leave the record as it is
		// rather than rewriting an equivalent one in a different key order.
		return data, nil
	}
	delete(record, "mac_addresses")
	if deviceID != "" {
		encoded, err := json.Marshal(deviceID)
		if err != nil {
			return nil, err
		}
		record["device_id"] = encoded
	}
	return json.Marshal(record)
}
