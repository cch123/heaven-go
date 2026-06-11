// timingbar.go：时机条 overlay（对应 HS TimingAccuracyDisplay 的
// "旋转 90°、半透明、置于底部中央"布局）：横向，中心 = 完美，
// 左 = 早（fast）、右 = 晚（slow）；三段色区比例 = 各判定窗口。
package engine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (a *App) drawTimingBar(screen *ebiten.Image, t float64) {
	const (
		cx    = float32(ScreenW / 2)
		cy    = float32(508)
		halfW = float32(105) // = WinNG
		bh    = float32(10)
	)
	justW := halfW * float32(WinJust/WinNG)
	aceW := halfW * float32(WinAce/WinNG)

	vector.DrawFilledRect(screen, cx-halfW-14, cy-bh/2-5, (halfW+14)*2, bh+10, color.RGBA{30, 30, 34, 110}, false)
	vector.DrawFilledRect(screen, cx-halfW, cy-bh/2, halfW*2, bh, color.RGBA{130, 60, 60, 150}, false)
	vector.DrawFilledRect(screen, cx-justW, cy-bh/2, justW*2, bh, color.RGBA{150, 130, 60, 170}, false)
	vector.DrawFilledRect(screen, cx-aceW, cy-bh/2, aceW*2+1, bh, color.RGBA{90, 200, 120, 220}, false)
	vector.StrokeLine(screen, cx, cy-bh/2-2, cx, cy+bh/2+2, 1, color.RGBA{255, 255, 255, 200}, false)

	dim := color.RGBA{230, 230, 235, 150}
	a.text(screen, "fast", a.faceSmall, float64(cx-halfW-14)-32, float64(cy)-9, dim, false)
	a.text(screen, "slow", a.faceSmall, float64(cx+halfW+14)+6, float64(cy)-9, dim, false)

	for _, h := range a.tdHits {
		age := t - h.t
		if age > 1.2 {
			continue
		}
		al := 1 - age/1.2
		var c color.RGBA
		switch h.rating {
		case JudgeAce:
			c = color.RGBA{140, 255, 170, uint8(255 * al)}
		case JudgeJust:
			c = color.RGBA{255, 230, 130, uint8(255 * al)}
		default:
			c = color.RGBA{255, 120, 120, uint8(255 * al)}
		}
		x := cx + float32(h.y)*halfW
		half := float32(2 + 4*al)
		vector.DrawFilledRect(screen, x-1.5, cy-bh/2-half, 3, bh+half*2, c, true)
		if h.rating == JudgeNG && age < 0.7 {
			label := "LATE"
			if h.y < 0 {
				label = "EARLY"
			}
			a.text(screen, label, a.faceSmall, float64(x)-18, float64(cy)-36, c, false)
		}
	}

	ax := cx + float32(a.tdArrow)*halfW
	drawTri(screen, ax, cy-bh/2-5, 5, true)
	drawTri(screen, ax, cy+bh/2+5, 5, false)
}

// drawTri 画一个指向时机条的小三角（down=true 表示顶点朝下）。
func drawTri(dst *ebiten.Image, x, y, r float32, down bool) {
	dir := float32(1)
	if !down {
		dir = -1
	}
	var p vector.Path
	p.MoveTo(x-r, y-dir*r)
	p.LineTo(x+r, y-dir*r)
	p.LineTo(x, y+dir*r)
	p.Close()
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].ColorR, vs[i].ColorG, vs[i].ColorB, vs[i].ColorA = 1, 1, 1, 0.9
	}
	dst.DrawTriangles(vs, is, whitePixel, &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

var whitePixel = func() *ebiten.Image {
	img := ebiten.NewImage(3, 3)
	img.Fill(color.White)
	return img
}()
