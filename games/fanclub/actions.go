package fanclub

func (m *Module) bopSingle(target int, beat float64) {
	if target == idolBopNone {
		return
	}
	sec := m.ctx.SecPerBeat(beat)
	if target == idolBopBoth || target == idolBopIdol {
		if !m.noBop.contains(beat) {
			m.ctx.Scene.PlayState(m.arisa, "IdolBeat"+m.performanceSuffix(), beat, sec)
			m.blueD.playState(m.ctx.Scene, "Beat", beat, sec)
			m.orangeD.playState(m.ctx.Scene, "Beat", beat, sec)
		}
	}
	if target == idolBopBoth || target == idolBopSpectators {
		if !m.noSpecBop.contains(beat) {
			for _, f := range m.fans {
				f.bop(m, beat)
			}
		}
	}
}

func (m *Module) doIdolPeace(sync bool, beat float64) {
	if m.noCall.contains(beat) {
		return
	}
	state := "IdolPeace"
	if !sync {
		state = "IdolPeaceNoSync"
	}
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayState(m.arisa, state+m.performanceSuffix(), beat, sec)
	m.blueD.playState(m.ctx.Scene, "Peace", beat, sec)
	m.orangeD.playState(m.ctx.Scene, "Peace", beat, sec)
}

func (m *Module) doIdolClaps(beat float64) {
	if m.responseToggle || m.noResponse.contains(beat) {
		return
	}
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayState(m.arisa, "IdolCrap"+m.performanceSuffix(), beat, sec)
	m.blueD.playState(m.ctx.Scene, "Crap", beat, sec)
	m.orangeD.playState(m.ctx.Scene, "Crap", beat, sec)
}

