// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package handoff

import (
	"encoding/binary"
	"math"
	"net/netip"
	"sort"

	"github.com/YangtseSu/goTrust/core/session"
)

// grantedPrefixes folds the granted address ranges into one sorted, minimal
// set of IPv4 prefixes. The grant is expressed as inclusive ranges, which is
// not what a PAC isInNet check or a Clash IP-CIDR rule can express directly.
func grantedPrefixes(rules []session.IPRule) []netip.Prefix {
	if len(rules) == 0 {
		return nil
	}

	type interval struct{ start, end uint32 }
	intervals := make([]interval, 0, len(rules))
	for _, rule := range rules {
		start, ok := ipv4Value(rule.From)
		if !ok {
			continue
		}
		end, ok := ipv4Value(rule.To)
		if !ok {
			end = start
		}
		if start > end {
			start, end = end, start
		}
		intervals = append(intervals, interval{start: start, end: end})
	}
	if len(intervals) == 0 {
		return nil
	}

	sort.Slice(intervals, func(i, j int) bool {
		if intervals[i].start != intervals[j].start {
			return intervals[i].start < intervals[j].start
		}
		return intervals[i].end < intervals[j].end
	})

	merged := []interval{intervals[0]}
	for _, next := range intervals[1:] {
		last := &merged[len(merged)-1]
		// Adjacent ranges merge too: the grant is a destination set, so
		// 10.0.0.0/25 next to 10.0.0.128/25 is 10.0.0.0/24.
		if uint64(next.start) <= uint64(last.end)+1 {
			if next.end > last.end {
				last.end = next.end
			}
			continue
		}
		merged = append(merged, next)
	}

	var prefixes []netip.Prefix
	for _, span := range merged {
		prefixes = append(prefixes, rangeToPrefixes(span.start, span.end)...)
	}
	return prefixes
}

// rangeToPrefixes returns the smallest aligned blocks covering the inclusive
// range, in ascending order.
func rangeToPrefixes(start, end uint32) []netip.Prefix {
	var prefixes []netip.Prefix
	for {
		size := uint64(0)
		bits := 0
		for candidate := 0; candidate <= 32; candidate++ {
			candidateSize := uint64(1) << (32 - candidate)
			if uint64(start)%candidateSize != 0 {
				continue
			}
			if uint64(end)-uint64(start)+1 < candidateSize {
				continue
			}
			size = candidateSize
			bits = candidate
			break
		}
		if size == 0 {
			break
		}
		prefix, err := addr4(start).Prefix(bits)
		if err != nil {
			break
		}
		prefixes = append(prefixes, prefix)
		next := uint64(start) + size
		if next > math.MaxUint32 {
			break
		}
		start = uint32(next)
	}
	return prefixes
}

func ipv4Value(value string) (uint32, bool) {
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.Is4() {
		return 0, false
	}
	bytes := addr.As4()
	return binary.BigEndian.Uint32(bytes[:]), true
}

func addr4(value uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(value >> 24),
		byte(value >> 16),
		byte(value >> 8),
		byte(value),
	})
}
