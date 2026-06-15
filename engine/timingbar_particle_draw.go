package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func drawTimingSprite(dst, img *ebiten.Image, x, y, height, rot float32, c color.RGBA) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return
	}
	scale := float64(height) / float64(h)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Rotate(float64(rot))
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(c)
	dst.DrawImage(img, op)
}

func drawTimingQuad(dst *ebiten.Image, x, y, size, rot float32, c color.RGBA) {
	half := size / 2
	cosR, sinR := float32(math.Cos(float64(rot))), float32(math.Sin(float64(rot)))
	corners := [4][2]float32{{-half, -half}, {half, -half}, {half, half}, {-half, half}}
	var p vector.Path
	for i, corner := range corners {
		px := x + corner[0]*cosR - corner[1]*sinR
		py := y + corner[0]*sinR + corner[1]*cosR
		if i == 0 {
			p.MoveTo(px, py)
		} else {
			p.LineTo(px, py)
		}
	}
	p.Close()
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	cr, cg, cb, ca := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255
	for i := range vs {
		vs[i].ColorR, vs[i].ColorG, vs[i].ColorB, vs[i].ColorA = cr, cg, cb, ca
	}
	dst.DrawTriangles(vs, is, whitePixel, &ebiten.DrawTrianglesOptions{AntiAlias: true})
}
