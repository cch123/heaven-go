package builttoscalervl

import (
	"math"
	"math/rand"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

type customBounceItem struct {
	time int
	pos  int
}

type scheduledWidget struct {
	beat, length        float64
	currentPos, nextPos int
	id                  int
	bounceItems         []customBounceItem
	endTime             int
	isShoot, mute       bool
}

type rod struct {
	m    *Module
	inst *kart.Instance

	startBeat, lengthBeat float64
	currentBeat           float64
	currentPos, nextPos   int
	id                    int
	time, endTime         int
	customBounce          []customBounceItem
	isShoot               bool

	currentCurve  kmdata.Curve
	active        bool
	dead          bool
	isMiss        bool
	isNearlyMiss  bool
	clearNearBeat float64
	fallingAngle  float64
	missAngle     float64

	squares []*square
}

func (m *Module) spawnRod(w scheduledWidget) {
	r := &rod{
		m: m, inst: m.rodT.NewInstance(),
		startBeat: w.beat, lengthBeat: w.length, currentBeat: w.beat,
		currentPos: w.currentPos, nextPos: w.nextPos, id: w.id,
		customBounce: append([]customBounceItem{}, w.bounceItems...),
		endTime:      w.endTime, isShoot: w.isShoot,
		fallingAngle: m.fallingAngle * (rand.Float64()*2 - 1),
		missAngle:    m.missAngle,
	}
	m.rods = append(m.rods, r)
	if w.isShoot {
		endBeat := w.beat + w.length*float64(w.endTime)
		if !w.mute {
			m.ctx.SoundAt(endBeat-2*w.length, "preparestart", 1)
			m.ctx.SoundAtOff(endBeat-w.length, "prepareend", 1, 0.325)
		}
		r.squares = m.spawnSquares(endBeat)
	}
	m.at(w.beat, func() {
		r.init()
		if m.block(2) != nil && m.blocks[2].isOpen {
			m.playBlockIdle(2, w.beat)
		}
	})
}

func (r *rod) init() {
	if r.dead {
		return
	}
	r.active = true
	r.currentBeat = r.startBeat
	r.time = 0
	r.bounceRecursion(r.startBeat, r.lengthBeat, r.currentPos, r.nextPos, 0, true)
	r.setParameters(r.currentPos, r.nextPos)
}

func (r *rod) bounceRecursion(beat, length float64, currentPos, nextPos, timeBefore int, playBounce bool) {
	m := r.m
	if inRange(currentPos) && playBounce {
		sound := []string{"left", "middleLeft", "middleRight", "right"}[currentPos]
		m.soundAt(beat, sound, 1)
		m.at(beat, func() { m.playBlockBounce(currentPos, beat+length) })
	}

	m.at(beat, func() {
		r.currentBeat = beat
		r.time = timeBefore + 1
		r.setParameters(currentPos, nextPos)
	})

	if !inRange(nextPos) {
		m.at(beat+length, r.end)
	} else if nextPos == 2 {
		targetBeat := beat + length
		if r.isShoot && timeBefore+1 == r.endTime {
			m.at(beat, func() { m.playBlockPrepare(nextPos, targetBeat) })
			m.ctx.ScheduleInputActionCond(targetBeat, actionAlt,
				func() bool { return m.block(2) != nil && m.blocks[2].isPrepare },
				func(state float64, _ engine.Judgment) { r.shootOnHit(state) },
				r.shootOnMiss,
			)
		} else {
			m.ctx.ScheduleInputCond(targetBeat,
				func() bool { return m.block(2) != nil && !m.blocks[2].isOpen },
				func(state float64, _ engine.Judgment) { r.bounceOnHit(state) },
				r.bounceOnMiss,
			)
		}
	} else {
		nextTime := timeBefore + 1
		following := followingPos(currentPos, nextPos, nextTime, r.customBounce)
		r.bounceRecursion(beat+length, length, nextPos, following, nextTime, true)
	}

	if inRange(currentPos) {
		m.at(beat+length, func() { m.playBlockIdle(currentPos, beat+length) })
	}
}

func (r *rod) setParameters(currentPos, nextPos int) {
	r.currentPos = currentPos
	r.nextPos = nextPos
	key := [2]int{currentPos, nextPos}
	idx, ok := curveMap[key]
	if r.isShoot && r.time == r.endTime {
		idx, ok = curveMapHigh[key]
	} else if !inRange(nextPos) {
		idx, ok = curveMapOut[key]
	}
	if !ok {
		r.currentCurve = kmdata.Curve{}
		return
	}
	r.currentCurve = r.m.curve(idx)
}

func (r *rod) bounceOnHit(state float64) {
	following := followingPos(r.currentPos, r.nextPos, r.time, r.customBounce)
	if state >= 1 || state <= -1 {
		r.isNearlyMiss = true
		r.clearNearBeat = r.currentBeat + 2*r.lengthBeat
		r.m.playBlockBounceNearlyMiss(r.nextPos)
		r.bounceRecursion(r.currentBeat+r.lengthBeat, r.lengthBeat, r.nextPos, following, r.time, false)
		return
	}
	r.m.ctx.Sound("middleRight")
	r.m.playBlockBounce(r.nextPos, r.currentBeat+2*r.lengthBeat)
	r.bounceRecursion(r.currentBeat+r.lengthBeat, r.lengthBeat, r.nextPos, following, r.time, false)
}

func (r *rod) bounceOnMiss() {
	r.m.ctx.Sound("tink")
	r.falling()
}

func (r *rod) falling() {
	r.currentCurve = r.m.missCurve(0)
	if r.currentPos <= r.nextPos {
		r.currentCurve = r.m.missCurve(1)
	}
	r.currentBeat = r.m.ctx.Beat()
	r.isMiss = true
	r.m.playBlockBounceMiss(r.nextPos)
	r.m.at(r.currentBeat+r.lengthBeat*0.2, func() { r.inst.SetOrder("", 1) })
	r.m.at(r.currentBeat+r.lengthBeat, func() {
		r.m.playBlockIdle(r.nextPos, r.currentBeat+r.lengthBeat)
		r.end()
	})
}

func (r *rod) shootOnHit(state float64) {
	if state >= 1 || state <= -1 {
		r.falling()
		return
	}
	r.m.playBlockShoot(r.nextPos)
	for _, sq := range r.squares {
		sq.dead = true
	}
	r.m.spawnAssembled(r.m.ctx.Beat())
	r.end()
}

func (r *rod) shootOnMiss() {
	if r.m.block(2) != nil && r.m.blocks[2].isPrepare {
		r.inst.SetOrder("", 1)
		r.m.playBlockShootMiss(r.nextPos)
		r.m.at(r.currentBeat+2*r.lengthBeat, r.end)
		return
	}
	r.falling()
}

func (r *rod) end() { r.dead = true }

func (r *rod) update(beat float64) {
	if !r.active || r.dead {
		return
	}
	if r.isNearlyMiss && r.clearNearBeat > 0 && beat >= r.clearNearBeat {
		r.isNearlyMiss = false
	}
	prog := (beat - r.currentBeat) / r.lengthBeat
	if prog < 0 {
		prog = 0
	}
	if prog > 1 {
		prog = 1 + (prog-1)*0.5
	}
	sampleT := prog
	if !r.isMiss && r.currentPos > r.nextPos {
		sampleT = 1 - prog
	}
	p := kart.EvalBezier(r.currentCurve, sampleT)
	r.inst.Offset = [2]float64{p[0], p[1]}
	if r.isMiss {
		r.inst.Rot = deg(r.fallingAngle * prog)
	} else if r.isNearlyMiss {
		r.inst.Rot = deg(r.missAngle * (1 - prog))
	} else {
		r.inst.Rot = 0
	}

	spin := math.Mod((beat-r.startBeat)*0.5/r.lengthBeat, 0.3)
	if spin < 0 {
		spin += 0.3
	}
	norm := spin / 0.3
	if r.currentPos > r.nextPos {
		norm = 1 - norm
	}
	r.inst.PlayNormalized("", "Animations/rod_rotate", norm)
}
