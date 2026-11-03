package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/YangtseSu/goTrust/core/atrust/client/authchallenge"
	"github.com/YangtseSu/goTrust/core/atrust/log"
)

// trustedDevice is one row of the portal's trustedDeviceList: an endpoint the
// account has bound, named so a person can pick one to unbind.
type trustedDevice struct {
	ID         string `json:"id"`
	DeviceName string `json:"deviceName"`
	DeviceType string `json:"deviceType"`
	OS         string `json:"os"`
	BindTime   string `json:"bindTime"`
}

// bindAuthDeviceData is the bind answer. bindStatus=1 means the account is at
// its device limit and one or more of the listed endpoints has to go before
// this one may be bound.
type bindAuthDeviceData struct {
	authStepData
	BindStatus        int             `json:"bindStatus"`
	CurBindNum        int             `json:"curBindNum"`
	BindNumLimit      int             `json:"bindNumLimit"`
	TrustedDeviceList []trustedDevice `json:"trustedDeviceList"`
}

type bindAuthDeviceResponse struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Data    bindAuthDeviceData `json:"data"`
}

// bindAuthDevice completes the trusted-device step. The portal is asked first
// with nothing to unbind; only an answer that reports the device limit reached
// turns into a question, and the question is a device-list pick rather than a
// yes/no confirmation.
func (s *Session) bindAuthDevice() (authStep, error) {
	result, err := s.requestBindAuthDevice("[]")
	if err != nil {
		return authStep{}, err
	}
	if result.Data.BindStatus != 1 {
		return authStepFromData(result.Data.authStepData), nil
	}
	if len(result.Data.TrustedDeviceList) == 0 {
		return authStep{}, fmt.Errorf("the gateway reached its trusted-device limit (%d) but listed no device to unbind",
			result.Data.BindNumLimit)
	}

	// The account is over its limit by curBindNum-bindNumLimit plus this
	// device, so at least one endpoint has to go.
	need := max(1, result.Data.CurBindNum-result.Data.BindNumLimit+1)
	numbers := make([]string, 0, need)
	for index := range need {
		numbers = append(numbers, strconv.Itoa(index+1))
	}

	message := bindDeviceMessage(result.Message, result.Data, numbers)
	response, err := s.challengeHandler.HandleCodeChallenge(authchallenge.CodeChallenge{
		Kind:    authchallenge.CodeDevice,
		Message: message,
	})
	if err != nil {
		return authStep{}, fmt.Errorf("choose the trusted device to unbind: %w", err)
	}

	unbindIDs, err := parseDeviceChoices(response.Code, need, result.Data.TrustedDeviceList)
	if err != nil {
		return authStep{}, err
	}
	unbindList, err := json.Marshal(unbindIDs)
	if err != nil {
		return authStep{}, err
	}
	result, err = s.requestBindAuthDevice(string(unbindList))
	if err != nil {
		return authStep{}, err
	}
	return authStepFromData(result.Data.authStepData), nil
}

// bindDeviceMessage lists the bound endpoints, one number each, and says how
// many have to go.
func bindDeviceMessage(reason string, data bindAuthDeviceData, numbers []string) string {
	var message strings.Builder
	if reason != "" {
		message.WriteString(reason)
		message.WriteString("\n")
	}
	for index, device := range data.TrustedDeviceList {
		name := device.DeviceName
		if name == "" {
			name = device.ID
		}
		fmt.Fprintf(&message, "%d) %s", index+1, name)
		if device.OS != "" {
			fmt.Fprintf(&message, "  os=%s", device.OS)
		}
		if device.BindTime != "" {
			fmt.Fprintf(&message, "  bound=%s", device.BindTime)
		}
		message.WriteString("\n")
	}
	fmt.Fprintf(&message, "Trusted devices: %d/%d. Unbind %s to make room for this one.",
		data.CurBindNum, data.BindNumLimit, strings.Join(numbers, ", "))
	return message.String()
}

// parseDeviceChoices turns the answered numbers into the ids to unbind. The
// count is exact: too few leaves the account over its limit and too many
// removes an endpoint the user did not need to remove.
func parseDeviceChoices(answer string, need int, devices []trustedDevice) ([]string, error) {
	fields := strings.FieldsFunc(answer, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	if len(fields) != need {
		return nil, fmt.Errorf("choose exactly %d trusted device number(s), got %d", need, len(fields))
	}
	ids := make([]string, 0, need)
	seen := make(map[int]bool, need)
	for _, field := range fields {
		index, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || index < 1 || index > len(devices) {
			return nil, fmt.Errorf("there is no trusted device numbered %q (1-%d)", field, len(devices))
		}
		if seen[index] {
			return nil, fmt.Errorf("trusted device %d was chosen twice", index)
		}
		seen[index] = true
		ids = append(ids, devices[index-1].ID)
	}
	return ids, nil
}

// requestBindAuthDevice posts one bind answer. unbindList is a JSON array in a
// string, which is what the endpoint takes.
func (s *Session) requestBindAuthDevice(unbindList string) (bindAuthDeviceResponse, error) {
	log.Println("Perform POST /passport/v1/auth/bindAuthDevice")

	body, err := json.Marshal(map[string]string{
		"unbindList": unbindList,
		"reason":     "",
	})
	if err != nil {
		return bindAuthDeviceResponse{}, err
	}
	u := s.baseURL + "/passport/v1/auth/bindAuthDevice?" + s.sharedParams(url.Values{}).Encode()
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return bindAuthDeviceResponse{}, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("x-csrf-token", s.csrfToken)
	req.Header.Set("x-sdp-traceid", s.randSdpId())

	resp, err := s.request(req)
	if err != nil {
		return bindAuthDeviceResponse{}, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return bindAuthDeviceResponse{}, err
	}
	log.DebugPrintf("Received bindAuthDevice: %s", string(responseBody))

	var result bindAuthDeviceResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return bindAuthDeviceResponse{}, err
	}
	if result.Code != 0 {
		return bindAuthDeviceResponse{}, &PortalCodeError{Op: "bindAuthDevice", Code: result.Code, Message: result.Message}
	}
	return result, nil
}
