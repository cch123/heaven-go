package shootemup

import (
	"math"

	"hsdemo/kart"
)

type particle struct {
	sprite     string
	start, dur float64
	pos, vel   vec2
	scale0     float64
	scale1     float64
	rot, spin  float64
	color      [4]float64
	order      int
}

func (m *Module) spawnHitParticles(pos vec2, beat float64) {
	// The original HitParticle prefab is a child ParticleSystem tree. The scene
	// extractor preserves only anchors, so the burst below mirrors its visible
	// pieces: expanding ring, sparkle flashes, and small debris chips.
	m.particles = append(m.particles,
		particle{sprite: "shoot_ring", start: beat, dur: 0.8, pos: pos, scale0: 0.35, scale1: 1.25, color: [4]float64{1, 1, 1, 0.9}, order: 260},
		particle{sprite: "shoot_sparkle", start: beat, dur: 0.55, pos: pos, scale0: 0.45, scale1: 0.85, rot: deg(15), spin: deg(160), color: white, order: 262},
		particle{sprite: "shoot_sparkle", start: beat + 0.05, dur: 0.45, pos: vec2{pos.x + 0.12, pos.y + 0.03}, scale0: 0.3, scale1: 0.65, rot: deg(80), spin: deg(-120), color: white, order: 262},
	)
	for i := 0; i < 8; i++ {
		ang := float64(i) * math.Pi / 4
		speed := 0.55 + float64(i%3)*0.12
		m.particles = append(m.particles, particle{
			sprite: fmtPiece(i), start: beat, dur: 0.75,
			pos: pos, vel: vec2{math.Cos(ang) * speed, math.Sin(ang) * speed},
			scale0: 0.32, scale1: 0.18,
			rot: ang, spin: deg(90 + float64(i)*25),
			color: white, order: 261,
		})
	}
}

func (m *Module) spawnSmoke(pos vec2, beat float64) {
	for i := 0; i < 5; i++ {
		ang := deg(-130 + float64(i)*65)
		m.particles = append(m.particles, particle{
			sprite: fmtSmoke(i), start: beat + float64(i)*0.03, dur: 1.2,
			pos: pos, vel: vec2{math.Cos(ang) * 0.25, math.Sin(ang)*0.22 + 0.15},
			scale0: 0.25, scale1: 0.8,
			rot: ang, spin: deg(25 + float64(i)*8),
			color: [4]float64{1, 1, 1, 0.65}, order: 255,
		})
	}
}

func (m *Module) drawParticles(beat float64) {
	alive := m.particles[:0]
	for _, p := range m.particles {
		u := (beat - p.start) / p.dur
		if u < 0 {
			alive = append(alive, p)
			continue
		}
		if u > 1 {
			continue
		}
		scale := p.scale0 + (p.scale1-p.scale0)*u
		col := p.color
		col[3] *= 1 - u
		world := kart.Translate(p.pos.x+p.vel.x*u, p.pos.y+p.vel.y*u).
			Mul(kart.Rotate(p.rot + p.spin*u)).
			Mul(kart.Scale(scale, scale))
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: p.sprite, World: world, Order: p.order, Tint: col,
		})
		alive = append(alive, p)
	}
	m.particles = alive
}

func fmtPiece(i int) string { return "shoot_piece" + itoa(1+i%12) }
func fmtSmoke(i int) string { return "smoke" + itoa(1+i%8) }

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return "10"
}
