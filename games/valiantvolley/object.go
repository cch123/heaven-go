package valiantvolley

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

type objectPlan struct {
	start, distance float64
	typ             int

	juggle           bool
	intervalStart    float64
	intervalLen      float64
	inputs           []float64
	juggleLengths    []float64
	lastJuggle       float64
	lastJuggleLength float64
}

type volleyObject struct {
	plan   objectPlan
	inst   *kart.Instance
	curves map[string]kmdata.Curve
	typ    int

	hit, barely, missed bool
	dead                bool
	dieBeat             float64
	hitBeat             float64
	hitPitch            float64
	currentJuggle       int
	spawnSec            float64
}

func (m *Module) spawnVolleyObject(plan objectPlan) {
	if m.objectT == nil {
		return
	}
	o := &volleyObject{
		plan: plan, typ: plan.typ,
		inst:     m.objectT.NewInstance(),
		curves:   m.curves,
		dieBeat:  plan.start + plan.distance*5,
		hitBeat:  math.Inf(1),
		hitPitch: 1,
		spawnSec: m.ctx.BeatToTime(plan.start),
	}
	o.inst.SetActive("", true)
	o.inst.SetActive("Object/missImpact", false)
	if plan.typ == objFruit {
		if spr := m.objectComp.Sprites["fruitSprite"]; spr != "" {
			o.inst.SetSprite("Object", spr)
		} else {
			o.inst.SetSprite("Object", "fruit")
		}
	}
	m.objects = append(m.objects, o)
	m.scheduleObjectActions(o)
}

func (m *Module) scheduleObjectActions(o *volleyObject) {
	p := o.plan
	m.ctx.At(p.start+p.distance, func() {
		beat := p.start + p.distance
		o.playAutoVolleyAnim(beat)
		o.autoAnt(m, 0, beat, 1)
	})
	m.ctx.At(p.start+p.distance*2, func() {
		beat := p.start + p.distance*2
		o.playAutoVolleyAnim(beat)
		o.autoAnt(m, 1, beat, 1)
	})
	m.scheduleObjectInput(o, p.start+p.distance*3)
	for _, input := range p.inputs {
		input := input
		m.ctx.At(input, func() {
			pitch := o.pitchFor(input)
			o.playJuggle(m, input, pitch)
			o.autoAnt(m, 0, input, pitch)
		})
		m.ctx.At(input+p.distance, func() {
			pitch := o.pitchFor(input)
			o.playJuggle(m, input+p.distance, pitch)
			o.autoAnt(m, 1, input+p.distance, pitch)
		})
		m.scheduleObjectInput(o, input+p.distance*2)
	}
}

func (m *Module) scheduleObjectInput(o *volleyObject, target float64) {
	action := 0
	if o.typ == objFruit {
		action = actionFruit
	}
	m.ctx.ScheduleInputAction(target, action,
		func(state float64, _ engine.Judgment) { o.hitInput(m, target, state) },
		func() { o.miss(m, target) })
}

func (o *volleyObject) playAutoVolleyAnim(beat float64) {
	state, timeScale := o.autoVolleyAnim(beat)
	if state == "" || o.inst == nil || o.dead || o.missed {
		return
	}
	o.inst.PlayState("", state, beat, timeScale)
}

func (o *volleyObject) autoVolleyAnim(beat float64) (string, float64) {
	if len(o.plan.inputs) == 0 || o.isLastJuggleWorkerBeat(beat) {
		o.currentJuggle = 0
		return "ObjectHit", 0.5
	}

	// Unity reuses the first juggle length for the automatic second worker hit:
	// that branch resets currentJuggle before playing ObjectJuggle. Keeping the
	// same reset here prevents long multi-intervals from drifting animation
	// speed before the player's return windows begin.
	if nearly(beat, o.plan.start+o.plan.distance*2) {
		o.currentJuggle = 0
	}
	length := o.currentJuggleLength()
	if length <= 0 {
		length = o.plan.distance
	}
	o.currentJuggle++
	return "ObjectJuggle", 0.5 / length
}

