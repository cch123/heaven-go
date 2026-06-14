package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// viewScaleAt 折叠 vfx/scale view 事件得到画布缩放（StaticCamera.UpdateScale：
// 进行中的事件从上一事件终值缓动到自身目标）。
func (a *App) viewScaleAt(beat float64) (float64, float64) {
	sx, sy := 1.0, 1.0
	lx, ly := 1.0, 1.0
	for _, e := range a.viewScales {
		if beat < e.beat {
			continue
		}
		prog := 1.0
		if e.length > 0 {
			prog = math.Min((beat-e.beat)/e.length, 1)
		}
		switch e.axis {
		case 1:
			sx = Ease(e.ease, lx, e.x, prog)
		case 2:
			sy = Ease(e.ease, ly, e.y, prog)
		default:
			sx = Ease(e.ease, lx, e.x, prog)
			sy = Ease(e.ease, ly, e.y, prog)
		}
		if prog >= 1 {
			switch e.axis {
			case 1:
				lx = e.x
			case 2:
				ly = e.y
			default:
				lx, ly = e.x, e.y
			}
		}
	}
	return sx, sy
}

// drawFlash：vfx/flash 是单一覆盖层（HS Fade 语义）——按拍序折叠，
// 最后一个已开始的事件决定当前颜色（事件结束后停在其终色），
// 不能把多个事件叠画（先前事件的不透明终色会永久压住画面）。
func (a *App) drawFlash(screen *ebiten.Image, beat float64) {
	var c [4]float64
	hit := false
	for _, f := range a.flashes {
		if beat < f.beat || f.length <= 0 {
			continue
		}
		u := math.Min((beat-f.beat)/f.length, 1)
		for i := range c {
			c[i] = f.c0[i] + (f.c1[i]-f.c0[i])*u
		}
		hit = true
	}
	if hit && c[3] > 0 {
		vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{
			uint8(c[0] * 255 * c[3]), uint8(c[1] * 255 * c[3]), uint8(c[2] * 255 * c[3]), uint8(c[3] * 255),
		}, false)
	}
}
