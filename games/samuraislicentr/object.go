package samuraislicentr

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

const (
	flyNearMiss = -3
	flyHeld     = -2
	flySliced   = -1
	flyIncoming = 0
	flyLaunched = 1
	flyFishHop  = 2
)

type samObject struct {
	inst       *kart.Instance
	typ        int
	cash       int
	startBeat  float64
	flyProg    int
	curve      string
	dead       bool
	killBeat   float64
	rot        float64
	second     *samObject
	heldBy     *carryChild
	heldRight  bool
	isDebris   bool
	splatBeat  float64
	catchSound bool
}

type carryChild struct {
	inst     *kart.Instance
	start    float64
	killBeat float64
	walking  bool
}

func (m *Module) spawnObject(beat float64, typ, cash int) {
	if m.objT == nil {
		return
	}
	o := &samObject{
		inst:      m.objT.NewInstance(),
		typ:       typ,
		cash:      cash,
		startBeat: beat,
		flyProg:   flyIncoming,
		curve:     "InCurve",
		killBeat:  beat + 12,
		rot:       beat*2*math.Pi + m.rng.Float64()*math.Pi,
	}
	o.playObjectState(m, objectIdleState(typ), beat)
	m.objects = append(m.objects, o)
	m.ctx.Sound("ntrSamurai_in00")
	if typ == objDemon {
		m.ctx.SoundAt(beat+1, "ntrSamurai_in01", 1.5)
		m.ctx.SoundAt(beat+1.5, "ntrSamurai_in01", 1.25)
		m.ctx.SoundAt(beat+2, "ntrSamurai_in01", 1)
	}
	o.scheduleLaunch(m, beat, 2)
}

func (o *samObject) scheduleLaunch(m *Module, start, timer float64) {
	target := start + timer
	m.ctx.ScheduleInputActionCond(target, actionStep, func() bool { return !o.dead },
		func(state float64, _ engine.Judgment) { o.launchSuccess(m, start, timer, state) },
		func() { o.launchMiss(m, start) })
	release := m.ctx.ScheduleInputActionRelease(start+1.75, actionStep,
		func(float64, engine.Judgment) { m.doUnStep(m.ctx.Beat()) },
		func() {})
	release.NoScore = true
}

func (o *samObject) scheduleHit(m *Module, start, timer float64) {
	target := start + timer
	m.ctx.ScheduleInputActionCond(target, actionSlice, func() bool { return !o.dead },
		func(state float64, _ engine.Judgment) { o.hitSuccess(m, start, timer, state) },
		func() { o.hitMiss(m, start, timer) })
}

func (o *samObject) launchSuccess(m *Module, start, timer, state float64) {
	if math.Abs(state) >= 1 {
		o.startBeat = start + timer
		o.curve = "NgLaunchCurve"
		o.flyProg = flyNearMiss
		m.ctx.SoundPitch("ntrSamurai_launchImpact", 1, 2)
		o.doSplat(m, o.startBeat+2)
		return
	}
	o.doLaunch(m)
	m.ctx.SoundPitch("ntrSamurai_launchImpact", 1, 0.85+m.rng.Float64()*0.2)
}

func (o *samObject) launchMiss(m *Module, start float64) {
	if o.flyProg == flyFishHop {
		o.doSplat(m, start+2.215)
		return
	}
	o.doSplat(m, start+3)
}

func (o *samObject) doLaunch(m *Module) {
	switch o.typ {
	case objFish:
		if o.flyProg == flyFishHop {
			o.flyProg = flyLaunched
			o.curve = "LaunchCurve"
			o.scheduleHit(m, o.startBeat+4, 2)
			m.playMackerel(0.25)
		} else {
			o.flyProg = flyFishHop
			o.curve = ""
			o.scheduleLaunch(m, o.startBeat+2, 2)
			m.playMackerel(0.8)
		}
	case objDemon:
		o.flyProg = flyLaunched
		o.curve = "LaunchHighCurve"
		o.scheduleHit(m, o.startBeat+2, 4)
	default:
		o.flyProg = flyLaunched
		o.curve = "LaunchCurve"
		o.scheduleHit(m, o.startBeat+2, 2)
	}
}

func (m *Module) playMackerel(vol float64) {
	n := 1 + m.rng.Intn(3)
	m.ctx.SoundPitch("holy_mackerel"+itoa(n), vol, 0.95+m.rng.Float64()*0.1)
}

