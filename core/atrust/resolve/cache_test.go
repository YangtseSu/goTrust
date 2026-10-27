package resolve

import (
	"net"
	"testing"
	"time"
)

func TestDNSCacheHonorsTTLAndPermanentEntries(t *testing.T) {
	ttl := 10 * time.Millisecond
	resolver := &Resolver{dnsCache: newDNSCache(ttl)}
	resolver.setDNSCache("Cached.Example.", net.ParseIP("192.0.2.10"))
	resolver.SetPermanentDNS("Permanent.Example.", net.ParseIP("192.0.2.11"))

	if ip, ok := resolver.getDNSCache("cached.example"); !ok || !ip.Equal(net.ParseIP("192.0.2.10")) {
		t.Fatalf("cached entry = %v, %v, want 192.0.2.10", ip, ok)
	}
	if ip, ok := resolver.getDNSCache("permanent.example"); !ok || !ip.Equal(net.ParseIP("192.0.2.11")) {
		t.Fatalf("permanent entry = %v, %v, want 192.0.2.11", ip, ok)
	}

	time.Sleep(3 * ttl)

	if ip, ok := resolver.getDNSCache("cached.example"); ok {
		t.Fatalf("expired entry = %v, want a miss", ip)
	}
	if ip, ok := resolver.getDNSCache("permanent.example"); !ok || !ip.Equal(net.ParseIP("192.0.2.11")) {
		t.Fatalf("permanent entry after ttl = %v, %v, want 192.0.2.11", ip, ok)
	}
}

func TestDNSCacheWithZeroTTLKeepsEntries(t *testing.T) {
	resolver := &Resolver{dnsCache: newDNSCache(0)}
	resolver.setDNSCache("zero.example", net.ParseIP("192.0.2.20"))

	time.Sleep(10 * time.Millisecond)

	if ip, ok := resolver.getDNSCache("zero.example"); !ok || !ip.Equal(net.ParseIP("192.0.2.20")) {
		t.Fatalf("entry with zero ttl = %v, %v, want 192.0.2.20", ip, ok)
	}
}
