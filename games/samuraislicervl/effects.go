package samuraislicervl

import (
	"math"
	"math/rand"

	"hsdemo/engine"
	"hsdemo/kart"
)

type smogParticle struct {
	inst    *kart.Instance
	base    [2]float64
	angle   float64
	speed   float64
	initRot float64
	moveRad float64
}

type smogState struct {
	root      [2]float64
	scale     float64
	particles []smogParticle

	animating bool
	startBeat float64
	length    float64
	from      float64
	to        float64
	ease      int
	lastT     float64
	hasLastT  bool
}

type flashEffect struct {
	lightning *kart.Instance
	flash     *kart.Instance
	startBeat float64
	deathBeat float64
	dead      bool
}

func newSmogState(ctx *engine.Ctx, rng *rand.Rand) smogState {
	comp := ctx.Assets.Extra.Components["smog"]
	scale := numOr(comp, "pixelScale", 0.0085)
	root := [2]float64{
		numOr(comp, "baseOffset.x", 62.5)*scale + numOr(comp, "manualOffset.x", 0.47),
		numOr(comp, "baseOffset.y", 7.5)*scale + numOr(comp, "manualOffset.y", -0.45),
	}
	def := [2]float64{numOr(comp, "defaultStartPos.x", -30), numOr(comp, "defaultStartPos.y", -22.5)}
	tmpl := kart.NewTemplate(ctx.Assets, refOr(ctx, comp, "particlePrefab", "SmogParticlePrefab"))
	st := smogState{root: root, scale: 1}
	for _, step := range smogSteps(def) {
		if tmpl == nil {
			continue
		}
		p := smogParticle{
			inst:    tmpl.NewInstance(),
			base:    [2]float64{step[0] * scale, step[1] * scale},
			angle:   rng.Float64() * 360,
			speed:   150 + rng.Float64()*250,
			initRot: rng.Float64() * 2 * math.Pi,
			moveRad: 0.25,
		}
		if rng.Float64() > 0.5 {
			p.speed = -p.speed
		}
		p.inst.SetScale("", 1, 1)
		p.inst.SetRot("", p.initRot)
		st.particles = append(st.particles, p)
	}
	return st
}

func smogSteps(def [2]float64) [][2]float64 {
	missing := math.Inf(-1)
	raw := [][2]float64{
		{45, 37.5}, {52.5, 24}, {45, missing}, {6, 45},
		{30, missing}, {7.5, 7.5}, {22.5, 15}, {30, 48},
		{missing, 24}, {7.5, 31.5}, {15, 27}, {missing, 15},
		{60, 30}, {45, 40.5}, {37.5, 18}, {52.5, 9},
	}
	for i := range raw {
		if math.IsInf(raw[i][0], -1) {
			raw[i][0] = def[0]
		}
		if math.IsInf(raw[i][1], -1) {
			raw[i][1] = def[1]
		}
	}
	return raw
}

func (s *smogState) animate(beat, length float64, show bool, ease int) {
	s.startBeat = beat
	s.length = length
	if show {
		s.from, s.to = 0, 1
	} else {
		s.from, s.to = 1, 0
	}
	s.ease = ease
	s.animating = true
	if length <= 0 {
		s.scale = s.to
		s.animating = false
	}
}

func (s *smogState) applyAt(events []smogEvt, beat float64) {
	s.scale = 1
	s.animating = false
	for _, ev := range events {
		if ev.beat > beat {
			break
		}
		from, to := 1.0, 0.0
		if ev.show {
			from, to = 0, 1
		}
		if ev.length <= 0 || beat >= ev.beat+ev.length {
			s.scale = to
			continue
		}
		u := (beat - ev.beat) / ev.length
		s.scale = engine.Ease(ev.ease, from, to, u)
		s.startBeat, s.length, s.from, s.to, s.ease, s.animating = ev.beat, ev.length, from, to, ev.ease, true
	}
}

func (s *smogState) update(t, beat float64) {
	dt := 0.0
	if s.hasLastT {
		dt = t - s.lastT
		if dt < 0 || dt > 1 {
			dt = 0
		}
	}
	s.lastT, s.hasLastT = t, true
	if s.animating {
		u := 1.0
		if s.length > 0 {
			u = (beat - s.startBeat) / s.length
		}
		s.scale = engine.Ease(s.ease, s.from, s.to, u)
		if u >= 1 {
			s.animating = false
		}
	}
	for i := range s.particles {
		s.particles[i].angle += s.particles[i].speed * dt
	}
}

func (s *smogState) queue(scene *kart.SceneInst, beat float64) {
	if s.scale <= 0 {
		return
	}
	for i := range s.particles {
		p := &s.particles[i]
		ang := deg(p.angle)
		x := s.root[0] + (p.base[0]+math.Cos(ang)*p.moveRad)*s.scale
		y := s.root[1] + (p.base[1]+math.Sin(ang)*p.moveRad)*s.scale
		p.inst.Offset = [2]float64{x, y}
		p.inst.Scale = [2]float64{s.scale, s.scale}
		p.inst.Queue(scene, beat, kart.Identity(), -5.75)
	}
}

func (m *Module) spawnFlash(beat float64, strike int) {
	if m.lightningT == nil || m.flashT == nil {
		return
	}
	if strike < 1 || strike > 3 {
		strike = 1
	}
	f := &flashEffect{
		lightning: m.lightningT.NewInstance(),
		flash:     m.flashT.NewInstance(),
		startBeat: beat,
		deathBeat: beat + 1,
	}
	f.lightning.PlayState("Lightning1", "LightningStrike"+itoa(strike), beat, m.ctx.SecPerBeat(beat))
	f.flash.PlayState("", "Flash", beat, m.ctx.SecPerBeat(beat))
	m.flashes = append(m.flashes, f)
}

func (f *flashEffect) update(beat float64) {
	if beat >= f.deathBeat {
		f.dead = true
	}
}

func (f *flashEffect) queue(scene *kart.SceneInst, beat float64) {
	if f.dead {
		return
	}
	f.lightning.Queue(scene, beat, kart.Identity(), 0)
	f.flash.Queue(scene, beat, kart.Identity(), 0)
}

func liveFlashes(in []*flashEffect, beat float64) []*flashEffect {
	out := in[:0]
	for _, f := range in {
		if !f.dead && beat < f.deathBeat {
			out = append(out, f)
		}
	}
	return out
}
