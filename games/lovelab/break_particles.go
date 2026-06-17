package lovelab

import (
	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/games/internal/particlefx"
)

func (m *Module) spawnFlaskBreakParticle(root string, beat float64) {
	if m.breakParticles == nil || root == "" {
		return
	}
	x, y, ok := m.nodeWorld(root)
	if !ok {
		x, y = 0, 0
	}
	startT := 0.0
	if m.ctx != nil {
		startT = m.ctx.Time()
	}
	if fx, ok := m.breakParticles.NewEffect(root, root, [2]float64{x, y}, beat, startT); ok {
		m.breakEffects = append(m.breakEffects, fx)
	}
}

func (m *Module) drawBreakParticles(screen *ebiten.Image, t float64) {
	if m.breakParticles == nil {
		return
	}
	m.breakEffects = liveBreakEffects(m.breakEffects, t)
	for _, fx := range m.breakEffects {
		m.breakParticles.Draw(screen, fx, t)
	}
}

func liveBreakEffects(in []particlefx.Effect, t float64) []particlefx.Effect {
	out := in[:0]
	for _, fx := range in {
		if t-fx.StartT < fx.Life {
			out = append(out, fx)
		}
	}
	return out
}
