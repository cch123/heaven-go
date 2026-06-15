package tossboys

import "hsdemo/engine"

func (m *Module) dispense(ev dispenseEvent, playSound bool) {
	if playSound && m.ball == nil {
		m.dispenseSound(ev.beat, ev.who, ev.call)
	}
	m.dispenseExec(ev.beat, ev.length, ev.who, false, "")
}

func (m *Module) dispenseExec(beat, length float64, who int, forcePass bool, forceDatamodel string) {
	if m.ball != nil || m.ballTemplate == nil {
		return
	}
	m.setReceiver(who)
	if k := m.currentReceiver(); k != nil {
		m.showArrow(k, beat, length-1)
	}
	m.ctx.Scene.PlayState(roleOr(m.ctx, "hatchAnim", "HatchHolder"), "HatchOpen", beat, m.ctx.SecPerBeat(beat))
	m.ball = newBall(m, beat, dispenseState(who), length)

	next, hasNext := m.passByBeat[beatKey(beat+length)]
	if hasNext || forcePass {
		targetBeat := beat + length
		datamodel := forceDatamodel
		if hasNext {
			datamodel = next.datamodel
		}
		m.scheduleHit(targetBeat, m.currentKid, m.justHitBall, m.miss)
		switch {
		case isSpecialEvent(datamodel):
			m.ctx.At(targetBeat-1, func() { m.doSpecial(targetBeat - 1) })
		case datamodel == "tossBoys/pop":
			m.ctx.At(targetBeat-1, func() {
				if k := m.currentReceiver(); k != nil {
					k.popBallPrepare(targetBeat - 1)
				}
			})
		}
		return
	}
	m.ctx.At(beat+length, func() { m.missAt(beat + length) })
}

func (m *Module) scheduleHit(beat float64, kid int, onHit func(float64, engine.Judgment, float64), onMiss func(float64)) {
	action := actionForKid(kid)
	m.ctx.ScheduleInputAction(beat, action, func(state float64, j engine.Judgment) {
		onHit(state, j, beat)
	}, func() { onMiss(beat) })
}

func (m *Module) showArrow(k *kidState, beat, length float64) {
	if k == nil || length <= 0 {
		return
	}
	comp, ok := componentByPath(m.ctx.Assets.Extra.Components, k.path)
	if !ok {
		return
	}
	arrow := comp.Refs["arrow"]
	if arrow == "" {
		return
	}
	m.ctx.At(beat, func() { m.ctx.Scene.SetActive(arrow, true) })
	m.ctx.At(beat+length, func() { m.ctx.Scene.SetActive(arrow, false) })
}

func (m *Module) determinePassValues(beat float64) {
	tempLast := m.lastKid
	m.lastKid = m.currentKid
	if ev, ok := m.passByBeat[beatKey(beat)]; ok {
		if ev.datamodel != "tossBoys/blur" {
			m.currentKid = ev.who
		}
		m.currentType = ev.datamodel
		m.currentLen = ev.length
	} else {
		m.currentKid = tempLast
	}
}

func (m *Module) determinePass(beat float64, barely bool) {
	m.determinePassValues(beat)
	switch m.currentType {
	case "tossBoys/pass":
		m.passBall(beat, m.currentLen)
	case "tossBoys/dual":
		m.dualToss(beat, m.currentLen)
	case "tossBoys/high":
		m.highToss(beat, m.currentLen)
	case "tossBoys/lightning":
		m.lightningToss(beat, m.currentLen)
	}
	if m.ball != nil {
		m.ball.playHit(m, beat, barely)
	}
	if ev, ok := m.passByBeat[beatKey(beat+m.currentLen)]; ok && ev.datamodel == "tossBoys/pop" {
		m.ctx.At(beat+m.currentLen-1, func() {
			if k := m.currentReceiver(); k != nil {
				k.popBallPrepare(beat + m.currentLen - 1)
			}
		})
	}
}

func (m *Module) passBall(beat, length float64) {
	last, current := kidColor(m.lastKid, false), kidColor(m.currentKid, true)
	state, secondBeat, _, thirdOffset := passState(last, current, "")
	if m.ball != nil {
		m.ball.setState(m, state, beat, length)
	}
	m.playPassSounds(beat, last, current, "", secondBeat, 0, thirdOffset)
	if ev, ok := m.passByBeat[beatKey(beat+length)]; ok && isSpecialEvent(ev.datamodel) {
		m.ctx.At(beat+length-1, func() { m.doSpecial(beat + length - 1) })
	}
	m.scheduleHit(beat+length, m.currentKid, m.justHitBall, m.miss)
}

