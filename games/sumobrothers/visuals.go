package sumobrothers

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (m *Module) setBackgroundColor(ev bgColorEvt) {
	m.bgRun = colorRun{bgColorEvt: ev, active: true}
}

func (m *Module) setMawashiColor(left, right [4]float64) {
	m.mawashiLeft = left
	m.mawashiRight = right
}

func (m *Module) persistColors(beat float64) {
	for _, ev := range m.bgColors {
		if ev.beat >= beat {
			break
		}
		m.setBackgroundColor(ev)
	}
	for _, ev := range m.mawashis {
		if ev.beat >= beat {
			break
		}
		m.setMawashiColor(ev.left, ev.right)
	}
}

func (m *Module) startStompShake(beat float64) {
	speed := m.shakeSpeed
	if speed <= 0 {
		speed = 0.125
	}
	vals := []float64{-0.3, 0.3, -0.2, 0.2, -0.1, 0.1, -0.1, 0}
	m.shakeKeys = m.shakeKeys[:0]
	for i, x := range vals {
		m.shakeKeys = append(m.shakeKeys, shakeKey{beat: beat + speed*float64(i), x: x})
	}
}

func (m *Module) cameraShakeAt(beat float64) float64 {
	if len(m.shakeKeys) == 0 {
		return 0
	}
	if beat < m.shakeKeys[0].beat {
		return 0
	}
	for i := 0; i+1 < len(m.shakeKeys); i++ {
		a, b := m.shakeKeys[i], m.shakeKeys[i+1]
		if beat >= a.beat && beat <= b.beat {
			u := 0.0
			if b.beat > a.beat {
				u = (beat - a.beat) / (b.beat - a.beat)
			}
			return easeInOutQuad(a.x, b.x, u)
		}
	}
	last := m.shakeKeys[len(m.shakeKeys)-1]
	if beat <= last.beat+m.shakeSpeed {
		return last.x
	}
	return 0
}

func easeInOutQuad(a, b, u float64) float64 {
	u = clamp01(u)
	if u < 0.5 {
		u = 2 * u * u
	} else {
		u = 1 - math.Pow(-2*u+2, 2)/2
	}
	return a + (b-a)*u
}

func (m *Module) spawnConfetti(beat float64) {
	cols := [][4]float64{
		{1, 0.18, 0.24, 1}, {1, 0.9, 0.1, 1}, {0.2, 0.55, 1, 1},
		{0.2, 1, 0.45, 1}, {1, 0.3, 0.9, 1}, {1, 1, 1, 1},
	}
	for i := 0; i < 72; i++ {
		side := -1.0
		if i%2 == 0 {
			side = 1
		}
		ang := -math.Pi/2 + (m.rng.Float64()-0.5)*1.55
		speed := 3.2 + m.rng.Float64()*4.0
		p := confettiParticle{
			born: beat, life: 1.25 + m.rng.Float64()*0.8,
			x:   side * (1.1 + m.rng.Float64()*0.45),
			y:   -0.25 + m.rng.Float64()*0.45,
			vx:  math.Cos(ang)*speed + side*(0.6+m.rng.Float64()*1.2),
			vy:  -math.Sin(ang)*speed + 2.8,
			rot: m.rng.Float64() * math.Pi,
			vr:  (m.rng.Float64() - 0.5) * 12,
			w:   0.045 + m.rng.Float64()*0.025,
			h:   0.11 + m.rng.Float64()*0.05,
			col: cols[m.rng.Intn(len(cols))],
		}
		m.confetti = append(m.confetti, p)
	}
}

func (m *Module) updateConfetti(beat float64) {
	if len(m.confetti) == 0 {
		return
	}
	out := m.confetti[:0]
	for _, p := range m.confetti {
		if beat-p.born <= p.life {
			out = append(out, p)
		}
	}
	m.confetti = out
}

func (m *Module) drawConfetti(screen *ebiten.Image, beat float64) {
	if len(m.confetti) == 0 {
		return
	}
	for _, p := range m.confetti {
		u := clamp01((beat - p.born) / p.life)
		t := (beat - p.born)
		x := p.x + p.vx*t*0.32
		y := p.y + p.vy*t*0.32 - 5.3*t*t
		alpha := 1.0
		if u > 0.72 {
			alpha = 1 - (u-0.72)/0.28
		}
		col := toRGBA([4]float64{p.col[0], p.col[1], p.col[2], p.col[3] * alpha})
		cx, cy := m.proj.Apply(x, y)
		w, h := float32(p.w*54), float32(p.h*54)
		wobble := float32(0.65 + 0.35*math.Abs(math.Sin(p.rot+p.vr*t)))
		vector.DrawFilledRect(screen, float32(cx)-w/2, float32(cy)-h/2, w*wobble, h, col, true)
	}
}
