package supersamuraislice

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kart"
)

const superSliceEffectMaxLife = 1.15

type superSliceParticle struct {
	sprite      string
	start, life float64
	x, y        float64
	vx, vy      float64
	gravity     float64
	rot, rotVel float64
	sx0, sy0    float64
	sx1, sy1    float64
	tint        [4]float64
}

func (m *Module) addEffect(beat float64, typ int, pos [2]float64) {
	startT := 0.0
	if m.ctx != nil {
		startT = m.ctx.Time()
	}
	m.effects = append(m.effects, effect{beat: beat, startT: startT, typ: typ, pos: pos})
}

func (m *Module) effectNodePos(path string) [2]float64 {
	if m.ctx != nil && m.ctx.Scene != nil {
		if world, ok := m.ctx.Scene.NodeWorld(path); ok {
			return [2]float64{world.Tx, world.Ty}
		}
	}
	if m.ctx != nil && m.ctx.Assets != nil {
		return nodePos(m.ctx.Assets, path)
	}
	return [2]float64{}
}

func (m *Module) drawEffect(dst *ebiten.Image, fx effect, t float64) {
	age := t - fx.startT
	if age < 0 || age > superSliceEffectMaxLife {
		return
	}
	root := kart.Translate(fx.pos[0], fx.pos[1])
	for _, p := range superSliceEffectParticles(fx.typ) {
		localAge := age - p.start
		if localAge < 0 || localAge > p.life || p.life <= 0 {
			continue
		}
		u := clamp01(localAge / p.life)
		alpha := p.tint[3] * (1 - u)
		if alpha <= 0 {
			continue
		}
		x := p.x + p.vx*localAge
		y := p.y + p.vy*localAge - p.gravity*localAge*localAge
		rot := p.rot + p.rotVel*localAge
		sx := lerp(p.sx0, p.sx1, u)
		sy := lerp(p.sy0, p.sy1, u)
		tint := p.tint
		tint[3] = alpha
		m.ctx.Assets.DrawSpriteOpts(dst, p.sprite, root.Mul(kart.TRS(x, y, rot, sx, sy)), m.proj, kart.SpriteOpts{Tint: tint})
	}
}