func (o *samObject) hitSuccess(m *Module, start, timer, state float64) {
	hitBeat := start + timer
	if math.Abs(state) >= 1 {
		o.startBeat = hitBeat
		o.curve = "NgDebrisCurve"
		o.flyProg = flyNearMiss
		m.ctx.Sound("ntrSamurai_ng")
		o.doSplat(m, hitBeat+2)
		return
	}
	o.flyProg = flySliced
	o.curve = "DebrisRightCurve"
	o.startBeat = hitBeat
	o.playObjectState(m, debrisState(o.typ, true), hitBeat)
	if m.rng.Float64() >= 0.5 {
		m.ctx.SoundPitch("ntrSamurai_just00", 1, 0.95+m.rng.Float64()*0.1)
	} else {
		m.ctx.SoundPitch("ntrSamurai_just01", 1, 0.95+m.rng.Float64()*0.1)
	}
	if o.typ == objMelon2B2T {
		m.ctx.Sound("melon_dig")
		m.emitBurst(o.position(m, hitBeat), 12, [4]float64{0.65, 0.95, 0.25, 1})
	}
	if o.cash > 0 {
		o.emitMoney(m, o.cash, hitBeat)
		if o.cash > 2 {
			m.ctx.SoundPitch("ntrSamurai_scoreMany", 1, 0.95+m.rng.Float64()*0.1)
		} else {
			m.ctx.SoundPitch("ntrSamurai_ng", 1, 0.95+m.rng.Float64()*0.1)
		}
	}
	other := m.newDebris(o.typ, hitBeat, false)
	other.rot = o.rot
	o.second = other
	m.objects = append(m.objects, other)
}

func (o *samObject) hitMiss(m *Module, start, timer float64) {
	flyDur := 3.0
	if o.typ == objDemon {
		flyDur = 5
	}
	o.doSplat(m, start+flyDur)
}

func (m *Module) newDebris(typ int, beat float64, right bool) *samObject {
	o := &samObject{
		inst:      m.objT.NewInstance(),
		typ:       typ,
		startBeat: beat,
		flyProg:   flySliced,
		curve:     "DebrisLeftCurve",
		isDebris:  true,
		heldRight: right,
		killBeat:  beat + 8,
	}
	o.playObjectState(m, debrisState(typ, false), beat)
	return o
}

func (o *samObject) doSplat(m *Module, beat float64) {
	if o.dead || o.splatBeat > 0 {
		return
	}
	o.splatBeat = beat
	m.ctx.SoundAt(beat, "item_splat", 1)
	m.ctx.At(beat, func() {
		o.playObjectState(m, splatState(o.typ), beat)
		if o.typ == objMelon2B2T {
			m.emitBurst(o.position(m, beat), 14, [4]float64{0.65, 0.95, 0.25, 1})
		}
	})
	m.ctx.At(beat+3, func() {
		o.dead = true
		o.killBeat = beat + 3
	})
}

func (o *samObject) update(m *Module, beat, dt float64) {
	if o.inst == nil || o.dead {
		return
	}
	if o.splatBeat > 0 && beat >= o.splatBeat {
		return
	}
	pos := o.position(m, beat)
	o.inst.Offset = pos
	switch o.flyProg {
	case flyNearMiss:
		if (beat-o.startBeat)/2 < 1 {
			o.rot += math.Pi * dt
		}
	case flySliced:
		o.rot += sign(o.isDebris) * 2 * math.Pi * dt
		if beat >= o.startBeat+1 && o.heldBy == nil {
			o.catchByChild(m)
		}
	case flyFishHop:
		if (beat-(o.startBeat+2))/2 < 1.215 {
			o.rot -= 4 * math.Pi * dt
		}
	case flyLaunched:
		o.rot += 6 * math.Pi * dt
	default:
		o.rot -= 2 * math.Pi * dt
	}
	o.inst.Rot = o.rot
}

func (o *samObject) position(m *Module, beat float64) [2]float64 {
	switch o.flyProg {
	case flyNearMiss:
		p := kart.EvalBezier(m.curves[o.curve], clamp01((beat-o.startBeat)/2))
		return [2]float64{p[0], p[1]}
	case flyHeld:
		if o.heldBy != nil {
			return o.heldBy.holdPos(m, beat, o.heldRight)
		}
	case flySliced:
		p := kart.EvalBezier(m.curves[o.curve], clamp01(beat-o.startBeat))
		return [2]float64{p[0], p[1]}
	case flyFishHop:
		base := nodePos(m.ctx.Assets, m.objectComp.Refs["doubleLaunchPos"])
		jumpPos := math.Min((beat-(o.startBeat+2))/2, 1.215)
		if jumpPos < 0 {
			jumpPos = 0
		}
		yMul := jumpPos*2 - 1
		yWeight := -(yMul * yMul) + 1
		return [2]float64{base[0], base[1] + 4.5*yWeight}
	case flyLaunched:
		dur := 3.0
		if o.typ == objDemon {
			dur = 5
		}
		start := o.startBeat + 2
		if o.typ == objFish {
			start = o.startBeat + 4
		}
		p := kart.EvalBezier(m.curves[o.curve], clamp01((beat-start)/dur))
		return [2]float64{p[0], p[1]}
	default:
		p := kart.EvalBezier(m.curves["InCurve"], clamp01((beat-o.startBeat)/3))
		return [2]float64{p[0], p[1]}
	}
	return o.inst.Offset
}

