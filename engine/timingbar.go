// timingbar.go：时机条 overlay（对应 HS TimingAccuracyDisplay 的
// "旋转 90°、半透明、置于底部中央"布局）：横向，中心 = 完美，
// 左 = 早（fast）、右 = 晚（slow）；三段色区比例 = 各判定窗口。
package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	timingBarHalfW          = float32(105) // screen-space equivalent of the prefab bar's +/-WinNG range.
	timingUnityBarHalfUnits = 1.4          // TimingAccuracy.prefab barTransform.localScale.y is the full 2.8-unit bar.
	timingBarAceNorm        = 0.111        // barJustTransform.localScale.y.
	timingBarOKNorm         = 0.5714286    // barOKTransform.localScale.y.
)

func (a *App) drawTimingBar(screen *ebiten.Image, t float64) {
	const (
		cx    = float32(ScreenW / 2)
		cy    = float32(508)
		halfW = timingBarHalfW
		bh    = float32(10)
	)
	justW := halfW * timingBarOKNorm
	aceW := halfW * timingBarAceNorm

	vector.DrawFilledRect(screen, cx-halfW-14, cy-bh/2-5, (halfW+14)*2, bh+10, color.RGBA{30, 30, 34, 110}, false)
	vector.DrawFilledRect(screen, cx-halfW, cy-bh/2, halfW*2, bh, color.RGBA{130, 60, 60, 150}, false)
	vector.DrawFilledRect(screen, cx-justW, cy-bh/2, justW*2, bh, color.RGBA{150, 130, 60, 170}, false)
	vector.DrawFilledRect(screen, cx-aceW, cy-bh/2, aceW*2+1, bh, color.RGBA{90, 200, 120, 220}, false)
	vector.StrokeLine(screen, cx, cy-bh/2-2, cx, cy+bh/2+2, 1, color.RGBA{255, 255, 255, 200}, false)

	dim := color.RGBA{230, 230, 235, 150}
	a.text(screen, "fast", a.faceSmall, float64(cx-halfW-14)-32, float64(cy)-9, dim, false)
	a.text(screen, "slow", a.faceSmall, float64(cx+halfW+14)+6, float64(cy)-9, dim, false)

	for i, h := range a.tdHits {
		age := t - h.t
		if age > 1.2 {
			continue
		}
		x := cx + float32(h.y)*halfW
		a.drawTimingHitStars(screen, x, cy, h, i, age)

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

func (a *App) drawTimingHitStars(dst *ebiten.Image, x, y float32, h timingHit, idx int, age float64) {
	assets := timingAccuracyImages(a.assetsRoot)
	unitPx := float64(timingBarHalfW) / timingUnityBarHalfUnits
	scale := timingRatingBaseScale(h.rating)
	if h.rating == JudgeJust {
		scale *= timingOKScale(h)
	}
	for si, ps := range timingParticleSystems(h.rating) {
		drawTimingParticleSystem(dst, assets, ps, x, y, h, idx, si, age, unitPx, scale)
	}
}

func timingBarNorm(signed float64) float64 {
	d := math.Abs(signed)
	sign := 1.0
	if signed < 0 {
		sign = -1
	}
	switch {
	case d <= WinAce:
		return sign * (d / WinAce) * timingBarAceNorm
	case d <= WinJust:
		frac := (d - WinAce) / (WinJust - WinAce)
		return sign * (timingBarAceNorm + (timingBarOKNorm-timingBarAceNorm)*frac)
	default:
		frac := (d - WinJust) / (WinNG - WinJust)
		if frac > 1 {
			frac = 1
		}
		return sign * (timingBarOKNorm + (1-timingBarOKNorm)*frac)
	}
}

func timingRatingBaseScale(j Judgment) float64 {
	if j == JudgeNG {
		return 0.6320368
	}
	return 1
}

func timingOKScale(h timingHit) float64 {
	signed := h.signed
	if math.Abs(signed) <= WinAce {
		return 1
	}
	// TimingAccuracyDisplay.MakeAccuracyVfx scales the OK object down as the hit
	// moves through the OK band; keep the original early/late frac asymmetry.
	var frac float64
	if signed > 0 {
		frac = (signed - WinAce) / (WinJust - WinAce)
	} else {
		frac = (signed + WinJust) / (WinJust - WinAce)
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return 1 - frac/2
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
