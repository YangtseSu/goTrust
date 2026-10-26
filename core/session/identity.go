// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
)

// seedDeviceID makes the stored session carry the install's device id, so
// every profile logs in as the same device instead of minting one per account.
// Stored material without the field is left otherwise untouched.
func seedDeviceID(data []byte, deviceID string) ([]byte, error) {
	if deviceID == "" {
		return data, nil
	}
	record := make(map[string]json.RawMessage)
	if len(data) > 0 {
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("parse the stored session: %w", err)
		}
	}
	encoded, err := json.Marshal(deviceID)
	if err != nil {
		return nil, err
	}
	record["device_id"] = encoded
	return json.Marshal(record)
}
