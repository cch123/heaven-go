package fanclub

import (
	"math"
	"strings"

	"hsdemo/kart"
)

const (
	fanClubEffectSprite     = "ntrIdol_handCrapEffect"
	fanClubWinkEffectSprite = "idol_wink_star"
	fanClubKissEffectSprite = "ntrIdol_heartEffect"

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
		path: "Effect_IdolCrap", scale: 0.62, layer: 0, order: 0,
		tint: [4]float64{1, 0.43, 0.05, 0.92},
	})
}

func (fx *fanClubEffects) spawnDancerClap(m *Module, d *dancer, beat float64) {
	if d == nil || !d.active {
		return
	}
	fx.spawn(m, fanClubEffectBurst{
		beat: beat, lifetime: fanClubClapLifetimeSec, kind: fanClubEffectClap,
		path: d.clapEffect, scale: 0.56, layer: 0, order: 0,
		tint: [4]float64{1, 0.46, 0.05, 0.86},
	})
}

func (fx *fanClubEffects) spawnFanClap(m *Module, f *fan, beat float64) {
	if f == nil || f.inst == nil {
		return
	}
	fx.spawn(m, fanClubEffectBurst{
		beat: beat, lifetime: fanClubClapLifetimeSec, kind: fanClubEffectFanClap,
		// Fan.prefab's ClapParticle event plays the Effect_FanCrap sibling
		// ParticleSystem. It is deliberately outside root_motion's SortingGroup;
		// putting it in the group makes the local order=32 burst draw as a
		// "light" stuck on top of the monkey head.
		fan: f, relPath: "Effect_FanCrap", scale: 0.18, layer: 0, order: 32,
		tint: [4]float64{1, 0.55, 0.08, 0.55},
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
		if burst.kind == fanClubEffectClap {
			fx.queueClapBurst(m, burst, sample, world)
			continue
		}
		world = world.Mul(kart.Rotate(sample.rot)).Mul(kart.Scale(sample.scale, sample.scale))
		q := kart.ExtraSprite{
			Sprite: burst.sprite(),
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

func (fx *fanClubEffects) queueClapBurst(m *Module, burst fanClubEffectBurst, sample fanClubEffectSample, world kart.Aff) {
	// The authored Idol/BackDancer clap ParticleSystem is a 5-particle burst
	// with renderer sortingOrder 0. Drawing one oversized order-32 sprite at the
	// emitter origin covers the performers' faces, so this keeps the burst small
	// and anchors it to the sampled hand midpoint.
	dirs := [5][3]float64{
		{-0.36, 0.04, -0.35},
		{-0.18, 0.18, -0.12},
		{0.00, 0.24, 0.06},
		{0.18, 0.18, 0.18},
		{0.36, 0.04, 0.38},
	}
	for _, d := range dirs {
		pw := world.
			Mul(kart.Translate(d[0]*sample.travel*0.75, d[1]*sample.travel*0.75)).
			Mul(kart.Rotate(sample.rot + d[2])).
			Mul(kart.Scale(sample.scale, sample.scale))
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: burst.sprite(),
			World:  pw,
			Layer:  burst.layer,
			Order:  burst.order,
			Tint:   sample.tint,
		})
	}
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
		// Unity's burst has randomPositionAmount, so the first rendered frame is
		// already a small spread. Without this base separation the three sprites
		// stack into one bright block over the spectator's head.
		spread := 0.18 + sample.travel*0.85
		pw := world.
			Mul(kart.Translate(d[0]*spread, d[1]*spread)).
			Mul(kart.Rotate(sample.rot + d[2])).
			Mul(kart.Scale(sample.scale, sample.scale))
		q := kart.ExtraSprite{
			Sprite: burst.sprite(),
			World:  pw,
			Layer:  burst.layer,
			Order:  burst.order,
			Tint:   sample.tint,
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
	if burst.kind == fanClubEffectClap {
		if w, ok := burst.actorHandWorld(m); ok {
			return w, true
		}
	}
	if burst.path == "" || m == nil || m.ctx == nil || m.ctx.Scene == nil {
		return kart.Identity(), false
	}
	return m.ctx.Scene.NodeWorld(burst.path)
}

func (burst fanClubEffectBurst) sprite() string {
	switch burst.kind {
	case fanClubEffectWink:
		return fanClubWinkEffectSprite
	case fanClubEffectKiss:
		return fanClubKissEffectSprite
	default:
		return fanClubEffectSprite
	}
}

func (burst fanClubEffectBurst) fanHandWorld(beat float64) (kart.Aff, bool) {
	if burst.fan == nil || burst.fan.inst == nil {
		return kart.Identity(), false
	}
	left, okL := burst.fan.inst.NodeWorldAt("root_motion/Body/fan_ArmL/fan_HandL", kart.Identity(), beat)
	right, okR := burst.fan.inst.NodeWorldAt("root_motion/Body/fan_ArmR/fan_HandR", kart.Identity(), beat)
	return midpointWorld(left, okL, right, okR)
}

func (burst fanClubEffectBurst) actorHandWorld(m *Module) (kart.Aff, bool) {
	if m == nil || m.ctx == nil || m.ctx.Scene == nil {
		return kart.Identity(), false
	}
	root := ""
	switch {
	case burst.path == "Effect_IdolCrap":
		root = m.arisa
	case strings.HasSuffix(burst.path, "/Effect_IdolCrap"):
		root = strings.TrimSuffix(burst.path, "/Effect_IdolCrap")
	}
	if root == "" {
		return kart.Identity(), false
	}
	left, okL := m.ctx.Scene.NodeWorld(root + "/idol_torso/idol_arm_L/idol_hand_L")
	right, okR := m.ctx.Scene.NodeWorld(root + "/idol_torso/idol_arm_R/idol_hand_R")
	return midpointWorld(left, okL, right, okR)
}

func midpointWorld(left kart.Aff, okL bool, right kart.Aff, okR bool) (kart.Aff, bool) {
	switch {
	case okL && okR:
		return kart.Translate((left.Tx+right.Tx)/2, (left.Ty+right.Ty)/2), true
	case okL:
		return kart.Translate(left.Tx, left.Ty), true
	case okR:
		return kart.Translate(right.Tx, right.Ty), true
	default:
		return kart.Identity(), false
	}
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
