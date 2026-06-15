package wariodemambo

import "math"

func (m *Module) scheduleEvents() {
	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() { m.toggleBop(ev) })
	}
	for _, ev := range m.intervals {
		ev := ev
		m.ctx.At(ev.beat-4, func() { m.preInterval(ev, ev.beat) })
	}
	for _, ev := range m.passes {
		ev := ev
		m.ctx.At(ev.beat-1, func() { m.prePassTurn(ev) })
	}
	for _, ev := range m.texts {
		ev := ev
		m.setText(ev.beat, ev.length, ev.text)
	}
	for _, ev := range m.reactions {
		ev := ev
		m.ctx.At(ev.beat, func() { m.intervalReaction(ev.beat, ev.length, ev.resetColor) })
	}
	for _, ev := range m.lights {
		ev := ev
		m.ctx.At(ev.beat, func() { m.changeLights(ev.beat, ev.stage) })
	}
	for _, ev := range m.dances {
		ev := ev
		m.ctx.At(ev.beat, func() { m.dance(ev.beat, ev.length, ev.typ) })
	}
	for _, ev := range m.colors {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setColors(ev.red, ev.dim) })
	}
}

func (m *Module) toggleBop(ev bopEvt) {
	m.autoBop = ev.auto
	if ev.auto {
		m.bop(ev.beat, true, true, true)
		return
	}
	for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
		bb := b
		m.ctx.At(bb, func() { m.bop(bb, ev.mambo, ev.dancers, ev.lights) })
	}
}

func (m *Module) preInterval(ev intervalEvt, gameSwitchBeat float64) {
	if ev.memorize {
		m.ctx.SoundAt(ev.beat-2, "memorize", 1)
	}
	m.shouldBeLeft = ev.left
	if !m.activeAt(m.ctx.Beat()) || m.startedIntervals[ev.idx] {
		return
	}
	m.setIntervalStart(ev, gameSwitchBeat)
}

func (m *Module) setIntervalStart(ev intervalEvt, gameSwitchBeat float64) {
	m.startedIntervals[ev.idx] = true
	m.bopState = bopThink
	m.play(m.warioBody, "WahThink", ev.beat)
	m.dancerCenter(ev.beat - 4)

	m.crStart = ev.beat
	m.crEvents = nil
	for _, in := range m.inputsBetween(ev.beat, ev.beat+ev.length) {
		if in.beat < gameSwitchBeat-1e-6 {
			continue
		}
		if in.jump {
			m.jumpInputController(in.beat)
		} else {
			m.turnInputController(in.beat)
		}
	}

	if ev.numbers && ev.autoPass {
		m.countDownFor(ev.beat+ev.length+ev.length, ev.length)
	}
	if ev.text && ev.autoPass {
		m.countTextFor(ev.beat+ev.length+ev.length, ev.length, true)
	}

	m.ctx.At(ev.beat-4, func() {
		m.dancerBopState = bopThink
		m.resetArms(true)
	})
	m.ctx.At(ev.beat-3, func() {
		m.startSpotEase(ev.beat-3, 2.5, easeOutQuint, m.point(m.spotLTarget), m.point(m.spotRTarget))
		m.spotsPos = spotsDancers
	})
	m.ctx.At(ev.beat-2.5, func() { m.dancerBopState = bopReady })
	m.ctx.At(ev.beat-1.5, func() { m.dancerBopState = bopNormal })
	m.ctx.At(ev.beat, func() { m.setColors(false, false) })
	m.ctx.At(ev.beat+ev.length+1, func() {
		if m.spotsPos == spotsDancers {
			m.moveSpotlights(ev.beat+ev.length+1, 1.5, true)
			m.spotsPos = spotsRandom
		}
	})
	m.ctx.At(ev.beat+(ev.length*3)-0.5, func() {
		if ev.autoPass {
			m.intervalReaction(ev.beat+(ev.length*3)-0.5, ev.length*0.375, ev.resetColor)
		}
	})
	if ev.autoPass {
		m.passTurn(ev.beat+ev.length, ev.length)
	}
	if ev.text {
		m.setText(ev.beat-2, 0, "Memorize!")
	}
}

func (m *Module) prePassTurn(ev passEvt) {
	if ev.numbers {
		m.countDownFor(ev.beat+ev.length, ev.length)
	}
	if ev.text {
		m.countTextFor(ev.beat+ev.length, ev.length, false)
	}
	if m.activeAt(m.ctx.Beat()) {
		m.passTurnStandalone(ev)
		return
	}
	m.pendingPasses = append(m.pendingPasses, ev)
}

