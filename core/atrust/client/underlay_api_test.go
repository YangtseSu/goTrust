package client_test

import (
	"context"
	"net"
	"testing"

	"github.com/YangtseSu/goTrust/core/atrust/client"
	"github.com/YangtseSu/goTrust/core/atrust/client/atrust"
)

type externalUnderlay struct{}

func (externalUnderlay) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, nil
}

var (
	_ client.UnderlayDialer = externalUnderlay{}
	_ client.UnderlayDialer = (*net.Dialer)(nil)
)

func TestPublicClientAcceptsExternalUnderlay(t *testing.T) {
	var dialer client.UnderlayDialer = externalUnderlay{}

	aTrustClient := atrust.NewClient(atrust.ClientOptions{UnderlayDialer: dialer})
	if aTrustClient == nil {
		t.Fatal("atrust.NewClient returned nil")
	}
	aTrustClient.Close()
}
