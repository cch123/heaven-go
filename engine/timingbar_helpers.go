package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (td *timingDisplayState) drawHitStars(dst *ebiten.Image, x, y float32, h timingHit, idx int, age float64, assetsRoot string) {
	assets := timingAccuracyImages(assetsRoot)
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
