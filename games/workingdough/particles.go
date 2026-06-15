package workingdough

import (
	"math"
	"math/rand"

	"hsdemo/kart"
)

type breakBurst struct {
	beat      float64
	origin    [2]float64
	particles []breakParticle
}

type breakParticle struct {
	sprite       string
	angle, speed float64
	spin         float64
	size         float64
	life         float64
}

func newBreakBurst(beat float64, origin [2]float64) breakBurst {
	// BreakingParticle is a Unity ParticleSystem, so the extractor only keeps an
	// empty prefab root. Mirror the serialized burst in code: 0.05s emission,
	// roughly 2s lifetime, 9-11 unit start speed, and a narrow cone.
	r := rand.New(rand.NewSource(int64(math.Round(beat*1000)) ^ 0x5eed))
	b := breakBurst{beat: beat, origin: origin}
	for i := 0; i < 12; i++ {
		a := -math.Pi/2 + (r.Float64()-0.5)*(50*math.Pi/180)
		if i%2 == 1 {
			a += math.Pi
		}
		sprite := "BallBreak0"
		if i%3 == 0 {
			sprite = "Ballbreak1"
		}
		b.particles = append(b.particles, breakParticle{
			sprite: sprite,
			angle:  a,
			speed:  9 + r.Float64()*2,
			spin:   (r.Float64()*2 - 1) * math.Pi * 2,
			size:   0.2 + r.Float64()*0.18,
			life:   1.1 + r.Float64()*0.5,
		})
	}
	return b
}

func (b breakBurst) alive(beat float64) bool {
	return beat-b.beat < 1.6
}

func (b breakBurst) queue(scene *kart.SceneInst, beat float64) {
	t := math.Max(0, beat-b.beat)
	for _, p := range b.particles {
		if t > p.life {
			continue
		}
		x := b.origin[0] + math.Cos(p.angle)*p.speed*t
		y := b.origin[1] + math.Sin(p.angle)*p.speed*t - 4.9*t*t
		alpha := 1 - t/p.life
		scene.Queue(kart.ExtraSprite{
			Sprite: p.sprite,
			World:  kart.TRS(x, y, p.spin*t, p.size, p.size),
			Order:  1,
			Tint:   [4]float64{1, 1, 1, alpha},
		})
	}
}
