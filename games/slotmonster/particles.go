package slotmonster

import (
	"math"
	"math/rand"

	"hsdemo/kart"
)

const (
	winParticleLifetime = 5
	winParticleRate     = 13
	winEmitterDuration  = 1.5
	winParticleOrder    = 689
	winParticleSpeed    = 5
)

type winEmitter struct {
	startT  float64
	amount  float64
	speed   float64
	stars   bool
	emitted int
}

type winParticle struct {
	birthT float64
	speed  float64
	x, y   float64
	vx, vy float64
	rot    float64
	rotVel float64
	size   float64
	stars  bool
}

func (m *Module) spawnWinEmitter(beat float64) {
	m.emitters = append(m.emitters, winEmitter{
		startT: m.ctx.BeatToTime(beat), amount: m.particleAmount,
		speed: m.particleSpeed, stars: m.particleStars,
	})
}

func (m *Module) updateParticles(t float64) {
	activeEmitters := m.emitters[:0]
	for i := range m.emitters {
		em := &m.emitters[i]
		if em.speed <= 0 {
			em.speed = 1
		}
		if em.amount <= 0 {
			continue
		}
		elapsed := (t - em.startT) * em.speed
		if elapsed < 0 {
			activeEmitters = append(activeEmitters, *em)
			continue
		}
		want := int(math.Floor(elapsed * winParticleRate * em.amount))
		maxCount := int(math.Ceil(winEmitterDuration * winParticleRate * em.amount))
		if want > maxCount {
			want = maxCount
		}
		for em.emitted < want {
			m.particles = append(m.particles, newWinParticle(m.rng, em.startT+float64(em.emitted)/(winParticleRate*em.amount*em.speed), em.speed, em.stars))
			em.emitted++
		}
		if elapsed < winEmitterDuration || em.emitted < maxCount {
			activeEmitters = append(activeEmitters, *em)
		}
	}
	m.emitters = activeEmitters

	activeParticles := m.particles[:0]
	for _, p := range m.particles {
		if (t-p.birthT)*p.speed <= winParticleLifetime {
			activeParticles = append(activeParticles, p)
		}
	}
	m.particles = activeParticles
}

func newWinParticle(rng *rand.Rand, birth, simSpeed float64, stars bool) winParticle {
	// Prefab ShapeModule: cone, angle 25, radius 1, scale (0.9, 0.35).
	startAngle := rng.Float64() * math.Pi * 2
	startRadius := math.Sqrt(rng.Float64())
	x := math.Cos(startAngle) * startRadius * 0.9
	y := math.Sin(startAngle) * startRadius * 0.35
	dir := (rng.Float64()*2 - 1) * 25 * math.Pi / 180
	vx := math.Sin(dir) * winParticleSpeed
	vy := math.Cos(dir) * winParticleSpeed
	rotVel := 0.0
	if stars {
		rotVel = (rng.Float64()*10 - 5)
	}
	return winParticle{
		birthT: birth, speed: simSpeed, x: x, y: y, vx: vx, vy: vy,
		rot: rng.Float64() * math.Pi * 2, rotVel: rotVel,
		size: 1, stars: stars,
	}
}

func (m *Module) queueParticles(t float64) {
	if len(m.particles) == 0 {
		return
	}
	origin, ok := m.ctx.Scene.NodeWorld(m.coinsRoot)
	if !ok {
		origin = kart.Identity()
	}
	const sprite = "SlotCoinPile"
	scale := slotParticleScale(m.ctx.Assets, sprite)
	for _, p := range m.particles {
		age := (t - p.birthT) * p.speed
		if age < 0 || age > winParticleLifetime {
			continue
		}
		alpha := 1.0
		if age > winParticleLifetime-0.5 {
			alpha = (winParticleLifetime - age) / 0.5
		}
		rot := p.rot + p.rotVel*age
		world := origin.Mul(kart.TRS(p.x+p.vx*age, p.y+p.vy*age, rot, scale*p.size, scale*p.size))
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: sprite, World: world, Order: winParticleOrder,
			Tint: [4]float64{1, 1, 1, alpha},
		})
	}
}

func slotParticleScale(as *kart.Assets, sprite string) float64 {
	sp, ok := as.Sheet.Sprites[sprite]
	if !ok || sp.H == 0 {
		return 0.05
	}
	ppu := sp.PPU
	if ppu == 0 {
		ppu = as.Sheet.PPU
	}
	return 1 / (float64(sp.H) / ppu)
}