func (m *Module) doIdolResponse(beat float64) {
	if !m.responseToggle || m.noResponse.contains(beat) {
		return
	}
	m.ctx.Scene.PlayState(m.arisa, "IdolResponse"+m.performanceSuffix(), beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) doIdolCall(part int, big bool, beat float64) {
	if m.noCall.contains(beat) {
		return
	}
	prefix := "IdolCall"
	if big {
		prefix = "IdolBigCall"
	}
	m.ctx.Scene.PlayState(m.arisa, prefix+itoa(part)+m.performanceSuffix(), beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) doIdolJump(beat, length float64) {
	m.noBop = interval{beat: beat, length: length}
	m.noResponse = interval{beat: beat, length: length}
	m.idolJumpStart = beat
	m.ctx.Scene.PlayState(m.arisa, "IdolJump"+m.performanceSuffix(), beat, m.ctx.SecPerBeat(beat))
	m.ctx.At(beat+1, func() {
		m.ctx.Scene.PlayState(m.arisa, "IdolLand"+m.performanceSuffix(), beat+1, m.ctx.SecPerBeat(beat+1))
	})
}

func (m *Module) updateJump(beat float64) {
	if beat >= m.idolJumpStart && beat < m.idolJumpStart+1 {
		yw := parabola01(beat - m.idolJumpStart)
		m.ctx.Scene.SetPosOver(m.arisaRoot, 0, 2*yw+0.25)
		s := (1 - yw*0.8) * 1.18
		m.ctx.Scene.SetScaleOver(m.arisaShadow, s, s)
		return
	}
	m.ctx.Scene.SetPosOver(m.arisaRoot, 0, 0)
	m.ctx.Scene.SetScaleOver(m.arisaShadow, 1.18, 1.18)
}

func (m *Module) playIdolAnimation(beat, length float64, typ, who int) {
	m.idolJumpStart = -1e9
	m.noResponse = interval{beat: beat, length: length + 0.5}
	m.noBop = interval{beat: beat, length: length + 0.5}
	m.noCall = interval{beat: beat, length: length + 0.5}
	if who == 0 || who == 2 {
		m.orangeD.playAnim(m, beat, length, typ)
	}
	if who == 0 || who == 3 {
		m.blueD.playAnim(m, beat, length, typ)
	}
	if who != 0 && who != 1 {
		return
	}
	state := ""
	switch typ {
	case idolAnimBop:
		state = "IdolBeat"
	case idolAnimPeaceVocal:
		state = "IdolPeace"
	case idolAnimPeace:
		state = "IdolPeaceNoSync"
	case idolAnimClap:
		state = "IdolCrap"
	case idolAnimCall:
		m.doIdolCall(0, false, beat)
		m.ctx.At(beat+0.75, func() { m.doIdolCall(1, false, beat+0.75) })
	case idolAnimResponse:
		state = "IdolResponse"
	case idolAnimJump:
		m.doIdolJump(beat, length)
	case idolAnimBigCall:
		m.doIdolCall(0, true, beat)
		m.ctx.At(beat+length, func() { m.doIdolCall(1, true, beat+length) })
	case idolAnimSquat:
		m.ctx.Scene.PlayState(m.arisa, "IdolSquat0"+m.performanceSuffix(), beat, m.ctx.SecPerBeat(beat))
		m.ctx.At(beat+length, func() {
			m.ctx.Scene.PlayState(m.arisa, "IdolSquat1"+m.performanceSuffix(), beat+length, m.ctx.SecPerBeat(beat+length))
		})
	case idolAnimWink:
		m.ctx.Scene.PlayState(m.arisa, "IdolWink0"+m.performanceSuffix(), beat, m.ctx.SecPerBeat(beat))
		m.ctx.At(beat+length, func() {
			m.ctx.Scene.PlayState(m.arisa, "IdolWink1"+m.performanceSuffix(), beat+length, m.ctx.SecPerBeat(beat+length))
		})
	case idolAnimDab:
		state = "IdolDab"
		m.ctx.Sound("arisa_dab")
	}
	if state != "" {
		m.ctx.Scene.PlayState(m.arisa, state+m.performanceSuffix(), beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) playStage(typ int, beat float64) {
	switch typ {
	case stageReset:
		m.ctx.Scene.PlayState(m.stage, "Bg", beat, m.ctx.SecPerBeat(beat))
		m.toSpot(true)
	case stageFlash:
		m.ctx.Scene.PlayState(m.stage, "Bg_Light", beat, m.ctx.SecPerBeat(beat))
		m.toSpot(true)
	case stageSpot:
		m.ctx.Scene.PlayState(m.stage, "Bg_Spot", beat, m.ctx.SecPerBeat(beat))
		m.toSpot(false)
	}
}

func (m *Module) dancerTravel(beat, length float64, exit, instant bool) {
	if instant {
		m.blueD.finishEntrance(m.ctx.Scene, exit)
		m.orangeD.finishEntrance(m.ctx.Scene, exit)
		return
	}
	m.blueD.startEntrance(m.ctx.Scene, beat, length, exit)
	m.orangeD.startEntrance(m.ctx.Scene, beat, length, exit)
}

func (m *Module) playPrepare(beat float64) {
	for _, f := range m.fans {
		if beat >= f.jumpStart && beat < f.jumpStart+1 {
			continue
		}
		f.play(m, "FanPrepare", beat)
	}
}

func (m *Module) playAnimationAll(state string, onlyOverrideBop bool, beat float64) {
	for _, f := range m.fans {
		cur := f.inst.CurrentState("")
		if onlyOverrideBop && cur != "" && cur != "FanBeat" && cur != "NoPose" {
			continue
		}
		f.play(m, state, beat)
	}
}

func (m *Module) playOneClap(beat float64, who int) {
	if who >= 0 {
		if who == 3 {
			return
		}
		if who < len(m.fans) {
			m.ctx.SoundAtPitchOff(beat, "crap_impact", 0.1, 1, 0)
			m.fans[who].play(m, "FanClap", beat)
			m.ctx.At(beat+0.1, func() { m.fans[who].free(m, beat+0.1) })
		}
		return
	}
	for i, f := range m.fans {
		if i == 3 {
			continue
		}
		f.play(m, "FanClap", beat)
	}
	m.ctx.At(beat+0.1, func() {
		for i, f := range m.fans {
			if i != 3 {
				f.free(m, beat+0.1)
			}
		}
	})
}

func (m *Module) playLongClap(beat float64) {
	for i, f := range m.fans {
		if i != 3 {
			f.play(m, "FanClap", beat)
		}
	}
	m.ctx.At(beat+1, func() {
		for i, f := range m.fans {
			if i != 3 {
				f.free(m, beat+1)
			}
		}
	})
}

func (m *Module) playChargeClap(beat float64) {
	for i, f := range m.fans {
		if i != 3 {
			f.play(m, "FanClap", beat)
		}
	}
	m.ctx.At(beat+0.1, func() {
		for i, f := range m.fans {
			if i != 3 {
				f.play(m, "FanClapCharge", beat+0.1)
			}
		}
	})
}

func (m *Module) playJump(beat float64) {
	for i, f := range m.fans {
		if i == 3 {
			continue
		}
		f.jumpStart = beat
		f.play(m, "FanJump", beat)
		m.ctx.At(beat+1, func() { f.play(m, "FanPrepare", beat+1) })
	}
}

func (m *Module) bopDancers(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	m.blueD.playState(m.ctx.Scene, "Beat", beat, sec)
	m.orangeD.playState(m.ctx.Scene, "Beat", beat, sec)
}

func (m *Module) angerOnMiss() {
	for i := 0; i <= 5 && i < len(m.fans); i++ {
		if i == 3 {
			continue
		}
		state := "FanFaceAngry"
		if i > 3 {
			state = "FanFaceAngryFlip"
		}
		m.fans[i].play(m, state, m.ctx.Beat())
	}
}

func (m *Module) startClapLoop(beat float64, who int) {
	if who < 0 || who >= len(m.fans) {
		return
	}
	m.playOneClap(beat, who)
	m.ctx.At(beat+0.5, func() { m.startClapLoop(beat+0.5, who) })
}
