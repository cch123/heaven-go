package wariodemambo

import "hsdemo/engine"

func (m *Module) bop(beat float64, mambo, dancers, lights bool) {
	if m.canBop {
		if mambo {
			switch m.bopState {
			case bopThink:
				m.play(m.warioBody, "WahThink", beat)
			case bopReady:
				m.play(m.warioBody, "WahBopReady", beat)
			case bopHappy:
				m.play(m.warioBody, "WahBopHap", beat)
			case bopFail:
				m.play(m.warioBody, "WahBad", beat)
			default:
				m.play(m.warioBody, "WahBopNorm", beat)
			}
		}
		if dancers {
			switch m.dancerBopState {
			case bopThink:
				m.play(m.dancerL, "ReadyDance", beat)
				m.play(m.dancerR, "ReadyDance", beat)
			case bopReady:
				m.play(m.dancerL, "DPreDancePose", beat)
				m.play(m.dancerR, "DPreDancePose", beat)
			case bopFail:
				m.play(m.dancerL, "DancerBad", beat)
				m.play(m.dancerR, "DancerBad", beat)
			default:
				m.play(m.dancerL, "DancerDance", beat)
				m.play(m.dancerR, "DancerDance", beat)
			}
		}
	}
	if lights {
		m.playLights(beat)
	}
}

func (m *Module) playLights(beat float64) {
	top, floor := "WTLightsIdle", "WBLightsIdle"
	switch m.lightsStage {
	case lightsStage1:
		top, floor = "WTLightsStage1", "WBLightsStage1"
		if !m.gameDim {
			top, floor = "WTLightsStage2", "WBLightsStage2"
		}
	case lightsStage2:
		top, floor = "WTLightsStageAlt", "WBLightsStageAlt"
		if !m.gameDim {
			top, floor = "WTLightsStageAlt2", "WBLightsStageAlt2"
		}
	case lightsStage3:
		top, floor = "WTLightsStage3", "WBLightsStage3"
		if !m.gameDim {
			top, floor = "WTLightsStage4", "WBLightsStage4"
		}
	}
	m.play(m.topLight, top, beat)
	m.play(m.leftLight, floor, beat)
	m.play(m.rightLight, floor, beat)
}

func (m *Module) startSpotEase(beat, length float64, ease int, lTarget, rTarget [2]float64) {
	m.spotLEase = spotEase{beat: beat, length: length, ease: ease, from: m.spotLPos, to: lTarget, active: true}
	m.spotREase = spotEase{beat: beat, length: length, ease: ease, from: m.spotRPos, to: rTarget, active: true}
}

func (m *Module) moveSpotlights(beat, length float64, start bool) {
	if m.intervalIsGoingOn(m.currentBeat()) {
		return
	}
	ease := easeLinear
	if start {
		ease = easeInQuad
	}
	m.startSpotEase(beat, length, ease, m.randomSpot(), m.randomSpot())
}

func (m *Module) randomSpot() [2]float64 {
	return [2]float64{
		-5 + m.rng.Float64()*10,
		-3.5 + m.rng.Float64()*2.4,
	}
}

func (m *Module) updateSpotlights(beat float64) {
	if m.spotLEase.active {
		m.spotLPos = evalSpot(m.spotLEase, beat)
		if beat >= m.spotLEase.beat+m.spotLEase.length {
			m.spotLEase.active = false
			if m.spotsPos == spotsRandom {
				m.moveSpotlights(beat, spotRandMinLen+m.rng.Float64()*(spotRandMaxLen-spotRandMinLen), false)
			}
		}
	}
	if m.spotREase.active {
		m.spotRPos = evalSpot(m.spotREase, beat)
		if beat >= m.spotREase.beat+m.spotREase.length {
			m.spotREase.active = false
		}
	}
	m.ctx.Scene.SetPosOver(m.spotL, m.spotLPos[0], m.spotLPos[1])
	m.ctx.Scene.SetPosOver(m.spotR, m.spotRPos[0], m.spotRPos[1])
}

func evalSpot(ev spotEase, beat float64) [2]float64 {
	u := 1.0
	if ev.length > 0 {
		u = clamp01((beat - ev.beat) / ev.length)
	}
	u = engine.Ease(ev.ease, 0, 1, u)
	return [2]float64{
		ev.from[0] + (ev.to[0]-ev.from[0])*u,
		ev.from[1] + (ev.to[1]-ev.from[1])*u,
	}
}

func (m *Module) updateDance(beat float64) {
	if !m.danceEase.active {
		return
	}
	u := 1.0
	if m.danceEase.length > 0 {
		u = clamp01((beat - m.danceEase.beat) / m.danceEase.length)
	}
	m.ctx.Scene.PlayNormalized(m.warioJump, m.danceEase.clip, u)
	if u >= 1 {
		m.danceEase.active = false
	}
}

func (m *Module) applyColors() {
	add := [4]float64{}
	if !m.gameDim {
		if m.gameRed {
			add = m.redAdd
		} else {
			add = m.blueAdd
		}
	}
	for _, path := range m.tintPaths {
		m.ctx.Scene.SetMaterialOver(path, [4]float64{1, 1, 1, 1}, add)
	}
	alpha := 0.75
	if !m.gameDim {
		alpha = 1
	}
	col := [4]float64{1, 1, 1, alpha}
	m.ctx.Scene.SetColorOver(m.spotL, col)
	m.ctx.Scene.SetColorOver(m.spotR, col)
}
