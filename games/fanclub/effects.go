package fanclub

import (
	"math"

	"hsdemo/kart"
)

const (
	fanClubEffectSprite = "impact_effect"

	fanClubClapLifetimeSec = 0.45
	fanClubWinkLifetimeSec = 0.40
	fanClubKissLifetimeSec = 1.00

	fanClubWinkEventDelaySec = 0.05
	fanClubFanParticleCount  = 3
)

type fanClubEffectKind int

const (
	fanClubEffectClap fanClubEffectKind = iota
	fanClubEffectFanClap
	fanClubEffectWink
	fanClubEffectKiss
)

type fanClubEffectBurst struct {
	beat       float64
	secPerBeat float64
	lifetime   float64
	kind       fanClubEffectKind
	path       string
	relPath    string
	fan        *fan
	scale      float64
	layer      int
	order      int
	rot        float64
	tint       [4]float64
}

type fanClubEffects struct {
	bursts []fanClubEffectBurst
}

type fanClubEffectSample struct {
	scale  float64
	rot    float64
	travel float64
	tint   [4]float64
}

func (fx *fanClubEffects) spawn(m *Module, burst fanClubEffectBurst) {
	if m == nil || m.ctx == nil {
		return
	}
	if burst.secPerBeat <= 0 {
		burst.secPerBeat = m.ctx.SecPerBeat(burst.beat)
	}
	if burst.lifetime <= 0 {
		burst.lifetime = fanClubClapLifetimeSec
	}
	if burst.scale == 0 {
		burst.scale = 1
	}
	if burst.tint == [4]float64{} {
		burst.tint = [4]float64{1, 0.55, 0.1, 0.85}
	}
	fx.bursts = append(fx.bursts, burst)
}

func (fx *fanClubEffects) spawnIdolClap(m *Module, beat float64) {
	fx.spawn(m, fanClubEffectBurst{
		beat: beat, lifetime: fanClubClapLifetimeSec, kind: fanClubEffectClap,
		path: "Effect_IdolCrap", scale: 0.86, layer: 0, order: 32,
		tint: [4]float64{1, 0.43, 0.05, 0.92},
	})
}

func (fx *fanClubEffects) spawnDancerClap(m *Module, d *dancer, beat float64) {
	if d == nil || !d.active {
		return
	}
	fx.spawn(m, fanClubEffectBurst{
		beat: beat, lifetime: fanClubClapLifetimeSec, kind: fanClubEffectClap,
		path: d.clapEffect, scale: 0.72, layer: 0, order: 32,
		tint: [4]float64{1, 0.46, 0.05, 0.86},
	})
}

func (fx *fanClubEffects) spawnFanClap(m *Module, f *fan, beat float64) {
	if f == nil || f.inst == nil {
		return
	}
	fx.spawn(m, fanClubEffectBurst{
		beat: beat, lifetime: fanClubClapLifetimeSec, kind: fanClubEffectFanClap,
		// Fan.prefab's ClapParticle animation event plays Effect_FanCrap's
		// ParticleSystem: burst=3, startLifetime=0.45, sortingOrder=32. A single
		// centered sprite makes the effect sit on the monkey head, so queue a
		// small deterministic burst from the particle root instead.
		fan: f, relPath: "Effect_FanCrap", scale: 0.24, layer: 0, order: 32,
		tint: [4]float64{1, 0.55, 0.08, 0.78},
	})
}

func (fx *fanClubEffects) spawnIdolWink(m *Module, beat float64) {
	path := "Effect_IdolWink"
	scale := 0.62
	if m != nil && m.performance == perfArrange {
		path = "Effect_IdolWinkArr"
		scale = 0.68
	}
	fx.spawn(m, fanClubEffectBurst{
		beat: beat, lifetime: fanClubWinkLifetimeSec, kind: fanClubEffectWink,
		path: path, scale: scale, layer: 0, order: 34, rot: -0.2,
		tint: [4]float64{1, 0.38, 0.98, 0.88},
	})
}

func (fx *fanClubEffects) spawnDancerWink(m *Module, d *dancer, beat float64) {
	if d == nil || !d.active {
		return
	}
	fx.spawn(m, fanClubEffectBurst{
		beat: beat, lifetime: fanClubWinkLifetimeSec, kind: fanClubEffectWink,
		path: d.winkEffect, scale: 0.54, layer: 0, order: 34, rot: -0.2,
		tint: [4]float64{1, 0.38, 0.98, 0.82},
	})
}

func (fx *fanClubEffects) spawnIdolKiss(m *Module, beat float64) {
	fx.spawn(m, fanClubEffectBurst{
		beat: beat, lifetime: fanClubKissLifetimeSec, kind: fanClubEffectKiss,
		path: "Effect_IdolKiss2", scale: 0.58, layer: 0, order: 34, rot: 0.25,
		tint: [4]float64{1, 0.18, 0.28, 0.86},
	})
}