func (m *Module) passTurnStandalone(ev passEvt) {
	if len(m.crEvents) == 0 {
		if iv, ok := m.lastIntervalBefore(ev.beat); ok && !m.startedIntervals[iv.idx] {
			m.setIntervalStart(iv, ev.beat)
		}
	}
	if len(m.crEvents) == 0 {
		return
	}
	m.passTurn(ev.beat, ev.length)
}

func (m *Module) passTurn(beat, length float64) {
	m.ctx.At(beat, func() {
		m.setColors(true, true)
		if length > 4 {
			m.setText(beat, 0, "Your turn!")
		}
	})
	m.ctx.At(beat+maxFloat(length-4, 0), func() { m.dancerCenter(beat + maxFloat(length-4, 0)) })
	m.ctx.At(beat+maxFloat(length-2.5, 0), func() {
		wario := m.point(m.spotWario)
		m.startSpotEase(beat+maxFloat(length-2.5, 0), 2.5, easeOutQuint, wario, wario)
		m.spotsPos = spotsWario
		m.bopState = bopNormal
	})
	m.ctx.At(beat-0.25+length, func() {
		for _, cr := range m.crEvents {
			target := beat + length + cr.relative
			if target < m.ctx.Beat()-1e-6 {
				continue
			}
			switch cr.tag {
			case "turnLeft":
				m.schedulePlayerInput(target, actionLeft, m.justTurnLeft, m.miss)
				m.dancerLeftTurn(target)
			case "turnRight":
				m.schedulePlayerInput(target, actionRight, m.justTurnRight, m.miss)
				m.dancerRightTurn(target)
			case "jump":
				m.schedulePlayerInput(target, actionJump, m.justJump, m.miss)
				m.dancerJump(target)
			}
		}
		m.crEvents = nil
	})
	m.ctx.At(beat+length, func() { m.setColors(true, false) })
}

func (m *Module) turnInputController(beat float64) {
	if m.crHasEventAt(beat) {
		return
	}
	if m.shouldBeLeft {
		m.ctx.SoundAt(beat, "left", 1)
		m.girlsInput(beat, false, true)
		m.shouldBeLeft = false
		m.crEvents = append(m.crEvents, crEvent{beat: beat, relative: beat - m.crStart, tag: "turnLeft"})
		return
	}
	m.ctx.SoundAt(beat, "right", 1)
	m.girlsInput(beat, false, false)
	m.shouldBeLeft = true
	m.crEvents = append(m.crEvents, crEvent{beat: beat, relative: beat - m.crStart, tag: "turnRight"})
}

func (m *Module) jumpInputController(beat float64) {
	if m.crHasEventAt(beat) {
		return
	}
	m.crEvents = append(m.crEvents, crEvent{beat: beat, relative: beat - m.crStart, tag: "jump"})
	m.ctx.SoundAt(beat, "jump", 1)
	m.girlsInput(beat, true, false)
}

func (m *Module) countDownFor(endBeat, length float64) {
	switch {
	case length >= 4:
		m.countDownFour(endBeat - 4)
	case length >= 3:
		m.countDownThree(endBeat - 3)
	case length >= 2:
		m.countDownTwo(endBeat - 2)
	case length >= 1:
		m.countDownOne(endBeat - 1)
	}
}

func (m *Module) countTextFor(endBeat, length float64, finalOneHolds bool) {
	if length >= 4 {
		m.setText(endBeat-4, 0, "4")
	}
	if length >= 3 {
		m.setText(endBeat-3, 0, "3")
	}
	if length >= 2 {
		m.setText(endBeat-2, 0, "2")
	}
	if length >= 1 {
		hold := 0.0
		if finalOneHolds {
			hold = 1
		}
		m.setText(endBeat-1, hold, "1")
	}
}

func (m *Module) countDownFour(beat float64) {
	m.ctx.SoundAt(beat, "four", 1)
	m.ctx.SoundAtOff(beat+1, "three", 1, 0.05)
	m.ctx.SoundAtOff(beat+2, "two", 1, 0.04)
	m.ctx.SoundAtOff(beat+3, "one", 1, 0.07)
}

func (m *Module) countDownThree(beat float64) {
	m.ctx.SoundAtOff(beat, "three", 1, 0.05)
	m.ctx.SoundAtOff(beat+1, "two", 1, 0.04)
	m.ctx.SoundAtOff(beat+2, "one", 1, 0.07)
}

