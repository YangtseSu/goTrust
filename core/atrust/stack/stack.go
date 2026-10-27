package stack

import (
	"context"
	"net"

	"github.com/YangtseSu/goTrust/core/atrust/client"
	"github.com/YangtseSu/goTrust/core/atrust/internal/ippool"
)

type Stack interface {
	Run()
	SetupIPPool(ipPool *ippool.IPPool[[]client.DomainResource])
	DialTCP(ctx context.Context, addr *net.TCPAddr) (net.Conn, error)
	DialUDP(ctx context.Context, addr *net.UDPAddr) (net.Conn, error)
}