func (fx *fanClubEffects) queue(m *Module, beat float64) {
	if m == nil || m.ctx == nil || m.ctx.Scene == nil {
		return
	}
	kept := fx.bursts[:0]
	for _, burst := range fx.bursts {
		sample, keep, draw := burst.sample(beat)
		if keep {
			kept = append(kept, burst)
		}
		if !draw {
			continue
		}
		world, ok := burst.world(m, beat)
		if !ok {
			continue
		}
		if burst.kind == fanClubEffectFanClap {
			fx.queueFanClapBurst(m, burst, sample, world)
			continue
		}
		world = world.Mul(kart.Rotate(sample.rot)).Mul(kart.Scale(sample.scale, sample.scale))
		q := kart.ExtraSprite{
			Sprite: fanClubEffectSprite,
			World:  world,
			Layer:  burst.layer,
			Order:  burst.order,
			Tint:   sample.tint,
		}
		if burst.fan != nil && burst.fan.groupKey >= 0 {
			q.HasGroup = true
			q.GroupKey = burst.fan.groupKey
			q.GroupOrder = burst.fan.groupOrder
		}
		m.ctx.Scene.Queue(q)
	}
	fx.bursts = kept
}

func (fx *fanClubEffects) queueFanClapBurst(m *Module, burst fanClubEffectBurst, sample fanClubEffectSample, world kart.Aff) {
	// Deterministic offsets mirror the prefab's 3-particle burst while keeping
	// every particle inside the fan SortingGroup. The source ParticleSystem is
	// a short cone with randomPositionAmount=0.2; fixed offsets are preferable
	// here so replays and tests remain stable.
	dirs := [fanClubFanParticleCount][3]float64{
		{-0.44, 0.34, -0.35},
		{0.00, 0.52, 0.05},
		{0.44, 0.34, 0.35},
	}
	for _, d := range dirs {
		pw := world.
			Mul(kart.Translate(d[0]*sample.travel, d[1]*sample.travel)).
			Mul(kart.Rotate(sample.rot + d[2])).
			Mul(kart.Scale(sample.scale, sample.scale))
		q := kart.ExtraSprite{
			Sprite: fanClubEffectSprite,
			World:  pw,
			Layer:  burst.layer,
			Order:  burst.order,
			Tint:   sample.tint,
		}
		if burst.fan != nil && burst.fan.groupKey >= 0 {
			q.HasGroup = true
			q.GroupKey = burst.fan.groupKey
			q.GroupOrder = burst.fan.groupOrder
		}
		m.ctx.Scene.Queue(q)
	}
}

func (burst fanClubEffectBurst) world(m *Module, beat float64) (kart.Aff, bool) {
	if burst.fan != nil && burst.fan.inst != nil {
		if burst.kind == fanClubEffectFanClap {
			return burst.fan.inst.NodeWorldAt(burst.relPath, kart.Identity(), beat)
		}
		return burst.fan.inst.NodeWorld(burst.relPath, kart.Identity())
	}
	if burst.path == "" || m == nil || m.ctx == nil || m.ctx.Scene == nil {
		return kart.Identity(), false
	}
	return m.ctx.Scene.NodeWorld(burst.path)
}

func (burst fanClubEffectBurst) sample(beat float64) (fanClubEffectSample, bool, bool) {
	secPerBeat := burst.secPerBeat
	if secPerBeat <= 0 {
		secPerBeat = 0.5
	}
	age := (beat - burst.beat) * secPerBeat
	if age < 0 {
		return fanClubEffectSample{}, true, false
	}
	if age >= burst.lifetime {
		return fanClubEffectSample{}, false, false
	}
	u := clamp01(age / burst.lifetime)
	scale := burst.scale * (0.65 + 0.55*u)
	alpha := burst.tint[3] * (1 - u)
	if burst.kind == fanClubEffectFanClap {
		scale = burst.scale * (1 - 0.35*u)
		alpha = burst.tint[3] * (1 - u*0.9)
	}
	if burst.kind == fanClubEffectKiss {
		scale = burst.scale * (0.55 + 0.75*math.Sin(u*math.Pi))
		alpha = burst.tint[3] * (1 - u*u)
	}
	tint := burst.tint
	tint[3] = alpha
	return fanClubEffectSample{
		scale:  scale,
		rot:    burst.rot + (u-0.5)*0.18,
		travel: 1.55 * u,
		tint:   tint,
	}, true, true
}

func (m *Module) winkEffectBeat(beat float64) float64 {
	if m == nil || m.ctx == nil {
		return beat
	}
	spb := m.ctx.SecPerBeat(beat)
	if spb <= 0 {
		return beat
	}
	return beat + fanClubWinkEventDelaySec/spb
}
