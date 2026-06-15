package fanclub

import "hsdemo/riq"

func (m *Module) scheduleHai(beat float64, noArisa, noCrowd bool) {
	if !noCrowd {
		m.playSequence("crowd_hai", beat+4)
	}
	if !noArisa {
		m.playSequence("arisa_hai", beat)
	}
	m.ctx.At(beat, func() {
		m.responseToggle = false
		m.noBop = interval{beat: beat, length: 8}
		m.prepare(beat+3, 0)
		m.prepare(beat+4, 0)
		m.prepare(beat+5, 0)
		m.prepare(beat+6, 0)
	})
	for _, off := range []float64{0, 1, 2} {
		o := off
		m.ctx.At(beat+o, func() { m.doIdolPeace(true, beat+o) })
	}
	m.ctx.At(beat+2.5, func() { m.disableSpecBop(beat+2.5, 5) })
	m.ctx.At(beat+3, func() {
		m.doIdolPeace(false, beat+3)
		m.playPrepare(beat + 3)
	})
	for _, off := range []float64{4, 5, 6, 7} {
		o := off
		m.ctx.At(beat+o, func() {
			m.playOneClap(beat+o, -1)
			m.doIdolClaps(beat + o)
		})
	}
}

func (m *Module) scheduleKamone(beat float64, noArisa, noCrowd bool, responseType int, alt bool) {
	if !noCrowd {
		if alt {
			m.playSequence("crowd_iina", beat+2)
		} else {
			m.playSequence("crowd_kamone", beat+2)
		}
	}
	if !noArisa {
		switch {
		case alt && (responseType == responseThroughFast || responseType == responseJumpFast):
			m.playSequence("arisa_iina_fast", beat)
		case alt:
			m.playSequence("arisa_iina", beat)
		case responseType == responseThroughFast || responseType == responseJumpFast:
			m.playSequence("arisa_kamone_fast", beat)
		default:
			m.playSequence("arisa_kamone", beat)
		}
	}
	doJump := responseType == responseJump || responseType == responseJumpFast
	isBig := responseType == responseThroughFast || responseType == responseJumpFast
	m.ctx.At(beat, func() {
		m.noResponse = interval{beat: beat, length: 2}
		m.responseToggle = true
		if doJump {
			m.noBop = interval{beat: beat, length: 6.25}
		} else {
			m.noBop = interval{beat: beat, length: 5.25}
		}
		m.disableSpecBop(beat+0.5, 6)
		m.prepare(beat+1, 3)
		m.prepare(beat+2.5, 0)
		m.prepare(beat+3, 2)
		m.prepare(beat+4, 1)
		m.doIdolCall(0, isBig, beat)
		m.blueD.playState(m.ctx.Scene, "Beat", beat, m.ctx.SecPerBeat(beat))
		m.orangeD.playState(m.ctx.Scene, "Beat", beat, m.ctx.SecPerBeat(beat))
	})
	call1 := 0.75
	if isBig {
		call1 = 1
	}
	m.ctx.At(beat+call1, func() { m.doIdolCall(1, isBig, beat+call1) })
	m.ctx.At(beat+1, func() {
		m.playPrepare(beat + 1)
		m.bopDancers(beat + 1)
	})
	m.ctx.At(beat+2, func() {
		m.playLongClap(beat + 2)
		m.doIdolResponse(beat + 2)
		m.bopDancers(beat + 2)
	})
	m.ctx.At(beat+3, func() {
		m.doIdolResponse(beat + 3)
		m.bopDancers(beat + 3)
	})
	m.ctx.At(beat+3.5, func() { m.playOneClap(beat+3.5, -1) })
	m.ctx.At(beat+4, func() {
		m.playChargeClap(beat + 4)
		m.doIdolResponse(beat + 4)
		m.bopDancers(beat + 4)
	})
	m.ctx.At(beat+5, func() {
		m.playJump(beat + 5)
		if doJump {
			m.doIdolJump(beat+5, 3)
			m.blueD.doJump(beat + 5)
			m.orangeD.doJump(beat + 5)
		} else {
			m.doIdolResponse(beat + 5)
			m.bopDancers(beat + 5)
		}
	})
}

func (m *Module) scheduleBigReady(beat float64, noCall bool) {
	if !noCall {
		m.playSequence("crowd_big_ready", beat)
	}
	m.ctx.At(beat, func() {
		m.prepare(beat+1.5, 0)
		m.prepare(beat+2, 0)
		m.disableSpecBop(beat, 3.75)
		m.playAnimationAll("FanBigReady", true, beat)
	})
	m.ctx.At(beat+1.5, func() { m.playAnimationAll("FanBigReady", true, beat+1.5) })
	m.ctx.At(beat+2, func() { m.playAnimationAll("FanBigReady", true, beat+2) })
	m.ctx.At(beat+2.5, func() { m.playOneClap(beat+2.5, -1) })
	m.ctx.At(beat+3, func() { m.playOneClap(beat+3, -1) })
}

func (m *Module) scheduleFaceposer(e *riq.Entity) {
	beat, length := e.Beat, e.Length
	enable := boolDefault(e, "poserOn", true)
	who := int(e.Float("who", 0))
	mouthOn, eyeOn := boolDefault(e, "mouthOn", true), boolDefault(e, "eyeOn", true)
	mouth, mouthEnd := int(e.Float("mouth", 0)), int(e.Float("mouthEnd", 0))
	eyeL, eyeR := int(e.Float("eyeL", 0)), int(e.Float("eyeR", 0))
	eyeLB, eyeRB := int(e.Float("eyeLBackup", 0)), int(e.Float("eyeRBackup", 0))
	eyeX, eyeY := e.Float("eyex", 0), e.Float("eyey", 0)
	m.ctx.At(beat, func() {
		if who == 0 {
			m.setArisaFaceposer(enable, mouthOn, eyeOn, mouth, mouthEnd, eyeL, eyeR, eyeX, eyeY, beat, length)
			return
		}
		d := &m.blueD
		if who == 1 {
			d = &m.orangeD
		}
		d.setFaceposer(m, enable, mouthOn, eyeOn, mouth, mouthEnd, eyeLB, eyeRB, beat, length)
	})
}

func (m *Module) finalCheer(beat float64) {
	if m.noJudgement {
		return
	}
	m.noJudgement = true
	m.noJudgementInput = false
	type cue struct {
		off   float64
		who   int
		vol   float64
		pitch float64
	}
	cues := []cue{
		{0, 1, 0.6, 1},
		{2.0 / 3.0, 0, 0.5, 0.98}, {2.0 / 3.0, 3, 0.5, 0.98},
		{2.0/3.0 + 0.25, 6, 0.6, 1}, {2.0/3.0 + 0.25, 8, 0.6, 1},
		{2.0/3.0 + 0.5, 7, 0.6, 1}, {2.0/3.0 + 0.5, 4, 0.6, 1},
		{1.5, 2, 0.6, 1}, {1.5, 11, 0.6, 1},
		{1.5 + 1.0/3.0, 5, 0.6, 1}, {1.5 + 1.0/3.0, 10, 0.6, 1},
		{2 + 1.0/3.0, 9, 0.6, 1},
	}
	for _, c := range cues {
		c := c
		m.ctx.SoundAtPitchPan(beat+c.off, "play_jump", c.vol, c.pitch, 0)
		m.ctx.At(beat+c.off, func() { m.startClapLoop(beat+c.off, c.who) })
	}
	m.ctx.At(beat+6, func() {
		if !m.noJudgementInput {
			m.angerOnMiss()
			m.ctx.ScoreMiss()
		}
	})
}
