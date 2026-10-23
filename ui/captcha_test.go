package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
)

func testBitmap(t *testing.T, width, height int) *ui.Bitmap {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		for y := range height {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("encode the test image: %v", err)
	}
	bitmap, err := ui.DecodeBitmap(encoded.Bytes())
	if err != nil {
		t.Fatalf("decode the test image: %v", err)
	}
	return bitmap
}

// A click on the displayed image must reach the gateway in the image's own
// pixels, whatever size the view chose to draw it at.
func TestCaptchaPointAtMapsToImagePixels(t *testing.T) {
	captcha := &Captcha{Width: 100, Height: 50}
	width, height := captcha.DisplaySize()
	if width != 400 || height != 200 {
		t.Fatalf("display size = %vx%v, want 400x200", width, height)
	}

	point, ok := captcha.PointAt(width/2, height/2)
	if !ok {
		t.Fatal("the centre of the image was rejected")
	}
	if point != (Point{X: 50, Y: 25}) {
		t.Fatalf("centre point = %+v, want {50 25}", point)
	}

	if _, ok := captcha.PointAt(width+1, height/2); ok {
		t.Fatal("a click past the right edge was accepted")
	}
	if _, ok := captcha.PointAt(-1, height/2); ok {
		t.Fatal("a click past the left edge was accepted")
	}
}

// The numbered markers show what will be submitted; the buttons must reach
// their actions, and an empty submission stays disabled.
func TestCaptchaPanelShowsPointsAndCallsActions(t *testing.T) {
	var added []Point
	var undone, cleared, submitted, cancelled int

	state := State{
		Session: session.State{Phase: session.PhaseConnecting, Detail: "logging in"},
		Config:  config.Default(),
		Captcha: &Captcha{
			Image:  testBitmap(t, 100, 50),
			Width:  100,
			Height: 50,
			Points: []Point{{X: 10, Y: 10}, {X: 90, Y: 40}},
		},
	}
	actions := Actions{
		AddCaptchaPoint: func(point Point) { added = append(added, point) },
		UndoCaptcha:     func() { undone++ },
		ClearCaptcha:    func() { cleared++ },
		SubmitCaptcha:   func() { submitted++ },
		CancelCaptcha:   func() { cancelled++ },
	}

	tester := ui.NewTester(View(&state, actions), 520, 560)
	tester.Frame()

	if !tester.HasText("已点击 2 个位置") {
		t.Fatal("the picked point count is missing")
	}
	if !tester.HasText("1") || !tester.HasText("2") {
		t.Fatal("the numbered markers are missing")
	}

	rect, ok := tester.Find("图形验证码图片")
	if !ok {
		t.Fatal("the captcha image is not in the view")
	}
	tester.ClickAt(rect.X+rect.W/4, rect.Y+rect.H/2)
	if len(added) != 1 {
		t.Fatalf("image click added %d points, want 1", len(added))
	}
	if added[0] != (Point{X: 25, Y: 25}) {
		t.Fatalf("image click mapped to %+v, want {25 25}", added[0])
	}

	for button, count := range map[string]*int{"撤销": &undone, "清空": &cleared, "提交": &submitted, "取消": &cancelled} {
		if err := tester.Click(button); err != nil {
			t.Fatalf("click %q: %v", button, err)
		}
		if *count != 1 {
			t.Fatalf("%q called its action %d times, want 1", button, *count)
		}
	}

	empty := state
	empty.Captcha = &Captcha{Image: testBitmap(t, 100, 50), Width: 100, Height: 50}
	submitted = 0
	tester = ui.NewTester(View(&empty, actions), 520, 560)
	tester.Frame()
	if err := tester.Click("提交"); err != nil {
		t.Fatalf("click 提交: %v", err)
	}
	if submitted != 0 {
		t.Fatal("an empty captcha could be submitted")
	}
}
