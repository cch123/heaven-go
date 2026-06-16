package samuraislicervl

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

const (
	hordeSpawn = iota
	hordeRush
	hordeArrived
	hordeBite
	hordeFall
	hordeDead
)

const rollingFadeSec = 0.1

type hordeConfig struct {
	gravity       float64
	fallVelocity  [2]float64
	rotationSpeed float64
}

type hordeSequence struct {
	demons     []*hordeDemon
	spawnIndex int
	missed     bool
	ended      bool
	bitePlayed bool
	deathBeat  float64
}

type hordeDemon struct {
	inst       *kart.Instance
	seq        *hordeSequence
	state      int
	spawn      [2]float64
	target     [2]float64
	pos        [2]float64
	sliceX     float64
	rushBeat   float64
	fallBeat   float64
	fallStartT float64
	deathBeat  float64
	rot        float64
	final      bool
}

func (m *Module) loadHordeConfig() hordeConfig {
	c := componentByPath(m.ctx.Assets, "HordePrefab")
	return hordeConfig{
		gravity:       numOr(c, "gravity", 30),
		fallVelocity:  [2]float64{numOr(c, "fallVelocity.x", 0), numOr(c, "fallVelocity.y", 10)},
		rotationSpeed: numOr(c, "rotationSpeed", -50),
	}
}

func (m *Module) comboDemon(beat float64) {
	seq := &hordeSequence{deathBeat: beat + 8}
	m.hordes = append(m.hordes, seq)

	m.isReady = true
	if math.Abs(beat+float64(m.nextPrepareBeat)-m.lastSuccessfulBeat) <= 2 {
		b := beat + float64(m.nextPrepareBeat)
		m.ctx.At(b, func() { m.playSamurai("SamuraiReady", b, 1) })
	} else {
		m.playSamurai("SamuraiReady", beat, 1)
	}

	m.ctx.SoundAt(beat, "combo1", 1)
	m.ctx.SoundAt(beat+0.3333, "combo2", 1)
	m.ctx.SoundAt(beat+0.6667, "combo3", 1)
	m.ctx.SoundAt(beat+1, "combo4", 1)

	m.ctx.ScheduleInputAction(beat+2, actionCombo,
		func(state float64, _ engine.Judgment) { m.comboDemonStart(seq, state) },
		func() { m.comboDemonMiss(seq, false) })

	m.ctx.At(beat, func() { seq.spawnBatch(m, 1, beat) })
	m.ctx.At(beat+0.3333, func() { seq.spawnBatch(m, 1, beat+0.3333) })
	m.ctx.At(beat+0.6667, func() { seq.spawnBatch(m, 1, beat+0.6667) })
	m.ctx.At(beat+1, func() { seq.spawnBatch(m, 4, beat+1) })
	m.ctx.At(beat+2, func() { seq.scheduleRushes(m, beat+2) })
}

func (m *Module) comboDemonStart(seq *hordeSequence, state float64) {
	if seq.ended {
		return
	}
	m.isDemonSuccess = true
	inputBeat := m.ctx.Beat()
	m.playSamurai("SamuraiHordeStart", inputBeat, 1)
	m.ctx.Sound("HIT6_START")
	m.ctx.SoundLoopPitchVolUntil("ROLLING_START", 1, 1, rollingEndBeat(inputBeat), rollingFadeSec)
	m.ctx.At(inputBeat+0.14, func() { m.playSamurai("SamuraiHordeLoop", inputBeat+0.14, 0.5) })

	m.isHolding = true
	if math.Abs(state) >= 1 {
		seq.missed = true
	}
	for i := 0; i < 7; i++ {
		t := inputBeat + float64(i)/7
		m.ctx.SoundAt(t, "HIT6_ALT", 1)
		if i < 6 {
			m.ctx.At(t, func() { seq.killNext(m, false, t) })
		}
	}

	m.ctx.ScheduleInputActionRelease(inputBeat+1, actionCombo,
		func(state float64, _ engine.Judgment) { m.comboDemonSuccess(seq, state) },
		func() { m.comboDemonEmpty(seq) })
	m.ctx.At(inputBeat+1, func() {
		if seq.ended {
			return
		}
		if seq.missed && math.Abs(state) >= 1 {
			m.comboDemonSuccess(seq, sign(state >= 1)*1.75)
			return
		}
		if !seq.missed {
			m.comboDemonSuccess(seq, 0)
		}
	})
}

