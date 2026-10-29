// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/proxy"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

// The card draws a fixed minute: a fresh ring is a minute of zeros, and a push
// replaces the oldest sample rather than appending past the end.
func TestTrafficRingKeepsTheLastMinute(t *testing.T) {
	var ring trafficRing
	state := ring.state()
	if len(state.UpSeries) != gotrustui.TrafficPoints || len(state.DownSeries) != gotrustui.TrafficPoints {
		t.Fatalf("fresh series = %d/%d points, want %d", len(state.UpSeries), len(state.DownSeries), gotrustui.TrafficPoints)
	}
	if state.Up != 0 || state.Down != 0 {
		t.Fatalf("fresh rates = %v/%v, want 0/0", state.Up, state.Down)
	}

	for i := 1; i <= gotrustui.TrafficPoints; i++ {
		ring.push(float64(i), float64(2*i))
	}
	state = ring.state()
	if state.UpSeries[0] != 1 || state.UpSeries[gotrustui.TrafficPoints-1] != gotrustui.TrafficPoints {
		t.Fatalf("full minute = %v..%v, want 1..%d", state.UpSeries[0], state.UpSeries[gotrustui.TrafficPoints-1], gotrustui.TrafficPoints)
	}
	if state.Up != gotrustui.TrafficPoints || state.Down != 2*gotrustui.TrafficPoints {
		t.Fatalf("newest = %v/%v, want %d/%d", state.Up, state.Down, gotrustui.TrafficPoints, 2*gotrustui.TrafficPoints)
	}

	// One more sample drops the oldest and becomes the newest point.
	ring.push(99, 198)
	state = ring.state()
	if state.UpSeries[0] != 2 {
		t.Fatalf("oldest after one more = %v, want 2", state.UpSeries[0])
	}
	if state.Up != 99 || state.Down != 198 {
		t.Fatalf("newest = %v/%v, want 99/198", state.Up, state.Down)
	}
}

// A rate is the growth of the session's counters over the second between two
// readings; the first reading only starts the minute.
func TestTrafficSamplerComputesRates(t *testing.T) {
	sampler := &trafficSampler{}
	server := new(proxy.Server)
	start := time.Unix(1_700_000_000, 0)

	if _, changed := sampler.sample(server, proxy.Traffic{}, start); !changed {
		t.Fatal("the first reading of a session did not start the minute")
	}
	state, changed := sampler.sample(server, proxy.Traffic{Up: 2048, Down: 8192}, start.Add(time.Second))
	if !changed {
		t.Fatal("a reading with traffic did not publish")
	}
	if state.Up != 2048 || state.Down != 8192 {
		t.Fatalf("rates = %v/%v, want 2048/8192 bytes per second", state.Up, state.Down)
	}
	if newest := state.DownSeries[gotrustui.TrafficPoints-1]; newest != 8192 {
		t.Fatalf("newest sample = %v, want 8192", newest)
	}
}

// A session ends or is replaced and its minute goes with it: the counters of a
// new server begin at zero, and the same address reused inside one sample is
// the same thing with smaller totals. An idle app publishes no new point.
func TestTrafficSamplerStartsOver(t *testing.T) {
	sampler := &trafficSampler{}
	first := new(proxy.Server)
	second := new(proxy.Server)
	start := time.Unix(1_700_000_000, 0)

	sampler.sample(first, proxy.Traffic{}, start)
	state, _ := sampler.sample(first, proxy.Traffic{Up: 10_000, Down: 20_000}, start.Add(time.Second))
	if state.Up != 10_000 {
		t.Fatalf("first session rate = %v, want 10000", state.Up)
	}

	state, changed := sampler.sample(second, proxy.Traffic{}, start.Add(2*time.Second))
	if !changed {
		t.Fatal("a new session did not reset the minute")
	}
	if state.Up != 0 || state.Down != 0 {
		t.Fatalf("new session rates = %v/%v, want 0/0", state.Up, state.Down)
	}
	for i := range state.UpSeries {
		if state.UpSeries[i] != 0 {
			t.Fatalf("sample %d = %v, want the minute emptied", i, state.UpSeries[i])
		}
	}

	// The same pointer with a smaller reading is an address reused by a new
	// server, not a negative rate.
	state, changed = sampler.sample(first, proxy.Traffic{}, start.Add(3*time.Second))
	if !changed || state.Up != 0 {
		t.Fatalf("reused address state = %+v, changed = %v", state, changed)
	}

	sampler.sample(first, proxy.Traffic{Up: 10}, start.Add(4*time.Second))
	state, changed = sampler.sample(nil, proxy.Traffic{}, start.Add(5*time.Second))
	if !changed || state.Up != 0 {
		t.Fatalf("lost session state = %+v, changed = %v", state, changed)
	}
	if _, changed := sampler.sample(nil, proxy.Traffic{}, start.Add(6*time.Second)); changed {
		t.Fatal("an idle app published another point")
	}
}
