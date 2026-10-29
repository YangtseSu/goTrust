// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"context"
	"time"

	"github.com/YangtseSu/goTrust/core/proxy"
	gotrustui "github.com/YangtseSu/goTrust/ui"
)

// sampleTraffic publishes the proxy's throughput for the dashboard card, one
// point a second for the life of the app. The card draws the last minute, so
// an idle session keeps flattening the line instead of freezing it.
func (a *app) sampleTraffic(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var sampler trafficSampler
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.mu.RLock()
			server := a.proxy
			a.mu.RUnlock()

			var total proxy.Traffic
			if server != nil {
				total = server.Traffic()
			}
			state, changed := sampler.sample(server, total, now)
			if !changed {
				continue
			}
			a.mu.Lock()
			a.view.Traffic = state
			a.mu.Unlock()
			a.invalidate()
		}
	}
}

// trafficSampler turns the proxy's running byte totals into per-second rates.
// One goroutine owns it; the window only sees the state it publishes.
type trafficSampler struct {
	server *proxy.Server
	last   proxy.Traffic
	at     time.Time
	ring   trafficRing
}

// sample advances the minute by one reading, taken at now from total. It
// reports the state to publish: a new server (or a reading smaller than the
// one before, which is the same thing after an address is reused) starts a
// new minute, because a session's counters begin at zero. A repeated reading
// of no session is not a new point, so an idle window is not repainted every
// second.
func (s *trafficSampler) sample(server *proxy.Server, total proxy.Traffic, now time.Time) (gotrustui.TrafficState, bool) {
	switch {
	case server == nil && s.server == nil:
		return gotrustui.TrafficState{}, false
	case server != s.server || total.Up < s.last.Up || total.Down < s.last.Down:
		s.server = server
		s.ring.reset()
		s.last, s.at = total, now
		return s.ring.state(), true
	}

	elapsed := now.Sub(s.at).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	s.ring.push(float64(total.Up-s.last.Up)/elapsed, float64(total.Down-s.last.Down)/elapsed)
	s.last, s.at = total, now
	return s.ring.state(), true
}

// trafficRing is the last minute of sampled rates, one pair a second. next is
// the oldest sample, which the next push replaces.
type trafficRing struct {
	up, down [gotrustui.TrafficPoints]float64
	next     int
}

// reset empties the minute. A new session starts its own chart: its counters
// begin at zero, so the previous session's rates do not belong to it.
func (r *trafficRing) reset() { *r = trafficRing{} }

func (r *trafficRing) push(up, down float64) {
	r.up[r.next] = up
	r.down[r.next] = down
	r.next = (r.next + 1) % len(r.up)
}

// state flattens the ring into the card's series, oldest first, so the newest
// sample is the last element and the card's header reads the live rate.
func (r *trafficRing) state() gotrustui.TrafficState {
	points := len(r.up)
	state := gotrustui.TrafficState{
		UpSeries:   make([]float64, points),
		DownSeries: make([]float64, points),
	}
	for i := range points {
		at := (r.next + i) % points
		state.UpSeries[i] = r.up[at]
		state.DownSeries[i] = r.down[at]
	}
	state.Up = state.UpSeries[points-1]
	state.Down = state.DownSeries[points-1]
	return state
}
