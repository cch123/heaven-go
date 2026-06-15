package samuraislicervl

import (
	"math"
	"math/rand"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

const (
	demonStateSpawn = iota
	demonStateMiss
	demonStateWalk
	demonStateDead
)

type demon struct {
	inst      *kart.Instance
	typ       int
	startBeat float64
	state     int
	curve     kmdata.Curve
	missBeat  float64
	walkBeat  float64
	deathBeat float64
	pos       [2]float64
}

func (m *Module) spawnDemon(beat float64, typ, prepareBeat int) {
	if m.demonT == nil {
		return
	}
	if typ < demonSmall || typ > demonHuge {
		typ = demonSmall
	}
	m.demonSound(beat, typ)
	m.nextPrepareBeat = prepareBeat

	curve := m.curves["spawnCurve"]
	spawn := kart.EvalBezier(curve, 0)
	if beat-1 <= m.lastDemonBeat {
		shiftCurveStart(&curve, 1, -0.75)
		spawn[0] += 1
		spawn[1] -= 0.75
	} else {
		m.lastDemonBeat = beat
	}

	d := &demon{
		inst:      m.demonT.NewInstance(),
		typ:       typ,
		startBeat: beat,
		state:     demonStateSpawn,
		curve:     curve,
		pos:       [2]float64{spawn[0], spawn[1]},
		deathBeat: beat + 6,
	}
	d.configureType(m, beat, "Summon")
	d.inst.Offset = d.pos
	m.demons = append(m.demons, d)

	m.ctx.At(beat+1, func() {
		if d.state == demonStateSpawn {
			d.configureType(m, beat+1, "Idle")
		}
	})
	m.ctx.ScheduleInputActionCond(beat+2, actionSlice,
		func() bool { return d.state != demonStateDead },
		func(state float64, _ engine.Judgment) { d.hit(m, state) },
		func() { d.walk(m, m.ctx.Beat()) })
}

func (m *Module) demonSound(beat float64, typ int) {
	base := "demon" + itoa(typ+1) + "_"
	m.ctx.SoundAt(beat, base+"1", 1)
	m.ctx.SoundAt(beat+0.5, base+"2", 1)
	m.ctx.SoundAt(beat+1, base+"3", 1)
}

func shiftCurveStart(c *kmdata.Curve, dx, dy float64) {
	if len(c.Points) == 0 {
		return
	}
	c.Points[0].P[0] += dx
	c.Points[0].P[1] += dy
	c.Points[0].LH[0] += dx
	c.Points[0].LH[1] += dy
	c.Points[0].RH[0] += dx
	c.Points[0].RH[1] += dy
}

func (d *demon) configureType(m *Module, beat float64, suffix string) {
	for i, rel := range demonRelPaths {
		d.inst.SetActive(rel, i == d.typ)
	}
	rel := demonRelPaths[d.typ]
	d.inst.PlayState(rel, demonStatePrefix[d.typ]+suffix, beat, m.ctx.SecPerBeat(beat))
}

func (d *demon) hit(m *Module, state float64) {
	if d.state == demonStateDead {
		return
	}
	now := m.ctx.Beat()
	if math.Abs(state) >= 1 {
		for _, rel := range demonRelPaths {
			d.inst.SetScale(rel, -1, 1)
		}
		d.state = demonStateMiss
		d.missBeat = now
		d.deathBeat = now + 1
		m.ctx.Sound("OSII")
		m.doSlice(now)
		return
	}
	d.state = demonStateDead
	d.deathBeat = now
	m.demonSuccess(now)
	m.spawnSlicedDemon(d.pos, d.typ, now, false, 1)
}

func (d *demon) walk(m *Module, beat float64) {
	if d.state == demonStateDead {
		return
	}
	d.state = demonStateWalk
	d.walkBeat = beat
	d.deathBeat = beat + 1
	d.configureType(m, beat, "Waddle")
	m.demonMiss(beat)
}

func (d *demon) update(m *Module, beat float64) {
	if d.state == demonStateDead {
		return
	}
	switch d.state {
	case demonStateSpawn:
		if beat >= d.startBeat+1 {
			u := (beat - (d.startBeat + 1)) / 1.1
			if u > 1 {
				d.walk(m, beat)
			} else {
				p := kart.EvalBezier(d.curve, clamp01(u))
				d.pos = [2]float64{p[0], p[1]}
			}
		}
	case demonStateMiss:
		p := kart.EvalBezier(m.curves["missCurve"], clamp01(beat-d.missBeat))
		d.pos = [2]float64{p[0], p[1]}
		if beat >= d.deathBeat {
			d.state = demonStateDead
		}
	case demonStateWalk:
		p := kart.EvalBezier(m.curves["walkCurve"], clamp01(beat-d.walkBeat))
		d.pos = [2]float64{p[0], p[1]}
		if beat >= d.deathBeat {
			d.state = demonStateDead
		}
	}
	d.inst.Offset = d.pos
}

func (d *demon) queue(scene *kart.SceneInst, beat float64) {
	if d.state == demonStateDead {
		return
	}
	d.inst.Queue(scene, beat, kart.Identity(), 0)
}

func (m *Module) demonSuccess(currentBeat float64) {
	m.isDemonSuccess = true
	m.playSamurai("SamuraiSlash", currentBeat, 0.5)
	sound := "HIT1"
	if math.Abs(currentBeat-m.lastSuccessfulBeat) <= 1.5 {
		m.playSamurai("SamuraiSlash2", currentBeat, 0.5)
		if m.rng.Float64() > 0.5 {
			sound = "HIT2_A"
		} else {
			sound = "HIT2_B"
		}
		m.lastSuccessfulBeat = math.Inf(-1)
		m.ctx.At(currentBeat+1, func() {
			if math.IsInf(m.lastSuccessfulBeat, -1) {
				m.playSamurai("SamuraiIdle", currentBeat+1, 0.5)
			}
		})
	} else {
		checkBeat := currentBeat + float64(m.nextPrepareBeat)
		m.ctx.At(checkBeat, func() {
			if math.Abs(checkBeat-m.lastSuccessfulBeat) <= 1.5 {
				m.playSamurai("SamuraiIdle", checkBeat, 0.5)
			}
		})
		m.lastSuccessfulBeat = currentBeat
	}
	m.ctx.Sound(sound)
	if m.thunderEffect {
		m.ctx.Sound(m.randomThunder())
		strike := 2
		if math.Abs(currentBeat-m.lastSuccessfulBeat) <= 1.5 {
			strike = 1
		}
		m.spawnFlash(currentBeat, strike)
	}
}

func (m *Module) demonMiss(beat float64) {
	m.playSamurai("SamuraiMiss", beat, 1)
	m.ctx.Sound("YARARE1")
}

func (m *Module) doSlice(beat float64) {
	if m.isDemonSuccess {
		m.isDemonSuccess = false
		return
	}
	m.playSamurai("SamuraiSlash", beat, 0.5)
	sound := randomChoice(m.rng, "SWING1_A", "SWING1_B", "SWING1_C")
	if math.Abs(beat-m.lastSuccessfulBeat) <= 1.5 {
		m.playSamurai("SamuraiSlash2", beat, 0.5)
		sound = randomChoice(m.rng, "SWING2_A", "SWING2_B", "SWING2_C")
		m.lastSuccessfulBeat += beat * 2
	} else {
		m.lastSuccessfulBeat = beat
	}
	m.ctx.Sound(sound)
}

func randomChoice(rng *rand.Rand, vals ...string) string {
	return vals[rng.Intn(len(vals))]
}

func liveDemons(in []*demon, beat float64) []*demon {
	out := in[:0]
	for _, d := range in {
		if d.state != demonStateDead && beat < d.deathBeat+0.1 {
			out = append(out, d)
		}
	}
	return out
}

var demonRelPaths = []string{"SDemon", "MDemon", "LDemon", "XLDemon"}
var demonStatePrefix = []string{"Small", "Medium", "Large", "XL"}
