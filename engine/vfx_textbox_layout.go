package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// anchorPos 返回原版 VFXObject.TextboxAnchor 坐标：XAnchor=3, YAnchor=3.5；
// Custom 用 x/y，单位与游戏世界一致 54px/unit。
func anchorPos(anchor int, x, y float64) (float64, float64) {
	cx, cy := float64(ScreenW/2), float64(ScreenH/2)
	xAnchor, yAnchor := 3*54.0, 3.5*54.0
	top, mid, bot := cy-yAnchor, cy, cy+yAnchor
	lft, ctr, rgt := cx-xAnchor, cx, cx+xAnchor
	switch anchor {
	case 0:
		return lft, top
	case 1:
		return ctr, top
	case 2:
		return rgt, top
	case 3:
		return lft, mid
	case 4:
		return ctr, mid
	case 5:
		return rgt, mid
	case 6:
		return lft, bot
	case 7:
		return ctr, bot
	case 8:
		return rgt, bot
	default: // Custom
		return cx + x*54, cy - y*54
	}
}

func drawTextboxPanel(dst *ebiten.Image, x, y, w, h float64) {
	r := math.Min(h*0.42, 34)
	drawRoundedRect(dst, x, y, w, h, r+7, color.RGBA{0, 0, 0, 245})
	drawRoundedRect(dst, x+6, y+6, w-12, h-12, r, color.RGBA{255, 255, 255, 245})
}

func drawRoundedRect(dst *ebiten.Image, x, y, w, h, r float64, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	if r > w/2 {
		r = w / 2
	}
	if r > h/2 {
		r = h / 2
	}
	fx, fy, fw, fh := float32(x), float32(y), float32(w), float32(h)
	fr := float32(r)
	vector.DrawFilledRect(dst, fx+fr, fy, fw-2*fr, fh, c, true)
	vector.DrawFilledRect(dst, fx, fy+fr, fw, fh-2*fr, c, true)
	vector.DrawFilledCircle(dst, fx+fr, fy+fr, fr, c, true)
	vector.DrawFilledCircle(dst, fx+fw-fr, fy+fr, fr, c, true)
	vector.DrawFilledCircle(dst, fx+fr, fy+fh-fr, fr, c, true)
	vector.DrawFilledCircle(dst, fx+fw-fr, fy+fh-fr, fr, c, true)
}
