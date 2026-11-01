package dial

import (
	"net"
	"strconv"
	"strings"

	"github.com/YangtseSu/goTrust/core/atrust/client"
	"github.com/YangtseSu/goTrust/core/atrust/internal/ipresource"
	"github.com/YangtseSu/goTrust/core/atrust/log"
	"github.com/YangtseSu/goTrust/core/atrust/resolve"
	"github.com/YangtseSu/goTrust/core/atrust/stack"
)

import (
	"context"
	"errors"
)

// ErrACLDenied is returned by DialIPPort when the caller forced VPN routing
// (via alwaysUseVPN / proxy_all) for a destination that the gateway would not
// accept. Sending it through the L3 tunnel would make the gateway terminate the
// whole session with a SHUTDOWN, killing every other in-flight connection.
// Refusing here keeps the tunnel alive.
var ErrACLDenied = errors.New("destination is outside the granted resource list (the gateway would shut the tunnel down)")

type Dialer struct {
	stack                stack.Stack
	resolver             *resolve.Resolver
	ipResources          []client.IPResource
	resourceIndex        *ipresource.Index
	alwaysUseVPN         bool
	dialDirectHTTPProxy  string // format: "ip:port"
	dialDirectSocksProxy string // WORKING IN PROCESS
}

// dialDirectIP need have a `hostAddr` parameter, which will be passed to PROXY. But `hostAddr` maybe empty, ipAddr never be empty.
func (d *Dialer) dialDirectIP(ctx context.Context, network, ipAddr string, hostAddr string) (net.Conn, error) {
	// only support http proxy now and tcp network type
	if d.dialDirectHTTPProxy != "" && network == "tcp" {
		usedAddr := ipAddr
		if hostAddr != "" {
			usedAddr = hostAddr
		}
		return d.dialDirectWithHTTPProxy(ctx, usedAddr)
		// only support tcp for socks proxy
	} else if d.dialDirectSocksProxy != "" && network == "tcp" {
		if hostAddr != "" {
			return d.dialDirectWithSocksProxy(ctx, network, hostAddr, false)
		} else {
			return d.dialDirectWithSocksProxy(ctx, network, ipAddr, true)
		}
	} else {
		return d.dialDirectWithoutProxy(ctx, network, ipAddr)
	}
}

func (d *Dialer) dialDirectHost(ctx context.Context, network, hostAddr string) (net.Conn, error) {
	// only support http proxy now and tcp network type
	if d.dialDirectHTTPProxy != "" && network == "tcp" {
		return d.dialDirectWithHTTPProxy(ctx, hostAddr)
		// only support tcp for socks proxy
	} else if d.dialDirectSocksProxy != "" && network == "tcp" {
		return d.dialDirectWithSocksProxy(ctx, network, hostAddr, false)
	} else {
		return d.dialDirectWithoutProxy(ctx, network, hostAddr)
	}
}

// tunnelRoute is the routing decision DialIPPort acts on: whether a
// destination goes through the tunnel, and whether the granted resources cover
// it. It returns the context carrying the resource that matched, which the
// tunnel needs to authorise the connection. The keep-alive asks the same
// question to tell a probe that crossed the tunnel from one that did not.
func (d *Dialer) tunnelRoute(ctx context.Context, network string, ip net.IP, port int) (context.Context, bool, bool) {
	useVPN := d.alwaysUseVPN
	matchedResource := false

	// Track whether dst:port matches any gateway-issued resource. We always
	// run both resource lookups (even if useVPN was already forced true by
	// alwaysUseVPN) so we can enforce the server-side ACL client-side.
	if res := ctx.Value(resolve.ContextKeyDomainResource); res != nil {
		var resource client.DomainResource
		var matched bool
		switch resources := res.(type) {
		case []client.DomainResource:
			resource, matched = matchDomainResourceForTunnel(resources, network, port)
		case client.DomainResource:
			resource, matched = matchDomainResourceForTunnel([]client.DomainResource{resources}, network, port)
		}
		if matched {
			ctx = context.WithValue(ctx, resolve.ContextKeyDomainResource, resource)
			useVPN = true
			matchedResource = true
		}
	}

	if !matchedResource && d.ipResources != nil {
		if resource, matched := matchIPResourceForTunnel(d.resourceIndex, ip, network, port); matched {
			ctx = context.WithValue(ctx, resolve.ContextKeyIPResource, resource)
			useVPN = true
			matchedResource = true
		}
	}

	return ctx, useVPN, matchedResource
}

// RouteFor reports whether Dial would send the destination through the tunnel,
// without opening anything. The keep-alive uses it as a witness: an answer
// reached over a direct connection proves the host is up, not that the session
// is being kept alive.
func (d *Dialer) RouteFor(ctx context.Context, network, addr string) (bool, error) {
	if strings.Count(addr, ":") > 1 {
		// IPv6 destinations are dialled directly.
		return false, nil
	}

	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return false, nil
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return false, nil
	}

	ip := net.ParseIP(host)
	if ip == nil {
		if d.resolver == nil {
			return false, errors.New("the destination name cannot be resolved without a resolver")
		}
		// The resolver attaches the domain resource to the context, which is
		// how a name-only grant is matched.
		ctx, ip, err = d.resolver.Resolve(ctx, host)
		if err != nil {
			// Dial falls back to a direct connection when a name does not
			// resolve, so that is the honest answer here too.
			return false, nil
		}
		if strings.Count(ip.String(), ":") > 0 {
			return false, nil
		}
	}

	_, useVPN, _ := d.tunnelRoute(ctx, network, ip, port)
	return useVPN, nil
}

