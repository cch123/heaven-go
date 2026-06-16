package chargingchicken

import "hsdemo/games/internal/particlefx"

func (m *Module) addParticleEffect(beat float64, root, anchor string, pos [2]float64) {
	if m.particles == nil {
		return
	}
	startT := 0.0
	if m.ctx != nil {
		startT = m.ctx.Time()
	}
	if fx, ok := m.particles.NewEffect(root, anchor, pos, beat, startT); ok {
		m.effects = append(m.effects, fx)
	}
}

func liveEffects(in []particlefx.Effect, t float64) []particlefx.Effect {
	out := in[:0]
	for _, fx := range in {
		if t-fx.StartT < fx.Life {
			out = append(out, fx)
		}
	}
	return out
}
