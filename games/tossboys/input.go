package tossboys

import "hsdemo/engine"

func (m *Module) justHitBall(state float64, _ engine.Judgment, beat float64) {
	m.justHitBallCore(state, beat, false)
}

func (m *Module) justHitBallUnSpecial(state float64, _ engine.Judgment, beat float64) {
	m.unspecial()
	m.justHitBallCore(state, beat, false)
}

func (m *Module) justHitBallCore(state, beat float64, _ bool) {
	if m.ball == nil {
		return
	}
	if ev, ok := m.passByBeat[beatKey(beat)]; ok {
		switch ev.datamodel {
		case "tossBoys/pop":
			m.popCurrent(beat)
			return
		case "tossBoys/blur":
			m.justKeepCurrent(state, beat)
			m.blurToss(beat)
			return
		default:
			if ev.who == m.currentKid {
				m.missAt(beat)
				return
			}
		}
	}
	barely := state >= 1 || state <= -1
	if k := m.currentReceiver(); k != nil {
		if barely {
			k.barely(beat)
		} else {
			k.hitBall(beat, true)
		}
	}
	m.determinePass(beat, barely)
}

func (m *Module) justKeepContinue(state float64, _ engine.Judgment, beat float64) {
	if m.ball == nil {
		return
	}
	if ev, ok := m.passByBeat[beatKey(beat)]; ok {
		if ev.datamodel == "tossBoys/pass" || ev.datamodel == "tossBoys/high" || ev.datamodel == "tossBoys/pop" {
			m.unspecial()
		}
		m.justHitBallCore(state, beat, false)
		return
	}
	m.justKeepCurrent(state, beat)
	m.scheduleHit(beat+1, m.currentKid, m.justKeepContinue, m.miss)
}

func (m *Module) justKeepCurrent(state, beat float64) {
	if m.ball == nil {
		return
	}
	m.ctx.Sound(kidColor(m.currentKid, false) + "Keep")
	m.ball.setState(m, kidColor(m.currentKid, true)+"Keep", beat, 0)
	barely := state >= 1 || state <= -1
	if barely {
		m.ball.playHit(m, beat, true)
		if k := m.currentReceiver(); k != nil {
			k.barely(beat)
		}
		return
	}
	if k := m.currentReceiver(); k != nil {
		k.hitBall(beat, true)
	}
	m.ball.playHit(m, beat, false)
}

func (m *Module) justKeep(state float64, _ engine.Judgment, beat float64) {
	if m.ball == nil {
		return
	}
	m.ctx.Sound(kidColor(m.lastKid, false) + "Keep")
	last, current := kidColor(m.lastKid, false), kidColor(m.currentKid, true)
	ballState, _, _, _ := passState(last, current, "Dual")
	m.ball.setState(m, ballState, beat, m.currentLen/2)
	barely := state >= 1 || state <= -1
	if barely {
		m.ball.playHit(m, beat, true)
		if k := m.receiver(m.lastKid); k != nil {
			k.barely(beat)
		}
		return
	}
	if k := m.receiver(m.lastKid); k != nil {
		k.hitBall(beat, true)
	}
	m.ball.playHit(m, beat, false)
}

func (m *Module) justKeepUnSpecial(state float64, j engine.Judgment, beat float64) {
	m.unspecial()
	m.justKeep(state, j, beat)
}

func (m *Module) miss(beat float64) { m.missAt(beat) }

func (m *Module) missAt(beat float64) {
	if m.ball == nil {
		return
	}
	if k := m.currentReceiver(); k != nil {
		k.miss(beat)
	}
	for _, k := range m.kids {
		k.crouch = false
	}
	m.unspecial()
	m.ball.dead = true
	m.ball = nil
	m.ctx.Sound("misshit")
	m.determinePassValues(beat)
}

func (m *Module) popCurrent(beat float64) {
	if m.ball == nil {
		return
	}
	if k := m.currentReceiver(); k != nil {
		k.popBall(beat)
	}
	m.ball.dead = true
	m.ball = nil
	m.ctx.Sound(kidColor(m.currentKid, false) + "Pop")
}