func (m *Module) dualToss(beat, length float64) {
	last, current := kidColor(m.lastKid, false), kidColor(m.currentKid, true)
	state, secondBeat, secondOffset, thirdOffset := passState(last, current, "Dual")
	if m.ball != nil {
		m.ball.setState(m, state, beat, length)
	}
	m.playPassSounds(beat, last, current, "Low", secondBeat, secondOffset, thirdOffset)
	if ev, ok := m.passByBeat[beatKey(beat+length)]; ok && (ev.datamodel == "tossBoys/lightning" || ev.datamodel == "tossBoys/blur") {
		m.ctx.At(beat+length-1, func() { m.doSpecial(beat + length - 1) })
	}
	stopSpecial := false
	if ev, ok := m.passByBeat[beatKey(beat+length)]; ok {
		stopSpecial = ev.datamodel == "tossBoys/pass" || ev.datamodel == "tossBoys/high" || ev.datamodel == "tossBoys/pop"
	}
	if stopSpecial {
		m.scheduleHit(beat+length, m.currentKid, m.justHitBallUnSpecial, m.miss)
	} else {
		m.scheduleHit(beat+length, m.currentKid, m.justHitBall, m.miss)
	}
}

func (m *Module) highToss(beat, length float64) {
	last, current := kidColor(m.lastKid, false), kidColor(m.currentKid, true)
	state, secondBeat, _, thirdOffset := passState(last, current, "High")
	if m.ball != nil {
		m.ball.setState(m, state, beat, length)
	}
	m.playPassSounds(beat, last, current, "High", secondBeat, 0, thirdOffset)
	if ev, ok := m.passByBeat[beatKey(beat+length)]; ok && isSpecialEvent(ev.datamodel) {
		m.ctx.At(beat+length-1, func() { m.doSpecial(beat + length - 1) })
	}
	m.scheduleHit(beat+length, m.currentKid, m.justHitBall, m.miss)
}

func (m *Module) lightningToss(beat, length float64) {
	last, current := kidColor(m.lastKid, false), kidColor(m.currentKid, true)
	_, secondBeat, secondOffset, thirdOffset := passState(last, current, "Dual")
	if m.ball != nil {
		m.ball.setState(m, kidColor(m.lastKid, true)+"Keep", beat, length/2)
	}
	m.playPassSounds(beat, last, current, "Low", secondBeat, secondOffset, thirdOffset)
	if ev, ok := m.passByBeat[beatKey(beat+length)]; ok && (ev.datamodel == "tossBoys/dual" || ev.datamodel == "tossBoys/blur") {
		m.ctx.At(beat+length-1, func() { m.doSpecial(beat + length - 1) })
	}
	stopSpecial := false
	if ev, ok := m.passByBeat[beatKey(beat+length)]; ok {
		stopSpecial = ev.datamodel == "tossBoys/pass" || ev.datamodel == "tossBoys/high" || ev.datamodel == "tossBoys/pop"
	}
	if stopSpecial {
		m.scheduleHit(beat+length/2, m.lastKid, m.justKeepUnSpecial, m.miss)
	} else {
		m.scheduleHit(beat+length/2, m.lastKid, m.justKeep, m.miss)
	}
	m.scheduleHit(beat+length, m.currentKid, m.justHitBall, m.miss)
}

func (m *Module) blurToss(beat float64) {
	if m.ball != nil {
		m.ball.setState(m, kidColor(m.currentKid, true)+"Blur", beat, 0)
	}
	m.scheduleHit(beat+2, m.currentKid, m.justKeepContinue, m.miss)
}

func passState(last, current, suffix string) (state string, secondBeat, secondOffset, thirdOffset float64) {
	secondBeat = 0.5
	state = last + current + suffix
	switch last + current {
	case "blueRed", "yellowRed":
		if suffix == "" || suffix == "High" {
			secondBeat = 0.25
			if suffix == "" {
				secondBeat = 0.5
			}
		} else {
			secondBeat = 0.25
			if last == "blue" {
				thirdOffset = 0.020
			}
		}
	case "redYellow":
		if suffix == "" {
			secondBeat = 0.5
			thirdOffset = 0.060
		} else if suffix == "Dual" {
			secondOffset = 0.060
		}
	default:
		if suffix == "" {
			secondBeat = 1
		}
	}
	return state, secondBeat, secondOffset, thirdOffset
}

func dispenseState(who int) string {
	switch who {
	case kidAo:
		return "BlueDispense"
	case kidKii:
		return "YellowDispense"
	default:
		return "RedDispense"
	}
}
