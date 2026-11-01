// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
)

// seedIdentity makes the stored session carry the install's device id and
// hardware address, so every profile logs in as the same device instead of
// minting one per account. Stored material without the fields is left otherwise
// untouched, and a value already in the file wins: the MAC the portal recorded
// for this profile stays the one it sees.
func seedIdentity(data []byte, deviceID string, macAddresses []string) ([]byte, error) {
	if deviceID == "" && len(macAddresses) == 0 {
		return data, nil
	}
	record := make(map[string]json.RawMessage)
	if len(data) > 0 {
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("parse the stored session: %w", err)
		}
	}
	if deviceID != "" {
		encoded, err := json.Marshal(deviceID)
		if err != nil {
			return nil, err
		}
		record["device_id"] = encoded
	}
	if len(macAddresses) > 0 {
		if _, present := record["mac_addresses"]; !present {
			encoded, err := json.Marshal(macAddresses)
			if err != nil {
				return nil, err
			}
			record["mac_addresses"] = encoded
		}
	}
	return json.Marshal(record)
}
