package monkeywatch

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

type watchMonkey struct {
	mod       *Module
	inst      *kart.Instance
	beat      float64
	hole      int
	dir       int
	pink      bool
	dead      bool
	deadBeat  float64
	inputDone bool
}

func (m *Module) spawnMonkey(beat float64, pink bool, instant bool) {
	if m.monkeyAtBeat(beat) != nil {
		return
	}
	t := m.yellowT
	if pink {
		t = m.pinkT
	}
	if t == nil || len(m.holePaths) == 0 {
		return
	}
	holeIdx := m.watchHoleIdx
	m.watchHoleIdx = (m.watchHoleIdx + 1) % len(m.holePaths)
	mk := &watchMonkey{mod: m, inst: t.NewInstance(), beat: beat, hole: holeIdx, dir: monkeyDirection(holeIdx), pink: pink}
	if instant {
		mk.playAppear(m.ctx.Beat()-1, true)
	} else {
		mk.playAppear(m.ctx.Beat(), false)
	}
	m.ctx.Scene.PlayState(m.holePaths[holeIdx], "HoleOpen", m.ctx.Beat(), 0.4)
	m.monkeys = append(m.monkeys, mk)
	if len(m.monkeys) > m.maxMonkeys {
		m.monkeys[0].disappear(m.ctx.Beat())
	}
}

func (m *Module) monkeyAtBeat(beat float64) *watchMonkey {
	for _, mk := range m.monkeys {
		if math.Abs(mk.beat-beat) < 1e-4 && !mk.dead {
			return mk
		}
	}
	return nil
}

func (mk *watchMonkey) playAppear(beat float64, instant bool) {
	state := "Appear"
	if mk.pink {
		state = "PinkAppear"
	}
	if instant {
		mk.inst.PlayState("", state, beat-1, 0.4)
		return
	}
	mk.inst.PlayState("", state, beat, 0.4)
}

func (mk *watchMonkey) prepare(prepareBeat, inputBeat float64) {
	state := "Prepare" + itoa(mk.dir)
	if mk.pink {
		state = "Pink" + state
	}
	mk.inst.PlayState("", state, prepareBeat-0.25, 0.4)
	canHit := func() bool { return !mk.dead && !mk.inputDone }
	mk.mod.ctx.ScheduleInputCond(inputBeat, canHit,
		func(state float64, _ engine.Judgment) { mk.just(inputBeat, state) },
		func() { mk.miss() })
}

func (mk *watchMonkey) just(targetBeat, state float64) {
	if mk.dead || mk.inputDone {
		return
	}
	mk.inputDone = true
	barely := math.Abs(state) >= 1
	if barely {
		mk.mod.ctx.PlayCommon("nearMiss")
	} else if mk.pink {
		mk.mod.ctx.Sound("clapOffbeat")
	} else {
		mk.mod.ctx.Sound(soundChoice("clapOnbeat", targetBeat, 5))
	}
	mk.mod.moveHand(targetBeat)
	mk.mod.playerClap(targetBeat, mk.pink, barely)
	stateName := "Clap" + itoa(mk.dir)
	if mk.pink {
		stateName = "Pink" + stateName
	}
	mk.inst.PlayState("", stateName, mk.mod.ctx.Beat(), 0.4)
	mk.mod.ctx.At(targetBeat+1, func() {
		which := "Just"
		if barely {
			which = "Barely"
		}
		if mk.pink {
			which = "Pink" + which
		}
		mk.inst.PlayState("", which, targetBeat+1, 0.4)
	})
}

func (mk *watchMonkey) miss() {
	if mk.dead || mk.inputDone {
		return
	}
	mk.inputDone = true
	state := "Miss"
	if mk.pink {
		state = "PinkMiss"
	}
	mk.inst.PlayState("", state, mk.mod.ctx.Beat(), 0.4)
	mk.mod.moveHand(mk.mod.ctx.Beat())
}

func (mk *watchMonkey) disappear(beat float64) {
	if mk.dead {
		return
	}
	mk.dead = true
	mk.deadBeat = beat
	state := "Appear"
	if mk.pink {
		state = "PinkAppear"
	}
	mk.inst.PlayState("", state, beat-0.5, -0.4)
	if mk.hole >= 0 && mk.hole < len(mk.mod.holePaths) {
		mk.mod.ctx.Scene.PlayState(mk.mod.holePaths[mk.hole], "HoleClose", beat+0.25, 0.4)
	}
}

func (mk *watchMonkey) queue(sc *kart.SceneInst, beat float64, base kart.Aff, z float64) {
	if mk.hole < 0 || mk.hole >= len(mk.mod.holePos) {
		return
	}
	pos := mk.mod.holePos[mk.hole]
	mk.inst.Queue(sc, beat, base.Mul(kart.Translate(pos[0], pos[1])), z)
}

func (m *Module) moveHand(beat float64) {
	m.ctx.Scene.PlayState(m.watchHand, "Click", beat, 0.4)
	m.handAngle += degreePerMonkey
	m.moveArrowTo(m.handAngle)
}

func (m *Module) playerClap(beat float64, big bool, barely bool) {
	if barely {
		m.ctx.Scene.PlayState(m.playerMonkey, "PlayerClapBarely", beat, 0.4)
		return
	}
	if big {
		m.ctx.Scene.PlayState(m.playerMonkey, "PlayerClapBig", beat, 0.4)
		m.flashClap(m.pinkClap, beat)
	} else {
		m.ctx.Scene.PlayState(m.playerMonkey, "PlayerClap", beat, 0.4)
		m.flashClap(m.yellowClap, beat)
	}
}

func (m *Module) flashClap(path string, beat float64) {
	if path == "" {
		return
	}
	m.ctx.Scene.SetActive(path, true)
	m.ctx.At(beat+0.4, func() { m.ctx.Scene.SetActive(path, false) })
}
