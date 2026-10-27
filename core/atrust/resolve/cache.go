package resolve

import (
	"net"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

// newDNSCache builds the hostname cache. A zero ttl keeps entries forever,
// which is what a configured ttl of zero has always meant.
func newDNSCache(ttl time.Duration) *ttlcache.Cache[string, net.IP] {
	return ttlcache.New(
		ttlcache.WithTTL[string, net.IP](ttl),
		// A lookup must not push the expiry out: the resolver caches the
		// answer for the configured ttl, not for the ttl after the last use.
		ttlcache.WithDisableTouchOnHit[string, net.IP](),
	)
}

func (r *Resolver) getDNSCache(host string) (net.IP, bool) {
	host = normalizeHostname(host)
	item := r.dnsCache.Get(host)
	if item == nil {
		return nil, false
	}
	return item.Value(), true
}

func (r *Resolver) setDNSCache(host string, ip net.IP) {
	host = normalizeHostname(host)
	r.dnsCache.Set(host, ip, ttlcache.DefaultTTL)
}

func (r *Resolver) SetPermanentDNS(host string, ip net.IP) {
	host = normalizeHostname(host)
	r.dnsCache.Set(host, ip, ttlcache.NoTTL)
}