func (m *Module) countDownTwo(beat float64) {
	m.ctx.SoundAtOff(beat, "two", 1, 0.04)
	m.ctx.SoundAtOff(beat+1, "one", 1, 0.07)
}

func (m *Module) countDownOne(beat float64) {
	m.ctx.SoundAtOff(beat, "one", 1, 0.07)
}

func (m *Module) intervalReaction(beat, length float64, resetColor bool) {
	if m.misses > 0 {
		m.play(m.warioBody, "WahBad", beat)
		m.bopState = bopFail
		m.dancerBopState = bopFail
		m.setText(beat, length, "No!")
	} else {
		m.ctx.Sound("applause")
		m.play(m.warioBody, "WahBopHap", beat)
		m.bopState = bopHappy
		m.dancerBopState = bopHappy
	}
	m.ctx.At(beat+length-1, func() {
		if !m.armCentered {
			if m.warioLeft {
				m.play(m.warioArm, "WHandLtoC", beat+length-1)
				m.play(m.warioFace, "WFaceLtoC", beat+length-1)
			} else {
				m.play(m.warioArm, "WHandRtoC", beat+length-1)
				m.play(m.warioFace, "WFaceRtoC", beat+length-1)
			}
		}
		m.armCentered = true
		if resetColor {
			m.setColors(false, true)
		}
	})
	m.ctx.At(beat+length, func() {
		m.bopState = bopNormal
		if resetColor && m.spotsPos == spotsWario {
			m.moveSpotlights(beat+length, 1.5, true)
			m.spotsPos = spotsRandom
		}
	})
	m.ctx.At(beat+length+1, func() { m.dancerCenter(beat + length + 1) })
	m.misses = 0
}

func (m *Module) dance(beat, length float64, typ int) {
	m.canBop = false
	m.isDancing = true
	m.armCentered = true
	m.resetArms(false)
	for _, p := range []string{m.warioBody, m.dancerL, m.dancerR} {
		state := "WDanceLoop"
		if p != m.warioBody {
			state = "DancerFinalLoop"
		}
		m.play(p, state, beat)
	}
	for _, p := range []string{m.dancerLArm, m.dancerRArm} {
		m.play(p, "DArmDanceLoop", beat)
	}
	for _, p := range []string{m.dancerLHead, m.dancerRHead} {
		m.play(p, "DHeadDanceLoop", beat)
	}

	pose := 0
	switch typ {
	case dancePose1:
		m.danceEase = danceEase{beat: beat, length: length, clip: "WWalkCRC", active: true}
		pose = 1
	case dancePose2:
		m.danceEase = danceEase{beat: beat, length: length, clip: "WWalkCLC", active: true}
		pose = 2
	case danceEnd:
		m.danceEase = danceEase{beat: beat, length: length, clip: "WWalkFinal", active: true}
		pose = 3
	}
	if pose != 0 {
		m.ctx.At(beat+length, func() {
			if pose == 3 {
				m.ctx.Scene.SetActive(m.endPose, true)
				return
			}
			m.play(m.warioBody, "WahPose"+smallInt(pose), beat+length)
			m.play(m.dancerL, "DancerPose"+smallInt(pose), beat+length)
			m.play(m.dancerR, "DancerPose"+smallInt(pose), beat+length)
			m.play(m.dancerLArm, "DArmPose"+smallInt(pose), beat+length)
			m.play(m.dancerRArm, "DArmPose"+smallInt(pose), beat+length)
			m.play(m.dancerLHead, "DHeadPose"+smallInt(pose), beat+length)
			m.play(m.dancerRHead, "DHeadPose"+smallInt(pose), beat+length)
		})
	}
	m.ctx.At(beat+length+0.25, func() {
		m.canBop = true
		m.isDancing = false
	})
}

func (m *Module) changeLights(beat float64, stage int) {
	m.lightsStage = stage
	m.playLights(beat)
}

func (m *Module) setColors(red, dim bool) {
	m.gameRed = red
	m.gameDim = dim
}

func (m *Module) setText(beat, length float64, command string) {
	m.ctx.At(beat, func() {
		m.currentText = command
		_ = m.ctx.Assets.SetText(m.commandText, command)
		m.play(m.textAnim, "Text", beat)
	})
	m.ctx.At(beat+length, func() {
		if length > 0 && m.currentText == command {
			m.currentText = ""
			_ = m.ctx.Assets.SetText(m.commandText, "")
		}
	})
}

func smallInt(v int) string {
	return string(rune('0' + v))
}

func finite(v float64) bool {
	return !math.IsInf(v, 0) && !math.IsNaN(v)
}