// The extractor does not yet emit Unity ParticleSystem modules generically.
// These bursts use the authored sliceparticles atlas and the same C# trigger
// beats, keeping the missing work localized to the serialized particle curves.
func superSliceEffectParticles(typ int) []superSliceParticle {
	switch typ {
	case effectExplode2:
		return []superSliceParticle{
			fx("sliceparticles_13", 0, 0.55, 0, 0, 0, 0, 0, 0, 0, 0.55, 0.55, 1.55, 1.55, rgba(255, 240, 180, 210)),
			fx("sliceparticles_7", 0, 0.42, -0.55, 0.04, -1.1, 0.1, 0, math.Pi/2, -1.4, 0.52, 0.42, 0.84, 0.58, rgba(255, 255, 255, 210)),
			fx("sliceparticles_8", 0.02, 0.45, 0.52, 0.03, 1.15, 0.08, 0, -math.Pi/2, 1.2, 0.5, 0.4, 0.82, 0.56, rgba(255, 248, 190, 210)),
			fx("sliceparticles_3", 0.04, 0.58, -0.08, 0.16, -0.2, 0.25, 0.6, 0.1, 1.8, 0.38, 0.34, 0.68, 0.58, rgba(255, 208, 105, 170)),
			fx("sliceparticles_4", 0.05, 0.6, 0.18, -0.12, 0.25, -0.1, 0.5, -0.35, -1.2, 0.36, 0.32, 0.66, 0.56, rgba(255, 190, 80, 160)),
		}
	case effectExplode3:
		return []superSliceParticle{
			fx("sliceparticles_14", 0, 0.55, 0, 0, 0, 0, 0, 0, 0.7, 0.48, 0.48, 1.35, 1.35, rgba(255, 245, 190, 210)),
			fx("sliceparticles_9", 0, 0.42, -0.42, 0.18, -0.75, 0.52, 0, 0.75, -0.9, 0.48, 0.42, 0.78, 0.56, rgba(255, 255, 255, 205)),
			fx("sliceparticles_10", 0.02, 0.45, 0.44, -0.12, 0.85, -0.25, 0, -0.92, 1.05, 0.48, 0.42, 0.82, 0.58, rgba(255, 225, 120, 205)),
			fx("sliceparticles_16", 0.06, 0.58, 0.05, -0.02, 0.05, -0.18, 0.4, -0.2, 0.9, 0.42, 0.42, 0.88, 0.88, rgba(255, 150, 70, 160)),
		}
	case effectLightning:
		return []superSliceParticle{
			fx("sliceparticles_17", 0, 0.36, 0, 0, 0, 0, 0, 0, 0, 0.5, 0.5, 1.25, 1.25, rgba(210, 245, 255, 220)),
			fx("sliceparticles_7", 0, 0.3, -0.12, 0.06, -0.18, 0.06, 0, -0.52, 2.8, 0.46, 0.5, 0.8, 0.72, rgba(190, 230, 255, 235)),
			fx("sliceparticles_8", 0.02, 0.32, 0.16, -0.03, 0.18, -0.05, 0, 0.58, -3.2, 0.42, 0.48, 0.78, 0.68, rgba(120, 190, 255, 220)),
			fx("sliceparticles_12", 0.04, 0.34, 0.02, 0.16, 0.08, 0.16, 0, -1.1, 2.4, 0.32, 0.38, 0.62, 0.56, rgba(255, 255, 255, 210)),
		}
	case effectWaterL:
		return waterParticles(1)
	case effectWaterR:
		return waterParticles(-1)
	default:
		return []superSliceParticle{
			fx("sliceparticles_13", 0, 0.5, 0, 0, 0, 0, 0, 0, 0.2, 0.5, 0.5, 1.45, 1.45, rgba(255, 238, 175, 205)),
			fx("sliceparticles_7", 0, 0.42, -0.35, 0.08, -0.65, 0.2, 0, -0.72, -1.1, 0.42, 0.4, 0.72, 0.54, rgba(255, 255, 255, 205)),
			fx("sliceparticles_11", 0.02, 0.44, 0.35, -0.04, 0.68, -0.06, 0, 0.72, 1.2, 0.42, 0.4, 0.72, 0.54, rgba(255, 225, 115, 205)),
			fx("sliceparticles_3", 0.04, 0.55, -0.08, -0.1, -0.1, -0.12, 0.45, -0.18, 1.1, 0.34, 0.32, 0.62, 0.5, rgba(255, 175, 75, 150)),
			fx("sliceparticles_6", 0.08, 0.52, 0.12, 0.14, 0.18, 0.18, 0.5, 0.32, -1.3, 0.32, 0.3, 0.58, 0.46, rgba(255, 200, 95, 145)),
		}
	}
}

func waterParticles(side float64) []superSliceParticle {
	return []superSliceParticle{
		fx("sliceparticles_0", 0, 0.42, 0, 0, 0, 0.95, 1.1, 0, 0.4*side, 0.45, 0.45, 0.85, 0.85, rgba(255, 255, 255, 190)),
		fx("sliceparticles_1", 0.02, 0.46, 0.2*side, 0.05, 0.55*side, 1.15, 1.25, 0.3*side, 1.3*side, 0.36, 0.36, 0.66, 0.66, rgba(155, 220, 255, 175)),
		fx("sliceparticles_2", 0.04, 0.42, -0.18*side, 0.08, -0.45*side, 1.0, 1.15, -0.2*side, -1.2*side, 0.45, 0.45, 0.75, 0.75, rgba(255, 255, 255, 165)),
		fx("sliceparticles_3", 0.06, 0.5, 0.05*side, -0.05, 0.35*side, 0.75, 1.35, 0.1*side, 0.8*side, 0.28, 0.28, 0.58, 0.48, rgba(120, 200, 255, 150)),
		fx("sliceparticles_5", 0.08, 0.52, -0.28*side, -0.08, -0.72*side, 0.75, 1.3, -0.35*side, -1.1*side, 0.26, 0.24, 0.52, 0.44, rgba(230, 250, 255, 145)),
	}
}

func fx(sprite string, start, life, x, y, vx, vy, gravity, rot, rotVel, sx0, sy0, sx1, sy1 float64, tint [4]float64) superSliceParticle {
	return superSliceParticle{
		sprite: sprite, start: start, life: life,
		x: x, y: y, vx: vx, vy: vy, gravity: gravity,
		rot: rot, rotVel: rotVel,
		sx0: sx0, sy0: sy0, sx1: sx1, sy1: sy1,
		tint: tint,
	}
}

func rgba(r, g, b, a int) [4]float64 {
	return [4]float64{
		float64(r) / 255,
		float64(g) / 255,
		float64(b) / 255,
		float64(a) / 255,
	}
}