func (o *volleyObject) isLastJuggleWorkerBeat(beat float64) bool {
	return o.plan.lastJuggle != 0 &&
		(nearly(beat, o.plan.lastJuggle) || nearly(beat, o.plan.lastJuggle+o.plan.distance))
}

func (o *volleyObject) autoAnt(m *Module, idx int, beat, pitch float64) {
	if o.dead || idx < 0 || idx >= len(m.ants) || m.ants[idx] == nil {
		return
	}
	action := "dirtHit"
	sound := "dirtHit"
	if o.typ == objFruit {
		action = "fruitHit"
		sound = "fruitHit"
	}
	m.ants[idx].action(m, beat, action, pitch)
	m.ctx.SoundPitch(sound, 1, pitch)
}

func (o *volleyObject) hitInput(m *Module, target, state float64) {
	if o.dead || o.missed {
		return
	}
	o.hitBeat = target
	if state >= 1 || state <= -1 {
		o.barely = true
		o.hit = false
		o.hitPitch = o.pitchFor(target - o.plan.distance*2)
		m.bopStatus = bopAngry
		m.ctx.PlayCommon("nearMiss")
		o.inst.PlayState("", "ObjectBarely", target, 0.5)
		return
	}
	o.hit = true
	o.barely = false
	o.hitPitch = o.pitchFor(target - o.plan.distance*2)
	if o.typ == objDirt {
		m.volleyHit(target, state, o.hitPitch)
		m.ctx.SoundPitch("dirtHit", 1, o.hitPitch)
	} else {
		m.fruitHit(target, state, o.hitPitch)
		m.ctx.SoundPitch("fruitHit", 1, o.hitPitch)
	}
	o.inst.PlayState("", "ObjectHit", target, 0.5)
}

func (o *volleyObject) miss(m *Module, beat float64) {
	if o.dead {
		return
	}
	o.missed = true
	o.hit = false
	o.barely = false
	o.inst.SetSprite("Object", "")
	o.inst.SetActive("Object/missImpact", true)
	if o.typ == objDirt {
		m.ctx.Sound("dirtMiss")
	} else {
		m.ctx.Sound("fruitMiss")
	}
	m.bopStatus = bopAngry
	if a := m.ants[2]; a != nil {
		a.cantBop = false
		a.isPreparing = false
	}
	o.dieBeat = beat + 0.25
}

func (m *Module) volleyHit(beat, state, pitch float64) {
	if a := m.ants[2]; a != nil {
		a.action(m, beat, "dirtHit", pitch)
	}
	m.finishPlayerHit(state)
}

func (m *Module) fruitHit(beat, state, pitch float64) {
	if a := m.ants[2]; a != nil {
		a.action(m, beat, "fruitHit", pitch)
	}
	m.finishPlayerHit(state)
}

func (m *Module) finishPlayerHit(state float64) {
	if m.ants[0] == nil || !m.ants[0].isPreparing || !m.ants[0].justHit {
		for _, a := range m.ants {
			if a != nil {
				a.cantBop = false
			}
		}
	}
	if state <= -1 || state >= 1 {
		m.ctx.PlayCommon("nearMiss")
		m.bopStatus = bopAngry
		if a := m.ants[2]; a != nil {
			a.cantBop = false
			a.isPreparing = false
		}
		return
	}
	m.bopStatus = bopHappy
}

func (o *volleyObject) playJuggle(m *Module, beat, pitch float64) {
	length := o.currentJuggleLength()
	if length <= 0 {
		length = o.plan.distance
	}
	o.inst.PlayState("", "ObjectJuggle", beat, 0.5/length)
	o.hitPitch = pitch
	o.currentJuggle++
}

func (o *volleyObject) pitchFor(input float64) float64 {
	if o.plan.lastJuggle != 0 && nearly(input, o.plan.lastJuggle) {
		return 1
	}
	if len(o.plan.inputs) > 0 {
		return 0.8
	}
	return 1
}

