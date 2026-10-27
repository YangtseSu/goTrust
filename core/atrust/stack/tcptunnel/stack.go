package tcptunnel

import (
	"github.com/YangtseSu/goTrust/core/atrust/client"
	"github.com/YangtseSu/goTrust/core/atrust/internal/ippool"
)

type Stack struct {
	client client.Client
	ipPool *ippool.IPPool[[]client.DomainResource]
}

func (s *Stack) Run() {}

func NewStack(client client.Client) (*Stack, error) {
	s := &Stack{
		client: client,
	}
	return s, nil
}

func (s *Stack) SetupIPPool(ipPool *ippool.IPPool[[]client.DomainResource]) {
	s.ipPool = ipPool
}
