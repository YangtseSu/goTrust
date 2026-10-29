// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
)

// The traffic card is the mockup's: Traffic, the two directions with their newest
// rates, the minute's window at the right, and the sparkline under them.
func TestTrafficCardShowsTheRates(t *testing.T) {
	state := State{
		Session: session.State{Phase: session.PhaseOnline},
		Config:  config.Default(),
		Traffic: TrafficState{Up: 1229, Down: 8_808_038},
	}
	tester := ui.NewTester(View(&state, Actions{}), 900, 740)
	tester.Frame()

	for _, want := range []string{"流量", "下行 8.4 MB/s", "上行 1.2 KB/s", "最近 60 秒"} {
		if !tester.HasText(want) {
			t.Fatalf("%q is missing from the traffic card; texts: %v", want, tester.Texts())
		}
	}
}

// A rate steps through the units a network meter shows, with one decimal above
// a kilobyte and none below it.
func TestTrafficRateUnits(t *testing.T) {
	cases := []struct {
		rate float64
		want string
	}{
		{rate: 0, want: "0 B/s"},
		{rate: 512, want: "512 B/s"},
		{rate: 1024, want: "1.0 KB/s"},
		{rate: 1536, want: "1.5 KB/s"},
		{rate: 8.4 * 1024 * 1024, want: "8.4 MB/s"},
		{rate: 3 * 1024 * 1024 * 1024, want: "3.0 GB/s"},
	}
	for _, testCase := range cases {
		if got := trafficRate(testCase.rate); got != testCase.want {
			t.Fatalf("trafficRate(%v) = %q, want %q", testCase.rate, got, testCase.want)
		}
	}
}
