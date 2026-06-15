package sumobrothers

import "hsdemo/engine"

func (m *Module) onLateBeatPulse(beat float64) {
	if m.allowBopInu && m.goBopInu {
		m.play(m.inu, "InuBop", beat)
	}
	if m.allowBopSumo && m.goBopSumo {
		m.brosBop(beat)
	}
}

func (m *Module) bop(beat, length float64, inu, sumo, inuAuto, sumoAuto bool) {
	m.goBopInu = inuAuto
	m.goBopSumo = sumoAuto
	for i := 0; i < int(length); i++ {
		b := beat + float64(i)
		m.ctx.At(b, func() {
			if inu {
				m.play(m.inu, "InuBop", b)
			}
			if sumo {
				m.brosBop(b)
			}
		})
	}
}

func (m *Module) brosBop(beat float64) {
	switch m.previousState {
	case stateIdle:
		m.play(m.pBody, "SumoBop", beat)
		m.play(m.gBody, "SumoBop", beat)
		m.play(m.pHead, "SumoPIdle", beat)
		m.play(m.gHead, "SumoGIdle", beat)
	case statePose:
		m.play(m.pBody, "SumoPosePBop"+m.sumoPoseCurrent, beat)
		m.play(m.gBody, "SumoPoseGBop"+m.sumoPoseCurrent, beat)
	}
}

func (m *Module) crouch(beat, length float64, inu, sumo bool) {
	if m.previousState != stateIdle {
		return
	}
	if inu {
		m.allowBopInu = false
		m.play(m.inu, "InuCrouch", beat)
	}
	if sumo {
		m.allowBopSumo = false
		m.play(m.pBody, "SumoCrouch", beat)
		m.play(m.gBody, "SumoCrouch", beat)
	}
	end := beat + length
	m.ctx.At(end, func() {
		if m.previousState != stateIdle {
			return
		}
		m.allowBopInu = true
		m.allowBopSumo = true
		if m.goBopSumo {
			m.brosBop(end)
		}
		if m.goBopInu {
			m.play(m.inu, "InuBop", end)
		}
	})
}

func (m *Module) stompSignal(beat float64, mute, inu bool, lookAtCam bool, startingDirection int) {
	if m.sumoState == stateStomp || m.cueActive {
		return
	}
	m.nextSwitchBeat = m.ctx.NextSwitchBeat(beat)
	m.cueRunning(beat + 3)
	if lookAtCam && m.sumoState == stateSlap {
		m.lookingAtCamera = true
		m.play(m.pHead, "SumoPSlapLook", beat)
		m.play(m.gHead, "SumoGSlapLook", beat)
	}
	m.ctx.At(beat+3, func() { m.allowBopSumo = false })
	if inu {
		m.ctx.At(beat, func() {
			m.allowBopInu = false
			m.play(m.inu, "InuBeatChange", beat)
		})
		m.ctx.At(beat+2, func() { m.play(m.inu, "InuBeatChange", beat+2) })
	}
	if !mute {
		m.stompSignalSound(beat)
	}
	m.previousState = m.sumoState
	m.sumoState = stateStomp

	stompType := 1
	startingLeftAfterTransition := false
	prepareAnimation := true
	if startingDirection == stompLeft {
		startingLeftAfterTransition = true
	}
	if startingDirection == stompRight {
		stompType = 2
	}
	switch m.previousState {
	case stateSlap:
		stompType = 3
		prepareAnimation = false
	case statePose:
		stompType = 4
		prepareAnimation = false
	}
	m.stompRecursive(beat+3, 1, stompType, startingLeftAfterTransition, false, prepareAnimation)
}

func (m *Module) stompSignalSound(beat float64) {
	m.soundAt(beat, "stompSignal")
	m.soundAt(beat+2, "stompSignal")
}