func (o *volleyObject) currentJuggleLength() float64 {
	if len(o.plan.juggleLengths) == 0 {
		return o.plan.distance
	}
	idx := o.currentJuggle
	if idx >= len(o.plan.juggleLengths) {
		idx = 0
	}
	return o.plan.juggleLengths[idx]
}

func (o *volleyObject) targetBeat() float64 {
	if math.IsInf(o.hitBeat, 1) {
		return o.plan.start + o.plan.distance*3
	}
	return o.hitBeat
}

func (o *volleyObject) expectsInputAt(beat float64) bool {
	if math.Abs(beat-(o.plan.start+o.plan.distance*3)) < 0.25 {
		return true
	}
	for _, input := range o.plan.inputs {
		if math.Abs(beat-(input+o.plan.distance*2)) < 0.25 {
			return true
		}
	}
	return false
}

func (o *volleyObject) update(m *Module, t, beat float64) {
	if beat < o.plan.start {
		return
	}
	if o.missed {
		if beat > o.dieBeat {
			o.dead = true
		}
		return
	}
	if o.hit || o.barely {
		u := (beat - o.hitBeat) / o.postHitLength()
		if u > 1 {
			o.dead = true
		}
		return
	}
	if beat > o.plan.start+o.plan.distance*3+1 {
		o.dead = true
	}
}

func (o *volleyObject) queue(sc *kart.SceneInst, t, beat float64) {
	if o.dead || o.inst == nil || beat < o.plan.start {
		return
	}
	pos := o.pos(beat)
	o.inst.Offset = [2]float64{pos[0], pos[1]}
	if o.inst.CurrentState("") != "ObjectJuggle" {
		speed := 120.0
		if o.typ == objFruit {
			speed = 600
		}
		o.inst.SetRot("", -speed*(t-o.spawnSec)*math.Pi/180)
	} else {
		o.inst.SetRot("", 0)
	}
	o.inst.Queue(sc, beat, kart.Identity(), 0)
}

func (o *volleyObject) pos(beat float64) [3]float64 {
	switch {
	case o.hit:
		return kart.EvalBezier(o.curve("object.hitCurve"), clamp01((beat-o.hitBeat)/o.postHitLength()))
	case o.barely:
		return kart.EvalBezier(o.curve("object.barelyCurve"), clamp01((beat-o.hitBeat)/o.postHitLength()))
	default:
		name, u := o.preHitCurve(beat)
		return kart.EvalBezier(o.curve(name), u)
	}
}

func (o *volleyObject) preHitCurve(beat float64) (string, float64) {
	d := o.plan.distance
	if o.plan.lastJuggle != 0 && o.plan.lastJuggleLength > 0 {
		if beat >= o.plan.lastJuggle && beat < o.plan.lastJuggle+o.plan.lastJuggleLength {
			return "object.bounceCurve1", clamp01((beat - o.plan.lastJuggle) / o.plan.lastJuggleLength)
		}
		second := o.plan.lastJuggle + d
		if beat >= second && beat < second+o.plan.lastJuggleLength {
			return "object.bounceCurve2", clamp01((beat - second) / o.plan.lastJuggleLength)
		}
	}
	switch {
	case beat < o.plan.start+d:
		return "object.enterCurve", clamp01((beat - o.plan.start) / d)
	case beat < o.plan.start+2*d:
		return "object.bounceCurve1", clamp01((beat - (o.plan.start + d)) / d)
	default:
		return "object.bounceCurve2", clamp01((beat - (o.plan.start + 2*d)) / d)
	}
}

func (o *volleyObject) postHitLength() float64 {
	if o.plan.lastJuggle != 0 && nearly(o.hitBeat, o.plan.lastJuggle+o.plan.distance*2) && o.plan.lastJuggleLength > 0 {
		// VolleyObject.Update switches final multi-juggle hit/barely travel to
		// lastJuggleLength so the return curve resolves at the interval end.
		return o.plan.lastJuggleLength
	}
	return o.plan.distance
}

func (o *volleyObject) curve(name string) kmdata.Curve {
	return o.curves[name]
}