func (m *Module) comboDemonSuccess(seq *hordeSequence, state float64) {
	if seq.ended {
		return
	}
	if math.Abs(state) >= 1 {
		m.comboDemonMiss(seq, false)
		return
	}
	seq.ended = true
	m.playSamurai("SamuraiHordeEnd", m.ctx.Beat(), 1)
	if !seq.missed {
		seq.killNext(m, true, m.ctx.Beat())
		m.ctx.Sound("ROLLING_ALL_HIT")
		if m.thunderEffect {
			m.ctx.Sound(m.randomThunder())
			m.spawnFlash(m.ctx.Beat(), 3)
		}
	} else {
		seq.failRest(m, m.ctx.Beat())
		m.ctx.Sound("OSII")
	}
	m.isReady = false
	m.isHolding = false
	seq.deathBeat = m.ctx.Beat() + 5
}

func (m *Module) comboDemonMiss(seq *hordeSequence, fromBite bool) {
	if seq.ended {
		return
	}
	seq.missed = true
	m.isReady = false
	if fromBite && !seq.bitePlayed {
		seq.bitePlayed = true
		m.playSamurai("SamuraiHordeMiss", m.ctx.Beat(), 1)
		m.ctx.Sound("YARARE2")
	}
}

func (m *Module) comboDemonEmpty(seq *hordeSequence) {
	if !seq.ended {
		seq.missed = true
	}
}

func (m *Module) sliceCombo(beat float64) {
	if m.isDemonSuccess {
		m.isDemonSuccess = false
		return
	}
	m.isReady = true
	m.isHolding = true
	m.playSamurai("SamuraiHordeStart", beat, 1)
	m.ctx.At(beat+0.14, func() { m.playSamurai("SamuraiHordeLoop", beat+0.14, 0.5) })
	m.ctx.At(beat+1, func() { m.playSamurai("SamuraiHordeEnd", beat+1, 0.5) })
	m.ctx.At(beat+2, func() {
		m.isReady = false
		if !m.autoBop {
			m.playSamurai("SamuraiIdle", beat+2, 1)
		}
	})
	m.ctx.SoundLoopPitchVolUntil("ROLLING_START", 1, 1, rollingEndBeat(beat), rollingFadeSec)
	for _, off := range []float64{0, 0.14, 0.28, 0.42, 0.56, 0.70, 0.84, 1} {
		m.ctx.SoundAt(beat+off, "slice", 1.1)
	}
}

func rollingEndBeat(startBeat float64) float64 {
	return startBeat + 1
}

func (s *hordeSequence) spawnBatch(m *Module, count int, beat float64) {
	for i := 0; i < count; i++ {
		s.spawnOne(m, beat)
	}
}

func (s *hordeSequence) spawnOne(m *Module, beat float64) {
	if m.hordeT == nil || len(m.hordeSpawnPositions) == 0 {
		return
	}
	idx := s.spawnIndex
	spawn := m.hordeSpawnPositions[idx%len(m.hordeSpawnPositions)]
	target := [2]float64{-2, spawn[1]}
	target[0] += -m.rng.Float64() * 1.5
	d := &hordeDemon{
		inst:      m.hordeT.NewInstance(),
		seq:       s,
		state:     hordeSpawn,
		spawn:     spawn,
		target:    target,
		pos:       spawn,
		sliceX:    m.rng.Float64()*0.5 - 0.25,
		final:     idx == 5,
		deathBeat: beat + 8,
	}
	d.inst.Offset = spawn
	d.inst.SetGroupOrder(1)
	if d.final {
		d.inst.PlayState("", "HordeSummonFinal", beat, m.ctx.SecPerBeat(beat))
	} else {
		d.inst.PlayState("", "HordeSummon", beat, m.ctx.SecPerBeat(beat))
	}
	s.demons = append(s.demons, d)
	s.spawnIndex++
}

func (s *hordeSequence) scheduleRushes(m *Module, beat float64) {
	for i, d := range s.demons {
		t := beat + float64(i)/7
		dd := d
		m.ctx.At(t, func() { dd.rush(m, t) })
	}
}

func (d *hordeDemon) rush(m *Module, beat float64) {
	if d.state != hordeSpawn {
		return
	}
	d.state = hordeRush
	d.rushBeat = beat
	d.inst.PlayState("", "HordeRush", beat, m.ctx.SecPerBeat(beat))
}