func (m *Module) stompRecursive(beat, remaining float64, typ int, startingLeftAfterTransition, autoDecreaseRemaining, prepareAnimation bool) {
	if m.sumoState != stateStomp || autoDecreaseRemaining {
		remaining--
	}
	if beat >= m.nextSwitchBeat-1 {
		remaining = 0
	}
	if remaining <= 0 {
		return
	}

	switch typ {
	case 3:
		if prepareAnimation {
			m.ctx.At(beat, func() {
				m.play(m.pBody, "SumoStompPrepareL", beat)
				m.play(m.gBody, "SumoStompPrepareL", beat)
				m.play(m.pHead, "SumoPStomp", beat)
				m.play(m.gHead, "SumoGStomp", beat)
			})
		}
		m.ctx.At(beat, func() { m.sumoStompDir = true })
		m.ctx.At(beat+1, func() {
			m.previousState = stateStomp
			m.lookingAtCamera = false
			m.play(m.gBody, "SumoStompL", beat+1)
			m.play(m.gHead, "SumoGStomp", beat+1)
			if m.sumoState == stateStomp && !m.isPlaying(m.inu, "InuFloatMiss") {
				m.play(m.inu, "InuFloat", beat+1)
			}
		})
	case 4:
		m.ctx.At(beat, func() {
			m.sumoStompDir = true
			m.play(m.pBody, "SumoPoseSwitch", beat)
			m.play(m.gBody, "SumoPoseSwitch", beat)
			m.play(m.pHead, "SumoPStomp", beat)
			m.play(m.gHead, "SumoGStomp", beat)
			m.play(m.bgStatic, "empty", beat)
			m.play(m.glasses, "glassesGone", beat)
			m.bgType = bgNone
		})
		m.ctx.At(beat+1, func() {
			m.previousState = stateStomp
			m.play(m.gBody, "SumoStompR", beat+1)
			m.play(m.gHead, "SumoGStomp", beat+1)
			if m.sumoState == stateStomp && !m.isPlaying(m.inu, "InuFloatMiss") {
				m.play(m.inu, "InuFloat", beat+1)
			}
		})
	case 1:
		if prepareAnimation {
			m.ctx.At(beat, func() {
				m.play(m.pBody, "SumoStompPrepareL", beat)
				m.play(m.gBody, "SumoStompPrepareR", beat)
				m.play(m.pHead, "SumoPStomp", beat)
				m.play(m.gHead, "SumoGStomp", beat)
			})
		}
		m.ctx.At(beat, func() {
			m.previousState = stateStomp
			m.sumoStompDir = true
		})
		m.ctx.At(beat+1, func() {
			m.play(m.gHead, "SumoGStomp", beat+1)
			m.play(m.gBody, "SumoStompR", beat+1)
			if m.sumoState == stateStomp && !m.isPlaying(m.inu, "InuFloatMiss") {
				m.play(m.inu, "InuFloat", beat+1)
			}
		})
	case 2:
		if prepareAnimation {
			m.ctx.At(beat, func() {
				m.play(m.pBody, "SumoStompPrepareR", beat)
				m.play(m.gBody, "SumoStompPrepareL", beat)
				m.play(m.pHead, "SumoPStomp", beat)
				m.play(m.gHead, "SumoGStomp", beat)
			})
		}
		m.ctx.At(beat, func() {
			m.previousState = stateStomp
			m.sumoStompDir = false
		})
		m.ctx.At(beat+1, func() {
			m.play(m.gHead, "SumoGStomp", beat+1)
			m.play(m.gBody, "SumoStompL", beat+1)
			if m.sumoState == stateStomp && !m.isPlaying(m.inu, "InuFloatMiss") {
				m.play(m.inu, "InuFloat", beat+1)
			}
		})
	}

	nextType := 2
	if typ == 2 {
		nextType = 1
	}
	if startingLeftAfterTransition && typ == 3 {
		nextType = 1
	}
	target := beat + 1
	m.ctx.ScheduleInputCond(target, func() bool {
		return !m.isPlaying(m.pBody, "SumoStompMiss")
	}, func(state float64, _ engine.Judgment) {
		m.stompHit(m.ctx.Beat(), state)
	}, func() {
		m.stompMiss(m.ctx.Beat())
	})
	m.ctx.At(beat, func() {
		m.stompRecursive(beat+2, remaining, nextType, false, autoDecreaseRemaining, true)
	})
}

