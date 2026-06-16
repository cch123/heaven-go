package cheerreaders

import (
	"math"
	"math/rand"

	"hsdemo/kart"
)

const (
	cheerConfettiWhiteSprite = "cheer-readers_183"
	cheerConfettiBlackSprite = "cheer-readers_184"

	// WhiteParticle/BlackParticle are prefab ParticleSystems with identical
	// modules except for the UVModule sprite. These constants mirror the
	// serialized Initial/Shape/Emission/Size/Rotation/Color/Renderer fields.
	cheerConfettiSystemLengthSec = 0.5
	cheerConfettiSimulationSpeed = 1
	cheerConfettiLifetimeSec     = 0.32
	cheerConfettiEmissionRate    = 50
	cheerConfettiParticleCount   = int(cheerConfettiSystemLengthSec * cheerConfettiEmissionRate)
	cheerConfettiShapeWidth      = 13
	cheerConfettiShapeHeight     = 7
	cheerConfettiStartRotMax     = 0.87266463
	cheerConfettiSpin            = 13.08997
	cheerConfettiOrder           = 99
	cheerConfettiAlphaPeak       = 32507.0 / 65535.0

	cheerConfettiSizeKey0T = 0
	cheerConfettiSizeKey0V = 0.4197002
	cheerConfettiSizeKey0O = 3.3661678
	cheerConfettiSizeKey1T = 0.9923706
	cheerConfettiSizeKey1V = 0.4368286
	cheerConfettiSizeKey1I = -1.2487001
)

func newCheerConfetti(m *Module, black bool) []confetti {
	sprite := cheerConfettiWhiteSprite
	seedSalt := uint64(0x4377)
	if black {
		sprite = cheerConfettiBlackSprite
		seedSalt = 0x4378
	}
	baseX, baseY := m.cheerConfettiBase(black)
	seed := int64(math.Float64bits(m.ctx.Beat()) ^ seedSalt)
	rng := rand.New(rand.NewSource(seed))
	out := make([]confetti, 0, cheerConfettiParticleCount)
	for i := 0; i < cheerConfettiParticleCount; i++ {
		out = append(out, confetti{
			x:      baseX + (rng.Float64()-0.5)*cheerConfettiShapeWidth,
			y:      baseY + (rng.Float64()-0.5)*cheerConfettiShapeHeight,
			born:   m.lastT,
			delay:  float64(i) / cheerConfettiEmissionRate,
			rot:    rng.Float64() * cheerConfettiStartRotMax,
			spin:   cheerConfettiSpin,
			sprite: sprite,
		})
	}
	return out
}

func (m *Module) cheerConfettiBase(black bool) (float64, float64) {
	role := "whiteYayParticle"
	if black {
		role = "blackYayParticle"
	}
	if m.ctx != nil && m.ctx.Scene != nil {
		if world, ok := m.ctx.Scene.NodeWorld(m.ctx.Role(role)); ok && world.A != 0 && world.D != 0 {
			return world.Apply(0, 0)
		}
	}
	// Serialized localPosition of both WhiteParticle and BlackParticle. The
	// nodes are static under NerdsHolder, so this is the exact fallback before
	// the first scene sample has populated NodeWorld.
	return 0.64, 0.88
}

func (m *Module) queueConfetti(t float64) {
	alive := m.particles[:0]
	for _, p := range m.particles {
		if t-p.born > cheerConfettiSystemLengthSec+cheerConfettiLifetimeSec {
			continue
		}
		alive = append(alive, p)
		age := (t - p.born - p.delay) * cheerConfettiSimulationSpeed
		if age < 0 || age > cheerConfettiLifetimeSec {
			continue
		}
		u := age / cheerConfettiLifetimeSec
		size := cheerConfettiSize(u)
		sx, sy := m.cheerConfettiSpriteScale(p.sprite, size)
		if sx == 0 || sy == 0 {
			continue
		}
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: p.sprite,
			World:  kart.TRS(p.x, p.y, p.rot+p.spin*age, sx, sy),
			Order:  cheerConfettiOrder,
			Tint:   [4]float64{1, 1, 1, cheerConfettiAlpha(u)},
		})
	}
	m.particles = alive
}

func (m *Module) cheerConfettiSpriteScale(sprite string, particleSize float64) (float64, float64) {
	if m.ctx == nil || m.ctx.Assets == nil {
		return 0, 0
	}
	sp, ok := m.ctx.Assets.Sheet.Sprites[sprite]
	if !ok {
		return 0, 0
	}
	ppu := sp.PPU
	if ppu <= 0 {
		ppu = m.ctx.Assets.Sheet.PPU
	}
	if ppu <= 0 || sp.H == 0 {
		return 0, 0
	}
	// ParticleSystem startSize is a billboard world height. DrawSprite renders
	// atlas slices in SpriteRenderer units (pixels/PPU), so normalize by the
	// slice height to preserve Unity's particle-size semantics.
	unitH := float64(sp.H) / ppu
	scale := particleSize / unitH
	return scale, scale
}

func cheerConfettiSize(u float64) float64 {
	u = clamp01(u)
	if u >= cheerConfettiSizeKey1T {
		return cheerConfettiSizeKey1V
	}
	return hermite(cheerConfettiSizeKey0T, cheerConfettiSizeKey0V, cheerConfettiSizeKey0O,
		cheerConfettiSizeKey1T, cheerConfettiSizeKey1V, cheerConfettiSizeKey1I, u)
}

func cheerConfettiAlpha(u float64) float64 {
	u = clamp01(u)
	if u <= cheerConfettiAlphaPeak {
		return u / cheerConfettiAlphaPeak
	}
	return (1 - u) / (1 - cheerConfettiAlphaPeak)
}

func hermite(t0, v0, out0, t1, v1, in1, t float64) float64 {
	if t1 == t0 {
		return v1
	}
	dt := t1 - t0
	u := (t - t0) / dt
	u2, u3 := u*u, u*u*u
	h00 := 2*u3 - 3*u2 + 1
	h10 := u3 - 2*u2 + u
	h01 := -2*u3 + 3*u2
	h11 := u3 - u2
	return h00*v0 + h10*out0*dt + h01*v1 + h11*in1*dt
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
