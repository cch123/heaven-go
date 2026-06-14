package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func timingStarSeed(h timingHit, idx int) uint32 {
	bits := math.Float64bits(h.t + h.y*17)
	return uint32(bits) ^ uint32(bits>>32) ^ uint32(idx*0x9e3779b9) ^ uint32(h.rating)*0x85ebca6b
}

func timingRand(seed uint32, salt int) float64 {
	x := seed + uint32(salt)*0x9e3779b9
	x ^= x >> 16
	x *= 0x7feb352d
	x ^= x >> 15
	x *= 0x846ca68b
	x ^= x >> 16
	return float64(x&0xffffff) / float64(0x1000000)
}

func drawTimingParticleSystem(dst *ebiten.Image, assets *timingAccuracyAssets, ps timingParticleSystem, x, y float32, h timingHit, hitIdx, systemIdx int, age, unitPx, parentScale float64) {
	systemAge := age - ps.burstTime
	if systemAge < 0 || systemAge > ps.lifetime {
		return
	}
	p := systemAge / ps.lifetime
	sizeScale := evalTimingCurve(ps.sizeCurve, p)
	if sizeScale <= 0 {
		return
	}
	seed := timingStarSeed(h, hitIdx) ^ uint32(ps.seedSalt*0x45d9f3b)
	globalTime := h.t + age

	for i := 0; i < ps.count; i++ {
		particleSeed := seed ^ uint32(i*0x9e3779b9) ^ uint32(systemIdx*0x85ebca6b)
		angle := timingRand(particleSeed, 1) * math.Pi * 2
		radius := ps.shapeRadius * unitPx
		// TimingAccuracy uses ShapeModule type 10 with radiusThickness=0, so
		// particles start on the ring edge instead of filling the disk.
		dist := radius + ps.startSpeed*systemAge*unitPx
		px := x + float32(math.Cos(angle)*dist)
		py := y + float32(math.Sin(angle)*dist)
		rot := float32(0)
		if ps.randomRotation {
			rot = float32(timingRand(particleSeed, 3) * math.Pi * 2)
		}
		rot += float32(ps.angularVelocity * systemAge)
		size := float32(ps.startSize * sizeScale * parentScale * unitPx)
		if size < 0.5 {
			continue
		}
		c := timingParticleColor(assets, ps, particleSeed, p, globalTime)
		if img := assets.image(ps.texture); img != nil {
			drawTimingSprite(dst, img, px, py, size, rot, c)
		} else {
			drawTimingQuad(dst, px, py, size, rot, c)
		}
	}
}

func evalTimingCurve(keys []timingCurveKey, t float64) float64 {
	if len(keys) == 0 {
		return 1
	}
	if t <= keys[0].t {
		return keys[0].v
	}
	for i := 1; i < len(keys); i++ {
		prev, next := keys[i-1], keys[i]
		if t <= next.t {
			u := (t - prev.t) / (next.t - prev.t)
			return prev.v + (next.v-prev.v)*u
		}
	}
	return keys[len(keys)-1].v
}

func timingParticleColor(assets *timingAccuracyAssets, ps timingParticleSystem, seed uint32, lifeP, globalTime float64) color.RGBA {
	c := ps.minColor
	if ps.randomColor {
		c = lerpRGBA(ps.minColor, ps.maxColor, timingRand(seed, 4))
	}
	alpha := (float64(c.A) / 255) * (0.5 + 0.5*lifeP)
	if ps.material == timingMaterialAce {
		grayscale := (float64(c.R) + float64(c.G) + float64(c.B)) / (255 * 3)
		c = timingSampleAceColor(assets.aceColors, grayscale+2.5*globalTime)
	}
	c.A = uint8(math.Max(0, math.Min(255, alpha*255)))
	return c
}

func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: uint8(float64(a.A) + (float64(b.A)-float64(a.A))*t),
	}
}

func timingSampleAceColor(colors []color.RGBA, t float64) color.RGBA {
	if len(colors) == 0 {
		return color.RGBA{255, 245, 80, 255}
	}
	t -= math.Floor(t)
	pos := t * float64(len(colors)-1)
	i := int(pos)
	if i >= len(colors)-1 {
		return colors[len(colors)-1]
	}
	return lerpRGBA(colors[i], colors[i+1], pos-float64(i))
}

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