func (m *Module) slapSignal(beat float64, mute, inu bool) {
	if m.sumoState == stateSlap || m.cueActive {
		return
	}
	m.nextSwitchBeat = m.ctx.NextSwitchBeat(beat)
	m.cueRunning(beat + 3)
	m.sumoSlapDir = 0
	m.ctx.At(beat+3, func() {
		m.allowBopSumo = false
		m.play(m.pBody, "SumoSlapPrepare", beat+3)
		m.play(m.gBody, "SumoSlapPrepare", beat+3)
		m.play(m.pHead, "SumoPSlap", beat+3)
		m.play(m.gHead, "SumoGSlap", beat+3)
		m.play(m.bgStatic, "empty", beat+3)
		m.play(m.glasses, "glassesGone", beat+3)
		m.bgType = bgNone
		if m.previousState == statePose {
			m.play(m.pBody, "SumoPoseSwitch", beat+3)
			m.play(m.gBody, "SumoPoseSwitch", beat+3)
			m.play(m.pHead, "SumoPStomp", beat+3)
			m.play(m.gHead, "SumoGStomp", beat+3)
		}
	})
	if inu {
		m.ctx.At(beat, func() {
			m.allowBopInu = false
			m.play(m.inu, "InuBeatChange", beat)
		})
		for i := 1; i <= 3; i++ {
			b := beat + float64(i)
			m.ctx.At(b, func() { m.play(m.inu, "InuBeatChange", b) })
		}
		m.ctx.At(beat+3, func() { m.allowBopInu = true })
	}
	if !mute {
		m.slapSignalSound(beat)
	}
	m.previousState = m.sumoState
	m.sumoState = stateSlap
	m.slapRecursive(beat+4, 4, false, false)
	m.ctx.At(beat+4, func() { m.previousState = stateSlap })
}

func (m *Module) slapSignalSound(beat float64) {
	for i := 0; i < 4; i++ {
		m.soundAt(beat+float64(i), "slapSignal")
	}
}

func (m *Module) slapRecursive(beat, remaining float64, autoDecreaseRemaining, slapSwitch bool) {
	if m.sumoState != stateSlap || autoDecreaseRemaining {
		remaining--
	}
	if remaining <= 0 {
		return
	}
	if remaining <= 1 && (m.sumoState == stateStomp || slapSwitch) {
		m.ctx.At(beat-0.5, func() { m.sumoSlapDir = 2 })
	}
	m.ctx.ScheduleInput(beat, func(state float64, _ engine.Judgment) {
		m.slapHit(m.ctx.Beat(), state)
	}, func() {
		m.slapMiss(m.ctx.Beat())
	})
	if beat >= m.nextSwitchBeat-1 {
		remaining = 0
	}
	m.ctx.At(beat, func() { m.slapRecursive(beat+1, remaining, autoDecreaseRemaining, slapSwitch) })
}

func (m *Module) lookAtCamera(beat, length float64) {
	if m.sumoState != stateSlap {
		return
	}
	m.ctx.At(beat, func() {
		m.lookingAtCamera = true
		m.play(m.pHead, "SumoPSlapLook", beat)
		m.play(m.gHead, "SumoGSlapLook", beat)
	})
	end := beat + length
	m.ctx.At(end, func() {
		m.lookingAtCamera = false
		if m.sumoState == stateSlap {
			m.play(m.pHead, "SumoPSlap", end)
			m.play(m.gHead, "SumoGSlap", end)
		}
	})
}

