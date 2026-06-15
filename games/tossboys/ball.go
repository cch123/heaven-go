package tossboys

import (
	"math"

	"hsdemo/kart"
)

type tossBall struct {
	inst      *kart.Instance
	state     string
	startBeat float64
	path      ballPath
	lastReal  [2]float64
	pos       [3]float64
	dead      bool
}

func newBall(m *Module, beat float64, state string, length float64) *tossBall {
	b := &tossBall{inst: m.ballTemplate.NewInstance(), lastReal: [2]float64{}}
	b.inst.PlayDefaultState("", beat, m.ctx.SecPerBeat(beat))
	b.setState(m, state, beat, length)
	return b
}

func (b *tossBall) setState(m *Module, state string, beat, length float64) {
	if b == nil {
		return
	}
	if b.state != "" {
		p := b.path.posAt(beat, b.startBeat, b.lastReal, m.targets)
		b.lastReal = [2]float64{p[0], p[1]}
	}
	p := m.paths[state]
	if length != 0 {
		p = p.durationOverride(length)
	}
	b.state = state
	b.startBeat = beat
	b.path = p
}

func (b *tossBall) update(m *Module, beat float64) {
	if b == nil || b.dead {
		return
	}
	b.pos = b.path.posAt(beat, b.startBeat, b.lastReal, m.targets)
	b.inst.Offset = [2]float64{b.pos[0], b.pos[1]}
	b.inst.Rot = deg(-b.path.rotAt() * math.Max(0, beat-b.startBeat))
}

func (b *tossBall) playHit(m *Module, beat float64, barely bool) {
	if b == nil {
		return
	}
	if barely {
		b.inst.PlayState("", "WiggleBall", beat, 0.5)
	} else {
		b.inst.PlayState("", "Hit", beat, 0.5)
	}
}