func (s *hordeSequence) killNext(m *Module, finalKill bool, beat float64) {
	if len(s.demons) == 0 {
		return
	}
	if finalKill {
		if !m.isHolding {
			return
		}
	} else if !m.isReady {
		return
	}
	d := s.demons[0]
	s.demons = s.demons[1:]
	d.kill(m, finalKill, beat)
}

func (d *hordeDemon) kill(m *Module, finalKill bool, beat float64) {
	if d.state == hordeDead {
		return
	}
	if finalKill {
		d.pos = d.target
		d.inst.SetGroupOrder(5)
		d.sliceX = -1
	}
	d.pos = [2]float64{d.sliceX - 1, d.target[1]}
	d.inst.Offset = d.pos
	scale := 1.0
	if finalKill {
		scale = 1.35
	}
	m.spawnSlicedDemon(d.pos, demonSmall, beat, true, scale)
	d.state = hordeDead
	d.deathBeat = beat + 1
}

func (s *hordeSequence) failRest(m *Module, beat float64) {
	if len(s.demons) == 0 {
		return
	}
	for len(s.demons) > 1 {
		d := s.demons[0]
		s.demons = s.demons[1:]
		d.kill(m, false, beat)
	}
	s.demons[0].fallFail(m, beat)
}

func (d *hordeDemon) fallFail(m *Module, beat float64) {
	d.state = hordeFall
	d.pos = d.target
	d.fallBeat = beat
	d.fallStartT = m.ctx.BeatToTime(beat)
	d.inst.Offset = d.pos
	d.inst.SetGroupOrder(5)
	d.inst.PlayState("", "HordeIdle", beat, m.ctx.SecPerBeat(beat))
	d.deathBeat = beat + 2
}

func (d *hordeDemon) update(m *Module, beat float64) {
	switch d.state {
	case hordeRush:
		d.pos = [2]float64{d.sliceX, d.target[1]}
		if beat >= d.rushBeat+1.0/7.0 {
			d.state = hordeArrived
		}
	case hordeArrived:
		d.pos = [2]float64{d.sliceX, d.target[1]}
		if d.seq != nil && d.seq.missed {
			d.bite(m, beat)
		}
	case hordeBite:
		d.pos = d.target
		if beat >= d.deathBeat {
			d.state = hordeDead
		}
	case hordeFall:
		t := m.ctx.BeatToTime(beat) - d.fallStartT
		y := d.target[1] + d.cfg(m).fallVelocity[1]*t - 0.5*d.cfg(m).gravity*t*t
		d.pos = [2]float64{d.target[0], y}
		d.rot = deg(d.cfg(m).rotationSpeed) * t
		d.inst.SetRot("", d.rot)
		if beat >= d.deathBeat {
			d.state = hordeDead
		}
	}
	d.inst.Offset = d.pos
}

func (d *hordeDemon) cfg(m *Module) hordeConfig { return m.hordeCfg }

func (d *hordeDemon) bite(m *Module, beat float64) {
	d.state = hordeBite
	d.pos = d.target
	d.inst.Offset = d.pos
	d.inst.SetGroupOrder(2)
	d.inst.PlayState("", "HordeBite", beat, m.ctx.SecPerBeat(beat))
	m.comboDemonMiss(d.seq, true)
	d.deathBeat = beat + 1
}

func (d *hordeDemon) queue(scene *kart.SceneInst, beat float64) {
	if d.state != hordeDead {
		d.inst.Queue(scene, beat, kart.Identity(), 0)
	}
}

func (s *hordeSequence) update(m *Module, beat float64) {
	for _, d := range s.demons {
		d.update(m, beat)
	}
	live := s.demons[:0]
	for _, d := range s.demons {
		if d.state != hordeDead || beat < d.deathBeat {
			live = append(live, d)
		}
	}
	s.demons = live
	if len(s.demons) == 0 && s.ended && beat > s.deathBeat {
		s.deathBeat = beat
	}
}

func (s *hordeSequence) queue(scene *kart.SceneInst, beat float64) {
	for _, d := range s.demons {
		d.queue(scene, beat)
	}
}

func liveHordes(in []*hordeSequence, beat float64) []*hordeSequence {
	out := in[:0]
	for _, h := range in {
		if len(h.demons) > 0 || beat < h.deathBeat {
			out = append(out, h)
		}
	}
	return out
}