func (m *Module) endPose(beat float64, randomPose bool, poseType, backgroundType int, confetti, alternateBG, throwGlasses bool) {
	if m.cueActive {
		return
	}
	m.cueRunning(beat + 3.5)
	m.previousState = m.sumoState
	m.sumoState = statePose
	if randomPose {
		if m.sumoPoseTypeNext > 0 && m.sumoPoseTypeNext < 4 {
			poseType = 1 + m.rng.Intn(2)
			if poseType >= m.sumoPoseTypeNext {
				poseType++
			}
		} else {
			poseType = 1 + m.rng.Intn(3)
		}
	}
	if alternateBG {
		if m.bgTypeNext != bgGreatWave {
			backgroundType = bgGreatWave
		} else {
			backgroundType = bgOtaniOniji
		}
	}
	if !throwGlasses && poseType == poseFinale {
		poseType = poseFinaleNoThrow
	}
	target := beat + 4
	m.ctx.ScheduleInputAction(target, actionAlt, func(state float64, _ engine.Judgment) {
		m.poseHit(m.ctx.Beat(), state)
	}, func() {
		m.poseMiss(m.ctx.Beat())
	})
	m.soundAt(beat, "poseSignal")
	m.ctx.At(beat+3, func() { m.poseLoopStop = m.ctx.SoundLoop("poseSignal") })
	m.ctx.At(beat, func() {
		m.allowBopInu = false
		m.play(m.inu, "InuAlarm", beat)
	})
	m.ctx.At(beat+3, func() {
		m.allowBopInu = true
		m.play(m.inu, "InuIdle", beat+3)
		if m.goBopInu {
			m.play(m.inu, "InuBop", beat+3)
		}
	})
	m.ctx.At(beat+3.5, func() {
		m.allowBopSumo = false
		m.sumoPoseConfetti = confetti
		m.sumoPoseTypeNext = poseType
		m.bgTypeNext = backgroundType
	})
	m.ctx.At(beat+4, func() { m.previousState = statePose })
	m.ctx.At(beat+4.5, func() { m.allowBopSumo = true })
}

func (m *Module) poseHit(beat, state float64) {
	m.stopPoseLoop()
	m.ctx.Sound("poseSignalEnd")
	m.sumoPoseCurrent = poseSuffix(m.sumoPoseTypeNext)
	m.sumoPoseType = m.sumoPoseTypeNext
	if m.sumoPoseType == poseFinale {
		m.play(m.glasses, "glassesThrow", beat)
	}
	if m.sumoPoseType == poseFinaleNoThrow {
		m.play(m.gBody, "SumoPoseG4Alt", beat)
		m.play(m.gHead, "SumoGPoseAlt4", beat)
		m.sumoPoseType = poseFinale
		m.sumoPoseTypeNext = poseFinale
		m.sumoPoseCurrent = "4"
	} else {
		m.play(m.gBody, "SumoPoseG"+m.sumoPoseCurrent, beat)
		m.play(m.gHead, "SumoGPose"+m.sumoPoseCurrent, beat)
	}
	m.play(m.pBody, "SumoPoseP"+m.sumoPoseCurrent, beat)
	m.play(m.bgStatic, bgDarkStateName(m.bgType), beat)
	m.bgType = m.bgTypeNext
	if m.bgType == bgNerd {
		m.ctx.Sound("Goofy")
	}
	m.ctx.At(beat, func() { m.play(m.bgMove, bgStateName(m.bgType), beat) })
	m.ctx.At(beat+2, func() {
		m.play(m.bgMove, "empty", beat+2)
		m.play(m.bgStatic, bgStateName(m.bgType), beat+2)
	})
	if m.barely(state) {
		m.ctx.Sound("tink")
		m.play(m.pHead, "SumoPPoseBarely"+m.sumoPoseCurrent, beat)
	} else {
		m.play(m.pHead, "SumoPPose"+m.sumoPoseCurrent, beat)
		if m.sumoPoseConfetti {
			m.spawnConfetti(beat)
		}
	}
	m.ctx.Sound("pose")
}

func (m *Module) poseMiss(beat float64) {
	m.stopPoseLoop()
	m.ctx.Sound("poseSignalEnd")
	m.ctx.Sound("miss")
	if m.sumoPoseTypeNext == poseFinaleNoThrow {
		m.sumoPoseTypeNext = poseFinale
	}
	m.sumoPoseType = m.sumoPoseTypeNext
	m.sumoPoseCurrent = "Miss" + poseSuffix(m.sumoPoseTypeNext)
	m.play(m.bgStatic, "empty", beat)
	m.play(m.pHead, "SumoPPose"+poseSuffix(m.sumoPoseType), beat)
	m.play(m.gHead, "SumoGPose"+poseSuffix(m.sumoPoseType), beat)
	m.play(m.pBody, "SumoPoseP"+m.sumoPoseCurrent, beat)
	m.play(m.gBody, "SumoPoseG"+m.sumoPoseCurrent, beat)
	if m.sumoPoseType == poseFinale {
		m.play(m.gHead, "SumoGPoseAlt4", beat)
	}
}

