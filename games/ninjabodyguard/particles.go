package ninjabodyguard

import (
	"image"
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kart"
)

const (
	ninjaHitParticleSprite     = "__ninja_hit_particle"
	ninjaHitParticlePPU        = 100
	ninjaHitParticleVisibleSec = ninjaHitParticleLifetimeSec / ninjaHitParticleSimulationSpeed

	// ArrowSliceA/B are authored as two one-shot ParticleSystems in the prefab.
	// These constants mirror the serialized Initial/Shape/Emission/Force and
	// ParticleSystemRenderer fields so the hit flash is driven from the same
	// particle semantics instead of a fixed hand-drawn slash.
	ninjaHitParticleLifetimeSec     = 3.5
	ninjaHitParticleSimulationSpeed = 3
	ninjaHitParticleStartSize       = 1.05
	ninjaHitParticleSpeedMin        = 6
	ninjaHitParticleSpeedMax        = 8
	ninjaHitParticleShapeAngleDeg   = 30
	ninjaHitParticleShapeArcDeg     = 10
	ninjaHitParticleShapeRadius     = 0.75
	ninjaHitParticleShapeYOffset    = -0.5
	ninjaHitParticleBurstCount      = 1
	ninjaHitParticleForceY          = -10
	ninjaHitParticleLengthScale     = 2
	ninjaHitParticleOrder           = 50
)

type ninjaHitEmitter struct {
	name        string
	shapeRotDeg float64
	seedSalt    int64
}

var ninjaHitParticleEmitters = []ninjaHitEmitter{
	{name: "ArrowSliceA", shapeRotDeg: 60, seedSalt: 0x45a},
	{name: "ArrowSliceB", shapeRotDeg: 110, seedSalt: 0x45b},
}

type hitSpark struct {
	beat      float64
	time      float64
	particles []ninjaHitParticle
}

type ninjaHitParticle struct {
	emitter int
	offset  [2]float64
	dir     float64
	speed   float64
	roll    float64
}

func registerNinjaHitParticleSprite(as *kart.Assets) {
	img := image.NewNRGBA(image.Rect(0, 0, 14, 96))
	b := img.Bounds()
	cx := float64(b.Dx()-1) * 0.5
	for y := b.Min.Y; y < b.Max.Y; y++ {
		vy := math.Abs((float64(y) - float64(b.Dy())*0.5) / (float64(b.Dy()) * 0.5))
		taper := 1 - math.Pow(vy, 1.8)
		for x := b.Min.X; x < b.Max.X; x++ {
			vx := math.Abs(float64(x)-cx) / cx
			alpha := clamp01(taper) * clamp01(1-vx*vx)
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: uint8(alpha * 255)})
		}
	}
	as.RegisterSprite(ninjaHitParticleSprite, ebiten.NewImageFromImage(img), ninjaHitParticlePPU, 0.5, 0.5)
}

func newNinjaHitSpark(beat, t float64) hitSpark {
	sp := hitSpark{beat: beat, time: t}
	for ei, em := range ninjaHitParticleEmitters {
		seed := int64(math.Float64bits(beat)) ^ em.seedSalt
		rng := rand.New(rand.NewSource(seed))
		for i := 0; i < ninjaHitParticleBurstCount; i++ {
			arc := degToRad(em.shapeRotDeg + lerp(-ninjaHitParticleShapeArcDeg/2, ninjaHitParticleShapeArcDeg/2, rng.Float64()))
			radius := ninjaHitParticleShapeRadius * math.Sqrt(rng.Float64())
			offAngle := degToRad(em.shapeRotDeg + lerp(-ninjaHitParticleShapeArcDeg/2, ninjaHitParticleShapeArcDeg/2, rng.Float64()))
			sp.particles = append(sp.particles, ninjaHitParticle{
				emitter: ei,
				offset: [2]float64{
					math.Cos(offAngle) * radius,
					ninjaHitParticleShapeYOffset + math.Sin(offAngle)*radius,
				},
				dir:   arc,
				speed: lerp(ninjaHitParticleSpeedMin, ninjaHitParticleSpeedMax, rng.Float64()),
				roll:  lerp(math.Pi/4, math.Pi, rng.Float64()),
			})
		}
	}
	return sp
}

func ninjaHitParticleSprites(sp hitSpark, base kart.Aff, t float64) []kart.ExtraSprite {
	age := (t - sp.time) * ninjaHitParticleSimulationSpeed
	if age < 0 || age > ninjaHitParticleLifetimeSec {
		return nil
	}
	out := make([]kart.ExtraSprite, 0, len(sp.particles))
	for _, p := range sp.particles {
		vx := math.Cos(p.dir) * p.speed
		vy := math.Sin(p.dir) * p.speed
		x := p.offset[0] + vx*age
		y := p.offset[1] + vy*age + 0.5*ninjaHitParticleForceY*age*age
		// Renderer.rotateWithStretchDirection aligns the particle's long axis
		// with velocity; roll keeps the authored random start rotation visible.
		rot := math.Atan2(vy+ninjaHitParticleForceY*age, vx) - math.Pi/2 + p.roll*0.08
		stretch := 1 + p.speed*ninjaHitParticleLengthScale*0.08
		alpha := 1.0
		if age > ninjaHitParticleLifetimeSec*0.9 {
			alpha = 1 - (age-ninjaHitParticleLifetimeSec*0.9)/(ninjaHitParticleLifetimeSec*0.1)
		}
		out = append(out, kart.ExtraSprite{
			Sprite: ninjaHitParticleSprite,
			World:  base.Mul(kart.TRS(x, y, rot, ninjaHitParticleStartSize, ninjaHitParticleStartSize*stretch)),
			Layer:  0,
			Order:  ninjaHitParticleOrder + p.emitter,
			Tint:   [4]float64{1, 1, 1, alpha},
		})
	}
	return out
}

func degToRad(v float64) float64 { return v * math.Pi / 180 }

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