func (d *Dialer) DialIPPort(ctx context.Context, network, ipAddr string) (net.Conn, error) {
	hostAddr := ""
	if _, hostAddrOK := ctx.Value(resolve.ContextKeyResolveHost).(string); hostAddrOK {
		// hostAddr doesn't have port field at now
		hostAddr = ctx.Value(resolve.ContextKeyResolveHost).(string)
	}
	parts := strings.Split(ipAddr, ":")
	if len(parts) >= 2 {
		// maybe need extra check for parts[len(parts)-1] is port or not?
		hostAddr += ":" + parts[len(parts)-1]
	}

	// If addr is IPv6, use direct connection
	if len(parts) > 2 {
		return d.dialDirectIP(ctx, network, ipAddr, hostAddr)
	}

	ip, portStr, err := net.SplitHostPort(ipAddr)
	if err != nil {
		return nil, errors.New("Invalid address: " + ipAddr)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, errors.New("Invalid port in address: " + ipAddr)
	}

	var target *net.IPAddr

	if pureIp := net.ParseIP(ip); pureIp != nil {
		target = &net.IPAddr{IP: pureIp}
	} else {
		log.Printf("Illegal situation, host is not pure IP format: %s", ip)
		return d.dialDirectIP(ctx, network, ipAddr, hostAddr)
	}

	var useVPN, matchedResource bool
	ctx, useVPN, matchedResource = d.tunnelRoute(ctx, network, target.IP, port)

	// Client-side ACL enforcement: if alwaysUseVPN forced VPN routing for a
	// dst:port that is not in the server-issued resource list, sending it
	// upstream makes the gateway terminate the L3 tunnel (a SHUTDOWN on the
	// next handshake). Filter here so the traffic never reaches the tunnel.
	// Skipped when IPResources are unavailable (parse_resource=false), since
	// there is no list to enforce. An empty but non-nil slice still means
	// resources were parsed and no IP destinations are allowed.
	if useVPN && !matchedResource && d.ipResources != nil {
		log.Printf("ACL: refusing %s/%s — outside the granted resource list (the gateway would shut the tunnel down)", ipAddr, network)
		return nil, ErrACLDenied
	}

	if useVPN {
		if network == "tcp" {
			log.Printf("%s -> VPN", ipAddr)

			return d.stack.DialTCP(ctx, &net.TCPAddr{
				IP:   target.IP,
				Port: port,
			})
		} else if network == "udp" {
			log.Printf("%s -> VPN", ipAddr)

			return d.stack.DialUDP(ctx, &net.UDPAddr{
				IP:   target.IP,
				Port: port,
			})
		} else {
			log.Printf("VPN only support TCP/UDP. Connection to %s will use direct connection", ipAddr)
			return d.dialDirectIP(ctx, network, ipAddr, hostAddr)
		}
	} else {
		return d.dialDirectIP(ctx, network, ipAddr, hostAddr)
	}
}

func matchDomainResourceForTunnel(resources []client.DomainResource, network string, port int) (client.DomainResource, bool) {
	if network == "tcp" {
		if resource, ok := client.MatchDomainResourceWhere(resources, network, port, func(resource client.DomainResource) bool {
			return !resource.EnableTCPPrefL3
		}); ok {
			return resource, true
		}
	}
	return client.MatchDomainResource(resources, network, port)
}

func matchesIPResource(index *ipresource.Index, target net.IP, network string, port int) bool {
	_, ok := matchIPResourceForTunnel(index, target, network, port)
	return ok
}

func matchIPResourceForTunnel(index *ipresource.Index, target net.IP, network string, port int) (client.IPResource, bool) {
	if network == "tcp" {
		if resource, ok := index.MatchWhere(target, network, port, func(resource client.IPResource) bool {
			return !resource.EnableTCPPrefL3
		}); ok {
			return resource, true
		}
	}
	return index.Match(target, network, port)
}

func (d *Dialer) Dial(ctx context.Context, network string, addr string) (net.Conn, error) {
	// If addr is IPv6, use direct connection
	if strings.Count(addr, ":") > 1 {
		return d.dialDirectIP(ctx, network, addr, "")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return d.dialDirectHost(ctx, network, addr)
	}

	var ip net.IP
	if ip = net.ParseIP(host); ip == nil {
		ctx, ip, err = d.resolver.Resolve(ctx, host)
		if err != nil {
			return d.dialDirectHost(ctx, network, addr)
		}

		if strings.Count(ip.String(), ":") > 0 {
			return d.dialDirectIP(ctx, network, ip.String()+":"+port, addr)
		}
	}

	return d.DialIPPort(ctx, network, ip.String()+":"+port)
}

func NewDialer(stack stack.Stack, resolver *resolve.Resolver, ipResources []client.IPResource, alwaysUseVPN bool, dialDirectProxy string) *Dialer {
	dialHttpProxy := ""
	dialSocksProxy := ""
	if strings.HasPrefix(dialDirectProxy, "http://") {
		dialHttpProxy = strings.TrimPrefix(dialDirectProxy, "http://")
	} else if strings.HasPrefix(dialDirectProxy, "socks://") {
		dialSocksProxy = strings.TrimPrefix(dialDirectProxy, "socks://")
	} else if len(dialDirectProxy) > 0 {
		log.Println("DialDirectProxy supports only [http/socks]; ignoring the option")
	}
	return &Dialer{
		stack:                stack,
		resolver:             resolver,
		ipResources:          ipResources,
		resourceIndex:        ipresource.New(ipResources),
		alwaysUseVPN:         alwaysUseVPN,
		dialDirectHTTPProxy:  dialHttpProxy,
		dialDirectSocksProxy: dialSocksProxy,
	}
}
