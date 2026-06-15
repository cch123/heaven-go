package fruitbasket

import (
	"math"

	"hsdemo/kart"
)

type scoreParticle struct {
	sprite string
	x, y   float64
	vx, vy float64
	born   float64
	life   float64
	size   float64
	spin   float64
	order  int
	tint   [4]float64
}

func (m *Module) spawnScoreParticles(hoop string, fruit int, beat float64) {
	sprite, tint := scoreParticleStyle(fruit)
	base := nodeWorldPos(m.ctx, hoop)
	if p := nodeWorldPos(m.ctx, hoop+"/AppleScore"); p != ([2]float64{}) {
		base = p
	}
	if fruit == fruitLemon {
		if p := nodeWorldPos(m.ctx, hoop+"/LemonScore"); p != ([2]float64{}) {
			base = p
		}
	}
	if fruit == fruitMelon {
		if p := nodeWorldPos(m.ctx, hoop+"/MelonScore"); p != ([2]float64{}) {
			base = p
		}
	}
	born := m.ctx.BeatToTime(beat)
	for i := 0; i < 12; i++ {
		r0 := eventRand(beat, i*5+0)
		r1 := eventRand(beat, i*5+1)
		r2 := eventRand(beat, i*5+2)
		r3 := eventRand(beat, i*5+3)
		r4 := eventRand(beat, i*5+4)
		angle := (float64(i)/12)*math.Pi*2 + r0*0.24
		speed := 2.0 + r1*1.8
		m.particles = append(m.particles, scoreParticle{
			sprite: sprite,
			x:      base[0],
			y:      base[1],
			vx:     math.Cos(angle) * speed,
			vy:     math.Sin(angle)*speed + 1.2,
			born:   born,
			life:   0.42 + r2*0.18,
			size:   0.16 + r3*0.10,
			spin:   (r4*2 - 1) * 12,
			order:  12,
			tint:   tint,
		})
	}
}

func (m *Module) updateParticles(t float64) {
	out := m.particles[:0]
	for _, p := range m.particles {
		if t-p.born <= p.life {
			out = append(out, p)
		}
	}
	m.particles = out
}

func (m *Module) queueParticles() {
	t := m.ctx.Time()
	for _, p := range m.particles {
		u := clamp01((t - p.born) / p.life)
		alpha := (1 - u) * p.tint[3]
		x := p.x + p.vx*u
		y := p.y + p.vy*u - 1.8*u*u
		s := p.size * (1 + 0.65*u)
		world := kart.Translate(x, y).Mul(kart.Rotate(p.spin * u)).Mul(kart.Scale(s, s))
		tint := p.tint
		tint[3] = alpha
		m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: p.sprite, World: world, Order: p.order, Tint: tint})
	}
}

func scoreParticleStyle(fruit int) (string, [4]float64) {
	switch fruit {
	case fruitLemon:
		return "Main/Lemon", [4]float64{1, 0.95, 0.25, 1}
	case fruitMelon:
		return "Main/Melon", [4]float64{0.35, 1, 0.45, 1}
	default:
		return "Main/Apple", [4]float64{1, 0.38, 0.22, 1}
	}
}