func (m *Module) slapHit(beat, state float64) {
	if m.barely(state) {
		m.ctx.Sound("tink")
		if m.lookingAtCamera {
			m.play(m.pHead, "SumoPSlapLookBarely", beat)
			m.play(m.gHead, "SumoGSlapLook", beat)
		} else {
			m.play(m.pHead, "SumoPSlapBarely", beat)
			m.play(m.gHead, "SumoGSlap", beat)
		}
	} else if m.lookingAtCamera {
		m.play(m.pHead, "SumoPSlapLook", beat)
		m.play(m.gHead, "SumoGSlapLook", beat)
	} else {
		m.play(m.pHead, "SumoPSlap", beat)
		m.play(m.gHead, "SumoGSlap", beat)
	}
	if m.sumoSlapDir == 1 {
		m.sumoSlapDir = 0
	} else if m.sumoSlapDir == 0 {
		m.sumoSlapDir = 1
	}
	m.ctx.Sound("slap")
	m.play(m.impact, "impact", beat)
	m.playSlapBodies(beat, true)
}

func (m *Module) slapMiss(beat float64) {
	if m.sumoSlapDir == 1 {
		m.sumoSlapDir = 0
	} else if m.sumoSlapDir == 0 {
		m.sumoSlapDir = 1
	}
	m.ctx.Sound("miss")
	m.playSlapBodies(beat, false)
	m.play(m.pBody, "SumoSlapMiss", beat)
	m.play(m.pHead, "SumoPMiss", beat)
	if m.lookingAtCamera {
		m.play(m.gHead, "SumoGSlapLook", beat)
	} else {
		m.play(m.gHead, "SumoGSlap", beat)
	}
	if m.sumoState == stateSlap {
		m.play(m.inu, "InuBopMiss", beat)
	}
}

func (m *Module) playSlapBodies(beat float64, player bool) {
	state := "SumoSlapBack"
	switch m.sumoSlapDir {
	case 2:
		state = "SumoSlapToStomp"
	case 1:
		state = "SumoSlapFront"
	}
	if player {
		m.play(m.pBody, state, beat)
	}
	m.play(m.gBody, state, beat)
}

func (m *Module) stompHit(beat, state float64) {
	if m.barely(state) {
		m.ctx.Sound("tink")
		m.play(m.pHead, "SumoPStompBarely", beat)
	} else {
		m.play(m.pHead, "SumoPStomp", beat)
	}
	m.ctx.Sound("stomp")
	if m.sumoStompDir {
		m.play(m.pBody, "SumoStompL", beat)
	} else {
		m.play(m.pBody, "SumoStompR", beat)
	}
	m.startStompShake(beat)
}

func (m *Module) stompMiss(beat float64) {
	if m.isPlaying(m.pBody, "SumoStompMiss") {
		return
	}
	m.ctx.Sound("miss")
	m.play(m.inu, "InuFloatMiss", beat)
	m.play(m.pBody, "SumoStompMiss", beat)
	m.play(m.pHead, "SumoPMiss", beat)
}

func (m *Module) cueRunning(until float64) {
	m.cueActive = true
	m.ctx.At(until, func() { m.cueActive = false })
}

func (m *Module) forceInputs(beat, length float64, typ, direction int, startCenter, slapSwitch, prepare bool) {
	switch typ {
	case forceStomp:
		stompType := 1
		startingLeftAfterTransition := false
		if direction == stompLeft {
			startingLeftAfterTransition = true
		}
		if direction == stompRight {
			stompType = 2
		}
		if startCenter {
			stompType = 3
		}
		stompAmount := (length + 1) / 2
		m.stompRecursive(beat-1, stompAmount+1, stompType, startingLeftAfterTransition, true, prepare)
	case forceSlap:
		m.sumoSlapDir = 0
		m.slapRecursive(beat, length+1, true, slapSwitch)
		if prepare {
			m.ctx.At(beat-1, func() {
				m.play(m.pBody, "SumoSlapPrepare", beat-1)
				m.play(m.gBody, "SumoSlapPrepare", beat-1)
				m.play(m.pHead, "SumoPSlap", beat-1)
				m.play(m.gHead, "SumoGSlap", beat-1)
			})
		}
		m.ctx.At(beat, func() { m.previousState = stateSlap })
	}
}
