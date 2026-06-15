package engine

import (
	"image/color"
	"math"
)

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
