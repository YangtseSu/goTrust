package dial

import (
	"context"
	"net"
	"testing"

	"github.com/YangtseSu/goTrust/core/atrust/client"
	"github.com/YangtseSu/goTrust/core/atrust/internal/ippool"
	"github.com/YangtseSu/goTrust/core/atrust/internal/ipresource"
	"github.com/YangtseSu/goTrust/core/atrust/resolve"
)

func TestMatchesIPResourcePreservesRangePortAndProtocolRules(t *testing.T) {
	resources := []client.IPResource{{
		IPMin:    net.IPv4(10, 0, 0, 1),
		IPMax:    net.IPv4(10, 0, 0, 10),
		PortMin:  443,
		PortMax:  443,
		Protocol: "tcp",
	}}
	index := ipresource.New(resources)

	tests := []struct {
		name    string
		ip      net.IP
		network string
		port    int
		want    bool
	}{
		{name: "match", ip: net.IPv4(10, 0, 0, 5), network: "tcp", port: 443, want: true},
		{name: "outside range", ip: net.IPv4(10, 0, 0, 11), network: "tcp", port: 443},
		{name: "wrong port", ip: net.IPv4(10, 0, 0, 5), network: "tcp", port: 80},
		{name: "wrong protocol", ip: net.IPv4(10, 0, 0, 5), network: "udp", port: 443},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchesIPResource(index, test.ip, test.network, test.port); got != test.want {
				t.Fatalf("matchesIPResource() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDialIPPortSelectsMatchingDomainResource(t *testing.T) {
	resources := []client.DomainResource{
		{PortMin: 443, PortMax: 443, Protocol: "tcp", AppID: "https"},
		{PortMin: 80, PortMax: 80, Protocol: "tcp", AppID: "http"},
	}
	ctx := context.WithValue(context.Background(), resolve.ContextKeyDomainResource, resources)
	dialer := &Dialer{stack: &capturingStack{}, resourceIndex: ipresource.New(nil)}

	_, err := dialer.DialIPPort(ctx, "tcp", "192.0.2.13:443")
	if err != nil {
		t.Fatalf("DialIPPort() error = %v", err)
	}
	if got := dialer.stack.(*capturingStack).domainResource.AppID; got != "https" {
		t.Fatalf("selected AppID = %q, want https", got)
	}
}

func TestDialIPPortPrefersTCPTunnelResourceOverOverlappingL3Resource(t *testing.T) {
	resources := []client.DomainResource{
		{PortMin: 443, PortMax: 443, Protocol: "tcp", AppID: "l3", EnableTCPPrefL3: true},
		{PortMin: 443, PortMax: 443, Protocol: "tcp", AppID: "tcp"},
	}
	ctx := context.WithValue(context.Background(), resolve.ContextKeyDomainResource, resources)
	dialer := &Dialer{stack: &capturingStack{}, resourceIndex: ipresource.New(nil)}

	if _, err := dialer.DialIPPort(ctx, "tcp", "192.0.2.13:443"); err != nil {
		t.Fatalf("DialIPPort() error = %v", err)
	}
	if got := dialer.stack.(*capturingStack).domainResource.AppID; got != "tcp" {
		t.Fatalf("selected AppID = %q, want tcp", got)
	}
}

func TestDialIPPortCarriesTCPPrefL3IPResource(t *testing.T) {
	resource := client.IPResource{
		IPMin: net.IPv4(10, 0, 0, 42), IPMax: net.IPv4(10, 0, 0, 42), PortMin: 443, PortMax: 443,
		Protocol: "tcp", AppID: "l3", EnableTCPPrefL3: true,
	}
	stack := &capturingStack{}
	dialer := &Dialer{stack: stack, ipResources: []client.IPResource{resource}, resourceIndex: ipresource.New([]client.IPResource{resource})}

	if _, err := dialer.DialIPPort(context.Background(), "tcp", "10.0.0.42:443"); err != nil {
		t.Fatalf("DialIPPort() error = %v", err)
	}
	if !stack.ipResource.EnableTCPPrefL3 || stack.ipResource.AppID != "l3" {
		t.Fatalf("selected IP resource = %#v, want l3 TCP-prefers-L3", stack.ipResource)
	}
}

type capturingStack struct {
	domainResource client.DomainResource
	ipResource     client.IPResource
	connections    int
}

func (s *capturingStack) Run()                                                {}
func (s *capturingStack) SetupIPPool(*ippool.IPPool[[]client.DomainResource]) {}
func (s *capturingStack) DialTCP(ctx context.Context, _ *net.TCPAddr) (net.Conn, error) {
	s.connections++
	if resource, ok := ctx.Value(resolve.ContextKeyDomainResource).(client.DomainResource); ok {
		s.domainResource = resource
	}
	if resource, ok := ctx.Value(resolve.ContextKeyIPResource).(client.IPResource); ok {
		s.ipResource = resource
	}
	return nil, nil
}
func (s *capturingStack) DialUDP(context.Context, *net.UDPAddr) (net.Conn, error) {
	return nil, nil
}

// TestRouteForAnswersWithoutDialling: the keep-alive asks where a destination
// would go. The answer must be the same decision Dial makes, and asking must
// not open anything.
func TestRouteForAnswersWithoutDialling(t *testing.T) {
	resources := []client.IPResource{{
		IPMin:    net.IPv4(192, 0, 2, 10),
		IPMax:    net.IPv4(192, 0, 2, 10),
		PortMin:  80,
		PortMax:  80,
		Protocol: "tcp",
	}}
	stack := &capturingStack{}
	dialer := &Dialer{stack: stack, ipResources: resources, resourceIndex: ipresource.New(resources)}

	for _, tt := range []struct {
		name string
		addr string
		want bool
	}{
		{name: "a granted address on its granted port", addr: "192.0.2.10:80", want: true},
		{name: "a granted address on another port", addr: "192.0.2.10:8080"},
		{name: "an address outside the grant", addr: "198.51.100.7:80"},
		{name: "an ipv6 destination", addr: "[2001:db8::1]:80"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dialer.RouteFor(context.Background(), "tcp", tt.addr)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("RouteFor(%s) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
	if stack.connections != 0 {
		t.Fatalf("RouteFor opened %d connections, want none", stack.connections)
	}
}
