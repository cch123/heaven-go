package lovelab

import (
	"math"

	"hsdemo/engine"
)

func (m *Module) bopping(beat float64) {
	if m.bopRight {
		m.play(m.labGuy, "GuyBopRight", beat)
		m.play(m.labGirl, "GirlBopRight", beat)
		m.play(m.labAssistant, "AssistantBopRight", beat)
	} else {
		m.play(m.labGuy, "GuyBopLeft", beat)
		m.play(m.labGirl, "GirlBopLeft", beat)
		m.play(m.labAssistant, "AssistantBopLeft", beat)
	}
	m.bopRight = !m.bopRight
}

func (m *Module) bop(ev bopEvt) {
	if ev.bop {
		for i := 0; i < int(ev.length+1e-6); i++ {
			b := ev.beat + float64(i)
			m.ctx.At(b, func() { m.bopping(b) })
		}
	}
	m.canBop = ev.auto
}

func (m *Module) girlLook(beat, length float64) {
	delay := 0.5
	if length <= 0.5 {
		delay = 0
	}
	m.play(m.labGuyHead, "GuyRightFace", beat)
	m.ctx.At(beat+delay, func() {
		m.play(m.labGirlHead, "GirlLeftFace", beat+delay)
	})
}

func (m *Module) boyShake(sh shakeEvt, firstBeat, lastBeat, intervalLength float64) {
	beat, length := sh.beat, sh.length
	m.ctx.At(beat, func() {
		m.girlLook(beat, intervalLength)
		if !m.hasStartedInterval {
			m.play(m.labGuyArm, "GrabFlask", beat)
			m.hasStartedInterval = true
			count := len(m.guyHearts)
			if len(m.girlHearts) != 0 {
				count = 0
			}
			add := 2.5
			if length <= 0.5 && len(m.currentHearts) > 0 && m.currentHearts[len(m.currentHearts)-1] <= 1 {
				add = 1.5
			}
			if h := m.createHeart(0, beat, length, count, add, intervalLength); h != nil {
				m.guyHearts = append(m.guyHearts, h)
			}
		} else if beat > firstBeat {
			m.play(m.labGuyArm, "ShakeFlaskDown", beat)
		}
	})
	end := beat + length
	m.ctx.At(end, func() {
		if lastBeat > end+1e-6 {
			m.play(m.labGuyArm, "ShakeFlaskUp", end)
			if h := m.createHeart(0, end, length, len(m.guyHearts), 2.5, intervalLength); h != nil {
				m.guyHearts = append(m.guyHearts, h)
				m.heartUp(m.guyHearts)
			}
			return
		}
		m.play(m.labGuyArm, "ThrowFlask", end)
		m.spawnFlaskForGirl(end+1, sh.speed)
		m.stopHearts(m.guyHearts, end)
	})
}

func (m *Module) passToGirl(beat, intervalBeat, endBeat, length float64) {
	inputs := m.shakesBetween(intervalBeat, endBeat)
	if len(inputs) == 0 {
		return
	}
	speed := inputs[len(inputs)-1].speed
	addDelay := 1.0
	if speed == flaskFast {
		addDelay = 0
	} else if speed == flaskMidSlow {
		addDelay = 0.5
	}
	m.releaseValid = true
	for i, in := range inputs {
		input := in
		relativeBeat := input.beat - intervalBeat
		shakeStart := beat + length + relativeBeat + addDelay
		if i == 0 {
			m.ctx.ScheduleInputActionCond(shakeStart, 0, m.canHitNow, func(float64, engine.Judgment) {
				m.onCatch(shakeStart)
			}, func() {
				m.onCatchMiss(shakeStart)
			})
		} else {
			m.ctx.ScheduleInputActionCond(shakeStart, actionShake, m.canHitNow, func(float64, engine.Judgment) {
				m.onDownAuto(shakeStart)
			}, func() {
				m.onMiss(shakeStart)
			})
		}
		shakeEnd := beat + length + (input.beat + input.length - intervalBeat) + addDelay
		if input.beat+input.length >= endBeat-1e-6 {
			m.ctx.ScheduleInputReleaseCond(shakeEnd, func() bool {
				return m.releaseValid && m.canHitNow()
			}, func(float64, engine.Judgment) {
				m.onRelease(shakeEnd)
			}, func() {
				m.onMiss(shakeEnd)
			})
		} else {
			m.ctx.ScheduleInputActionCond(shakeEnd, actionUp, m.canHitNow, func(float64, engine.Judgment) {
				m.onUp(shakeEnd)
			}, func() {
				m.onMiss(shakeEnd)
			})
		}
	}
	m.ctx.At(beat, func() { m.hasMissed = false })
	m.ctx.At(endBeat+1, func() {
		m.play(m.labGuyHead, "GuyFaceIdle", endBeat+1)
		m.play(m.labGuyArm, "ArmIdle", endBeat+1)
	})
}

func (m *Module) onCatch(beat float64) {
	m.play(m.labGirlHead, "GirlIdleFace", beat)
	if len(m.guyHearts) == 0 {
		return
	}
	add := 2.5
	if m.guyHearts[0].length <= 0.5 && len(m.currentHearts) > 0 && m.currentHearts[len(m.currentHearts)-1] <= 1 {
		add = 1.5
	}
	if !m.isHolding {
		m.isHolding = true
		m.isHoldingFlask = true
		if h := m.createHeart(1, beat, m.guyHearts[0].length, len(m.girlHearts), add, m.guyHearts[0].intervalSpeed); h != nil {
			m.girlHearts = append(m.girlHearts, h)
		}
		m.ctx.Sound("rightCatch")
		m.play(m.labGirlArm, "GrabFlask", beat)
		m.destroyFirstGirlFlask()
	}
}

