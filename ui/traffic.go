// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/YangtseSu/goTrust/locales"
)

// TrafficPoints is the number of one-second samples the traffic card draws:
// the mockup's last minute.
const TrafficPoints = 60

// TrafficState is the dashboard's traffic card: what the proxy moved over the
// last minute, one sample a second, in bytes per second. Up is what the client
// sent into the tunnel, Down what came back.
type TrafficState struct {
	// Up and Down are the newest rates, in bytes per second.
	Up, Down float64
	// UpSeries and DownSeries are the minute's samples, oldest first.
	UpSeries, DownSeries []float64
}

// trafficChartHeight is the mockup's sparkline: 80 DIPs under the header.
const trafficChartHeight float32 = 80

// trafficCard draws the proxy's throughput between the session and the granted
// resources: the newest rates in the header and the minute behind them as two
// lines, the download's filled.
func trafficCard(c *ui.Context, traffic TrafficState) {
	colors := paletteOf(c)
	card(c).Gap(c.Theme().Space(2.5)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			sectionHeading(c, locales.TrafficTitle())
			trafficLegend(c, locales.TrafficDown(trafficRate(traffic.Down)), colors.accent)
			trafficLegend(c, locales.TrafficUp(trafficRate(traffic.Up)), colors.success)
			ui.Box(c).Grow(1)
			caption(c, locales.TrafficWindow())
		})
		ui.Box(c).Height(trafficChartHeight).Shrink(0).
			Draw(func(p *ui.Painter, r ui.Rect) { paintTraffic(p, r, traffic, colors) }).
			Role(ui.RoleImage).Label(locales.TrafficChart())
	})
}

// trafficLegend is one direction of the header: the mockup's 7 DIP dot and its
// rate, both in the series' colour.
func trafficLegend(c *ui.Context, text string, color ui.Color) {
	ui.Row(c).AlignItems(ui.Center).Gap(c.Theme().Space(1.5)).Children(func() {
		ui.Box(c).Size(7, 7).Radius(4).Shrink(0).Background(color)
		caption(c, text)
	})
}

// trafficRate renders a rate with the unit the mockup shows. The steps are
// binary, as a network meter counts bytes, and one decimal is enough for a
// value that moves every second.
func trafficRate(rate float64) string {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	unit := 0
	for rate >= 1024 && unit < len(units)-1 {
		rate /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f %s", rate, units[unit])
	}
	return fmt.Sprintf("%.1f %s", rate, units[unit])
}

// paintTraffic draws the minute's two series on one scale, so the taller line
// is the faster direction: the download as the mockup's accent area fading to
// nothing under its stroke, the upload as the success line above it. A minute
// with no sample or no traffic draws a muted baseline instead, so the card
// reads as idle rather than broken; the header already says 0 B/s.
func paintTraffic(p *ui.Painter, r ui.Rect, traffic TrafficState, colors palette) {
	points := min(len(traffic.DownSeries), len(traffic.UpSeries))
	if points < 2 {
		paintTrafficBaseline(p, r, colors.border)
		return
	}
	scale := 0.0
	for i := range points {
		scale = max(scale, traffic.DownSeries[i], traffic.UpSeries[i])
	}
	if scale <= 0 {
		paintTrafficBaseline(p, r, colors.border)
		return
	}
	xs := make([]float32, points)
	down := make([]float32, points)
	up := make([]float32, points)
	for i := range points {
		xs[i] = float32(i) / float32(points-1)
		down[i] = float32(traffic.DownSeries[i] / scale)
		up[i] = float32(traffic.UpSeries[i] / scale)
	}

	// Keep a flat line and a peak inside the card: the stroke is 1.8 DIP wide,
	// and a series that sits at zero would otherwise land on the card's edge.
	plot := ui.Rect{X: r.X, Y: r.Y + 3, W: r.W, H: r.H - 6}
	p.FillPathGradient(chart.Area(plot, xs, nil, down, chart.CurveMonotone), ui.LinearGradient{
		Angle: 180,
		From:  colors.accent.Alpha(0.22),
		To:    colors.accent.Alpha(0),
	})
	p.StrokePath(chart.Line(plot, xs, down, chart.CurveMonotone), 1.8, colors.accent)
	p.StrokePath(chart.Line(plot, xs, up, chart.CurveMonotone), 1.6, colors.success)
}

// paintTrafficBaseline draws the idle card's flat line, in the same inset the
// series use.
func paintTrafficBaseline(p *ui.Painter, r ui.Rect, color ui.Color) {
	p.Line(r.X, r.Y+r.H-3, r.X+r.W, r.Y+r.H-3, 1, color)
}
