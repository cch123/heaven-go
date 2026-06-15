package engine

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

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