func (m *Module) onUp(beat float64) {
	if len(m.guyHearts) == 0 {
		return
	}
	idx := len(m.girlHearts)
	length := m.guyHearts[int(math.Min(float64(idx), float64(len(m.guyHearts)-1)))].length
	if h := m.createHeart(1, beat, length, idx, 2.5, m.guyHearts[0].intervalSpeed); h != nil {
		m.girlHearts = append(m.girlHearts, h)
		m.heartUp(m.girlHearts)
	}
	m.onWhiffUp(beat)
}

func (m *Module) onDownAuto(beat float64) {
	m.hasShakenUp = false
	m.ctx.Sound("shakeDown")
	m.play(m.labGirlArm, "ShakeFlaskDown", beat)
}

func (m *Module) onRelease(beat float64) {
	m.hasShakenUp = false
	m.play(m.labGirlHead, "GirlIdleFace", beat)
	m.ctx.Sound("rightThrowNoShake")
	m.play(m.labGirlArm, "ThrowFlask", beat)
	m.isHolding = false
	m.isHoldingFlask = false
	m.hasStartedInterval = false
	m.releaseValid = false
	m.stopHearts(m.girlHearts, beat)
	m.spawnFlaskForWeird(beat)
	lastCounter := 0
	if len(m.currentHearts) > 0 {
		lastCounter = m.currentHearts[0]
	}
	m.ctx.At(beat+1, func() {
		m.labGirlIdle(beat + 1)
		m.ctx.Sound("heartsCombine")
		m.stopHearts(m.girlHearts, beat+1)
		n := lastCounter
		for i := 0; i < n && len(m.guyHearts) > 0 && len(m.girlHearts) > 0; i++ {
			g, gl := m.guyHearts[0], m.girlHearts[0]
			g.inst.PlayState("Heart/HeartHolder", "HeartMerge", beat+1, 0.5)
			gl.inst.PlayState("Heart/HeartHolder", "HeartGirlMerge", beat+1, 0.5)
			_, endY, ok := m.nodeWorld(m.endPoint)
			if !ok {
				endY = 1.6
			}
			if h := m.createHeart(2, beat+1, 1, 0, 0, 1); h != nil {
				h.inst.Offset[1] = gl.y
				h.y = gl.y
				h.dropStartY = gl.y
				h.dropEndY = endY
				h.dropLength = (2.25 + 0.063*float64(len(m.girlHearts))) / 2
				h.waiting = false
				m.completeHearts = append(m.completeHearts, h)
			}
			m.guyHearts = m.guyHearts[1:]
			m.girlHearts = m.girlHearts[1:]
		}
		if len(m.currentHearts) > 0 {
			m.currentHearts = m.currentHearts[1:]
		}
	})
	for x := 0; x < lastCounter; x++ {
		i := x
		t := beat + 2.25 + 0.063*float64(i)
		m.ctx.At(t, func() {
			if lastCounter == 1 {
				m.ctx.Sound("bagHeartLast")
			} else {
				m.ctx.SoundPitch("bagHeart", 1, 1+0.14*float64(i))
			}
			m.play(m.heartBox, "HeartBoxSquish", t)
		})
	}
}

func (m *Module) onMiss(beat float64) {
	if m.hasMissed {
		return
	}
	m.play(m.labGuyHead, "GuyFaceIdle", beat)
	m.hasShakenUp = false
	m.hasMissed = true
	m.hasStartedInterval = false
	m.releaseValid = false
	n := 0
	if len(m.currentHearts) > 0 {
		n = m.currentHearts[0]
		m.currentHearts = m.currentHearts[1:]
	}
	if n <= 0 {
		n = len(m.guyHearts)
	}
	m.markDeadHearts(m.guyHearts[:minInt(n, len(m.guyHearts))], beat)
	m.markDeadHearts(m.girlHearts[:minInt(n, len(m.girlHearts))], beat)
	m.guyHearts = m.guyHearts[minInt(n, len(m.guyHearts)):]
	m.girlHearts = m.girlHearts[minInt(n, len(m.girlHearts)):]
}

func (m *Module) onCatchMiss(beat float64) {
	if !m.isHoldingFlask {
		m.releaseValid = false
	}
	m.flaskBreak(0, beat)
	m.destroyFirstGirlFlask()
	m.onMiss(beat)
}

func (m *Module) onWhiffUp(beat float64) {
	if m.hasShakenUp {
		return
	}
	m.hasShakenUp = true
	if m.isHoldingFlask {
		m.ctx.Sound("shakeUp")
		m.play(m.labGirlArm, "ShakeFlaskUp", beat)
		return
	}
	m.ctx.Scene.PlayState(m.labGirlArm, "WhiffUp", beat, 0.75)
}

func (m *Module) onWhiffDown(beat float64) {
	if !m.hasShakenUp {
		return
	}
	m.hasShakenUp = false
	if m.isHoldingFlask {
		m.ctx.Sound("shakeDown")
		m.play(m.labGirlArm, "ShakeFlaskDown", beat)
		return
	}
	m.ctx.Scene.PlayState(m.labGirlArm, "WhiffDown", beat, 0.75)
}

func (m *Module) labGirlIdle(beat float64) {
	m.play(m.labGirlHead, "GirlIdleFace", beat)
	m.play(m.labGirlArm, "ArmIdle", beat)
}

func (m *Module) flaskBreak(which int, beat float64) {
	x, y := -5.1, -0.5
	if which != 0 {
		x, y = -1, -3.45
	}
	m.spawnHeartBurst(x, y, beat)
	m.ctx.PlayCommon("miss")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