func (o *samObject) catchByChild(m *Module) {
	if !o.catchSound {
		m.ctx.Sound("ntrSamurai_catch")
		o.catchSound = true
	}
	k := m.createChild(o.startBeat + 1)
	if k == nil {
		return
	}
	o.heldBy, o.heldRight, o.flyProg = k, true, flyHeld
	if o.second != nil {
		o.second.heldBy, o.second.heldRight, o.second.flyProg = k, false, flyHeld
	}
}

func (m *Module) createChild(start float64) *carryChild {
	if m.childT == nil {
		return nil
	}
	k := &carryChild{inst: m.childT.NewInstance(), start: start, killBeat: start + 6}
	k.inst.PlayState("", "ChildBeat", start, 0.5)
	k.inst.SetGroupOrder(7)
	m.kids = append(m.kids, k)
	return k
}

func (k *carryChild) queue(m *Module, beat float64) {
	if k == nil || k.inst == nil {
		return
	}
	u := clamp01((beat - (k.start + 1)) / 4)
	p0 := nodePos(m.ctx.Assets, m.childComp.Refs["WalkPos0"])
	p1 := nodePos(m.ctx.Assets, m.childComp.Refs["WalkPos1"])
	k.inst.Offset = [2]float64{lerp(p0[0], p1[0], u), lerp(p0[1], p1[1], u)}
	if u > 0 && !k.walking {
		k.walking = true
		k.inst.PlayState("", "ChildWalk", k.start+1, 0.5)
	}
	k.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
}

func (k *carryChild) holdPos(m *Module, beat float64, right bool) [2]float64 {
	if k == nil || k.inst == nil {
		return [2]float64{}
	}
	u := clamp01((beat - (k.start + 1)) / 4)
	p0 := nodePos(m.ctx.Assets, m.childComp.Refs["WalkPos0"])
	p1 := nodePos(m.ctx.Assets, m.childComp.Refs["WalkPos1"])
	root := kart.Translate(lerp(p0[0], p1[0], u), lerp(p0[1], p1[1], u))
	rel := "child_armL/hold_pos"
	if right {
		rel = "child_armR/hold_pos"
	}
	if aff, ok := k.inst.NodeWorld(rel, root); ok {
		x, y := aff.Apply(0, 0)
		return [2]float64{x, y}
	}
	return [2]float64{root.Tx, root.Ty}
}

func (o *samObject) queue(m *Module, beat float64) {
	if o == nil || o.inst == nil || o.dead {
		return
	}
	o.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
}

func (o *samObject) playObjectState(m *Module, state string, beat float64) {
	if o.inst != nil {
		o.inst.PlayState("Object", state, beat, 0.5)
	}
}

func objectIdleState(typ int) string {
	switch typ {
	case objFish:
		return "ObjFish"
	case objDemon:
		return "ObjDemon"
	case objMelon2B2T:
		return "ObjMelonPickel"
	default:
		return "ObjMelon"
	}
}

func debrisState(typ int, first bool) string {
	switch typ {
	case objFish:
		return "ObjFishDebris"
	case objDemon:
		if first {
			return "ObjDemonDebris01"
		}
		return "ObjDemonDebris02"
	case objMelon2B2T:
		if first {
			return "ObjMelonPickelDebris01"
		}
		return "ObjMelonPickelDebris02"
	default:
		return "ObjMelonDebris"
	}
}

func splatState(typ int) string {
	switch typ {
	case objFish:
		return "ObjFishSplat"
	case objDemon:
		return "ObjDemonSplat"
	case objMelon2B2T:
		return "ObjMelonPickelSplat"
	default:
		return "ObjMelonSplat"
	}
}

func (o *samObject) emitMoney(m *Module, n int, beat float64) {
	pos := o.position(m, beat)
	m.emitBurst(pos, n, [4]float64{1, 0.82, 0.12, 1})
}

func (m *Module) emitBurst(pos [2]float64, n int, col [4]float64) {
	for i := 0; i < n; i++ {
		a := m.rng.Float64() * 2 * math.Pi
		spd := 0.9 + m.rng.Float64()*1.7
		m.parts = append(m.parts, ambientParticle{
			born: m.ctx.Beat(), life: 1.2 + m.rng.Float64()*0.8,
			x: pos[0], y: pos[1],
			vx: math.Cos(a) * spd, vy: math.Sin(a)*spd + 1,
			size: 0.06 + m.rng.Float64()*0.06,
			col:  col,
		})
	}
}

func nodePos(as *kart.Assets, path string) [2]float64 {
	for _, n := range as.Rig.Nodes {
		if n.Path == path {
			return n.Pos
		}
	}
	return [2]float64{}
}
