package animalacrobat

import (
	"image/color"

	"hsdemo/engine"
)

func (m *Module) scheduleObstacle(ob *acrobatObstacle) {
	holdBeat := ob.beat - 1
	releaseBeat := ob.beat + ob.input.holdLength
	m.ctx.SoundAt(releaseBeat-2, "eek", 1)
	m.ctx.SoundAt(releaseBeat-1, "eek", 1)
	m.scheduleEarFlaps(ob, releaseBeat)
	obPtr := ob
	m.ctx.ScheduleInput(holdBeat, func(state float64, j engine.Judgment) {
		m.justHold(obPtr, holdBeat, j)
	}, func() {
		m.missObstacle(obPtr, holdBeat)
	})
	releaseInput := m.ctx.ScheduleInputRelease(releaseBeat, func(state float64, j engine.Judgment) {
		m.justRelease(obPtr, releaseBeat, j)
	}, func() {
		m.missObstacle(obPtr, releaseBeat)
	})
	prevCanHit := releaseInput.CanHit
	releaseInput.CanHit = func() bool {
		if prevCanHit != nil && !prevCanHit() {
			return false
		}
		return obPtr.canHit
	}
	if ob.kind == kindGiraffe {
		m.ctx.SoundAt(ob.beat+2, "giraffeCymbal", 1)
		m.ctx.SoundAt(ob.beat+2.25, "applause", 1)
	}
}

func (m *Module) scheduleEarFlaps(ob *acrobatObstacle, releaseBeat float64) {
	switch ob.kind {
	case kindElephant:
		for _, b := range []float64{releaseBeat - 2, releaseBeat - 1} {
			b := b
			m.ctx.At(b, func() { ob.inst.PlayState("", "ElephantEar", b, animScale) })
		}
	case kindGiraffe:
		for _, b := range []float64{releaseBeat - 2, releaseBeat - 1} {
			b := b
			m.ctx.At(b, func() { ob.inst.PlayState("GiraffeRoot", "GiraffeEar", b, animScale) })
		}
	}
}

func (m *Module) justHold(ob *acrobatObstacle, beat float64, j engine.Judgment) {
	if m.monkeyMissed && beat-m.lastMissBeat < 3 {
		m.missObstacle(ob, beat)
		return
	}
	ob.held = true
	m.holding = ob
	m.monkeyMissed = false
	m.stopTrail(m.ctx.Time())
	m.ctx.Scene.SetActive(m.player, false)
	m.forcePlayerShadowOff()
	ob.inst.ResetSubtree(ob.input.monkeyRel)
	setInstActive(ob.inst, ob.input.monkeyRel, true)
	setInstActive(ob.inst, ob.input.endShadowRel, true)
	ob.inst.PlayState(ob.input.monkeyRel, "PlayerHang", beat, animScale)
	m.ctx.At(beat+ob.input.holdLength/2, func() {
		if ob.held {
			ob.inst.PlayState(ob.input.monkeyRel, "PlayerHanging", beat+ob.input.holdLength/2, animScale)
		}
	})
	if ob.kind == kindGiraffe {
		m.ctx.Sound("giraffeCatch")
	} else {
		m.ctx.Sound("catch")
	}
	if j == engine.JudgeNG {
		m.ctx.Sound("common_nearMiss")
		m.spawnSweatParticle(ob, beat)
	} else {
		m.spawnHoldParticle(ob, beat)
	}
}

func (m *Module) justRelease(ob *acrobatObstacle, releaseBeat float64, j engine.Judgment) {
	if !ob.held {
		m.missObstacle(ob, releaseBeat)
		return
	}
	ob.held = false
	ob.done = true
	ob.canHit = false
	if m.holding == ob {
		m.holding = nil
	}
	setInstActive(ob.inst, ob.input.monkeyRel, false)
	setInstActive(ob.inst, ob.input.gripShadowRel, false)
	setInstActive(ob.inst, ob.input.endShadowRel, false)
	m.ctx.Scene.SetActive(m.player, true)
	m.forcePlayerShadowOn()
	if ob.kind == kindGiraffe {
		m.ctx.Sound("giraffeJump")
		if j == engine.JudgeNG {
			m.muteRelease = true
		}
		if m.drumrollStop != nil {
			m.drumrollStop()
		}
		m.drumrollStop = m.ctx.SoundLoopVol("giraffeDrumroll", 0.55)
		m.ctx.At(releaseBeat+4, func() {
			if m.drumrollStop != nil {
				m.drumrollStop()
				m.drumrollStop = nil
			}
		})
		ob.inst.PlayState("FireHoop", "FireClose", releaseBeat+2, animScale)
	} else if j == engine.JudgeNG {
		m.ctx.Sound("common_nearMiss")
		m.muteRelease = true
	} else {
		m.ctx.Sound("release")
	}
	if !ob.end {
		m.ctx.SoundAt(releaseBeat+1, "turn", 1)
	}
	m.startJumpFromObstacle(releaseBeat, ob)
}

func (m *Module) missObstacle(ob *acrobatObstacle, beat float64) {
	if ob.done {
		return
	}
	ob.done = true
	ob.canHit = false
	ob.held = false
	m.monkeyMissed = true
	m.lastMissBeat = beat
	if m.holding == ob {
		m.holding = nil
		m.ctx.Scene.SetActive(m.player, true)
		m.forcePlayerShadowOn()
		setInstActive(ob.inst, ob.input.gripShadowRel, false)
		setInstActive(ob.inst, ob.input.endShadowRel, false)
	}
	m.ctx.Sound("miss")
	m.playPlayer("PlayerAir", beat)
	m.emitObstacleSparkle(ob, beat, color.NRGBA{255, 90, 110, 220})
}

func (m *Module) updateObstacleRotation(beat float64) {
	for _, ob := range m.animals {
		if ob.kind == kindGorilla {
			continue
		}
		rot := obstacleAngle(ob.spec, beat, ob.beat)
		ob.inst.SetRot(ob.spec.rotRel, rot)
		if ob.kind == kindMonkeysLong {
			u := clamp01((beat - ob.beat) / 2.5)
			ob.inst.PlayNormalized("WhiteMonkeysPivot", "WhiteMonkeysSwing", u)
		}
	}
}

func (m *Module) updateEarlyRelease(beat float64) {
	if m.holding == nil || m.ctx.App.Autoplay {
		return
	}
	ob := m.holding
	releaseBeat := ob.beat + ob.input.holdLength
	// The real game fails immediately if the player lets go before the flick
	// release action becomes hittable; once inside the release window the
	// engine's normal release judgment owns the timing result.
	if beat < releaseBeat-0.28 && m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
		ob.canHit = false
		m.ctx.ScoreMiss()
		m.missObstacle(ob, beat)
	}
}

func (m *Module) emitObstacleSparkle(ob *acrobatObstacle, beat float64, col color.NRGBA) {
	m.emitSparkle(beat, ob.x+ob.gripX, ob.gripY, col)
}
