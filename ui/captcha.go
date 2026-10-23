// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Point is one click in the captcha image's own pixel coordinates. The
// gateway expects the image's coordinates, not the screen's, so the view
// converts before handing a point to the application.
type Point struct {
	X int
	Y int
}

// Captcha is a click captcha the gateway is waiting on: the image it sent and
// the points the user has picked so far.
type Captcha struct {
	Image *ui.Bitmap
	// Width and Height are the image's pixel size. The gateway needs them
	// with the coordinates because the click captcha scales the picture.
	Width  int
	Height int
	// Points are in image pixels, in the order the user clicked them.
	Points []Point
}

// The captcha is drawn large enough to aim at but never taller than this.
const (
	captchaMaxWidth  float32 = 440
	captchaMaxHeight float32 = 200
)

// DisplaySize returns the size in DIPs the view draws the image at. The
// aspect ratio is preserved; a small image is enlarged, as the reference
// implementation does.
func (c *Captcha) DisplaySize() (float32, float32) {
	if c.Width <= 0 || c.Height <= 0 {
		return 0, 0
	}
	scale := min(captchaMaxWidth/float32(c.Width), captchaMaxHeight/float32(c.Height))
	return float32(c.Width) * scale, float32(c.Height) * scale
}

// PointAt maps a click measured in display DIPs to the image pixel under it.
// It reports false for a click outside the picture.
func (c *Captcha) PointAt(x, y float32) (Point, bool) {
	width, height := c.DisplaySize()
	if width <= 0 || height <= 0 {
		return Point{}, false
	}
	if x < 0 || y < 0 || x > width || y > height {
		return Point{}, false
	}
	px := int(math.Round(float64(x) * float64(c.Width) / float64(width)))
	py := int(math.Round(float64(y) * float64(c.Height) / float64(height)))
	return Point{X: clamp(px, 0, c.Width-1), Y: clamp(py, 0, c.Height-1)}, true
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
