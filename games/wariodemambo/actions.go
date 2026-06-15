package wariodemambo

import "hsdemo/engine"

func (m *Module) schedulePlayerInput(beat float64, action int, hit func(float64, engine.Judgment), miss func()) {
	m.appendExpectedInput(beat)
	m.ctx.ScheduleInputActionCond(beat, action, m.canHitNow, hit, miss)
}

func (m *Module) girlsInput(beat float64, jump, left bool) {
	switch {
	case jump:
		m.dancerJump(beat)
	case left:
		m.dancerLeftTurn(beat)
	default:
		m.dancerRightTurn(beat)
	}
}

func (m *Module) dancerLeftTurn(beat float64) {
	if m.isDancing {
		return
	}
	m.ctx.At(beat, func() {
		m.dancerLeft = true
		if m.dancerArmCentered {
			m.dancerArmCentered = false
			m.play(m.dancerLArm, "DarmCtoL", beat)
			m.play(m.dancerRArm, "DarmCtoL", beat)
			m.play(m.dancerLHead, "DHeadCtoL", beat)
			m.play(m.dancerRHead, "DHeadCtoL", beat)
			return
		}
		m.play(m.dancerLArm, "DArmRtoL", beat)
		m.play(m.dancerRArm, "DArmRtoL", beat)
		m.play(m.dancerLHead, "DHeadRtoL", beat)
		m.play(m.dancerRHead, "DHeadRtoL", beat)
	})
}

func (m *Module) dancerRightTurn(beat float64) {
	if m.isDancing {
		return
	}
	m.ctx.At(beat, func() {
		m.dancerLeft = false
		if m.dancerArmCentered {
			m.dancerArmCentered = false
			m.play(m.dancerLArm, "DArmCtoR", beat)
			m.play(m.dancerRArm, "DArmCtoR", beat)
			m.play(m.dancerLHead, "DHeadCtoR", beat)
			m.play(m.dancerRHead, "DHeadCtoR", beat)
			return
		}
		m.play(m.dancerLArm, "DArmLtoR", beat)
		m.play(m.dancerRArm, "DArmLtoR", beat)
		m.play(m.dancerLHead, "DHeadLtoR", beat)
		m.play(m.dancerRHead, "DHeadLtoR", beat)
	})
}

func (m *Module) dancerCenter(beat float64) {
	if m.isDancing {
		return
	}
	m.ctx.At(beat, func() {
		if m.dancerArmCentered {
			return
		}
		m.dancerArmCentered = true
		arm, head := "DArmRtoC", "DHeadRtoC"
		if m.dancerLeft {
			arm, head = "DArmLtoC", "DHeadLtoC"
		}
		m.play(m.dancerLArm, arm, beat)
		m.play(m.dancerRArm, arm, beat)
		m.play(m.dancerLHead, head, beat)
		m.play(m.dancerRHead, head, beat)
	})
}

func (m *Module) dancerJump(beat float64) {
	m.ctx.At(beat, func() {
		m.play(m.dancerLJump, "DJump", beat)
		m.play(m.dancerRJump, "DJump", beat)
	})
}

func (m *Module) justJump(_ float64, _ engine.Judgment) {
	beat := m.currentBeat()
	m.hasFlicked = true
	m.ctx.Sound("jump")
	m.play(m.warioJump, "WJump", beat)
}

func (m *Module) justTurnLeft(_ float64, _ engine.Judgment) {
	m.turnWario(m.currentBeat(), true, false)
}

func (m *Module) justTurnRight(_ float64, _ engine.Judgment) {
	m.turnWario(m.currentBeat(), false, false)
}

func (m *Module) miss() {
	m.misses++
	m.ctx.PlayCommon("miss")
}

func (m *Module) turnWario(beat float64, left bool, whiff bool) {
	if !m.armControlsEnabled {
		return
	}
	if left {
		m.ctx.Sound("left")
		m.warioLeft = true
		if m.armCentered {
			m.armCentered = false
			m.play(m.warioArm, "WHandCtoL", beat)
			m.play(m.warioFace, "WFaceCtoL", beat)
		} else {
			m.play(m.warioArm, "WHandRtoL", beat)
			m.play(m.warioFace, "WFaceRtoL", beat)
		}
	} else {
		m.ctx.Sound("right")
		m.warioLeft = false
		if m.armCentered {
			m.armCentered = false
			m.play(m.warioArm, "WHandCtoR", beat)
			m.play(m.warioFace, "WFaceCtoR", beat)
		} else {
			m.play(m.warioArm, "WHandLtoR", beat)
			m.play(m.warioFace, "WFaceLtoR", beat)
		}
	}
	if whiff {
		m.ctx.PlayCommon("miss")
	}
}

func (m *Module) whiffJump(beat float64) {
	if cur, playing := m.ctx.Scene.StateInfo(m.warioJump, beat); cur == "WJump" && playing {
		return
	}
	m.hasFlicked = true
	m.ctx.ScoreMiss()
	m.misses++
	m.play(m.warioJump, "WJump", beat)
	m.ctx.Sound("jump")
	m.ctx.PlayCommon("miss")
}

func (m *Module) whiffRight(beat float64) {
	if !(m.warioLeft || m.armCentered) {
		return
	}
	m.hasFlicked = false
	m.ctx.ScoreMiss()
	m.misses++
	m.turnWario(beat, false, true)
}

func (m *Module) whiffLeft(beat float64) {
	if !(!m.warioLeft || m.armCentered) {
		return
	}
	m.hasFlicked = false
	m.ctx.ScoreMiss()
	m.misses++
	m.turnWario(beat, true, true)
}

func (m *Module) resetArms(enabled bool) {
	m.armControlsEnabled = enabled
	if enabled {
		return
	}
	beat := m.currentBeat()
	m.play(m.warioArm, "WHandIdle", beat)
	m.play(m.warioFace, "WFaceIdle", beat)
}
