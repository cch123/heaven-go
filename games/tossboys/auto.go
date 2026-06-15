package tossboys

func (m *Module) scheduleAutoDispense(ev dispenseEvent) {
	next, ok := m.passByBeat[beatKey(ev.beat+ev.length)]
	if !ok {
		return
	}
	isBlur := next.datamodel == "tossBoys/blur"
	curReceiver := next.who
	if isBlur {
		curReceiver = kidNone
	}
	m.scheduleAutoRec(ev.beat+ev.length, -1, ev.interval, ev.ignore, ev.callAuto, curReceiver, ev.who, isBlur, next.length, isSpecialEvent(next.datamodel), isBlur, next.datamodel)
}

func (m *Module) scheduleAutoRec(beat float64, index, interval int, ignore, call bool, curReceiver, previousReceiver int, isBlur bool, currentLength float64, special, force bool, datamodel string) {
	if interval <= 0 {
		interval = 2
	}
	if index%interval == 0 && !isBlur && !(ignore && special) {
		dispenseBeat := beat - 2
		who := curReceiver
		m.ctx.At(dispenseBeat, func() {
			if m.ball != nil || who == kidNone {
				return
			}
			m.dispenseSound(dispenseBeat, who, call)
			m.dispenseExec(dispenseBeat, 2, who, force, datamodel)
		})
	}
	if !isBlur && !(ignore && special) {
		index++
	}

	tempLast := previousReceiver
	lastLength := currentLength
	if isBlur {
		lastLength = 1
	}
	previousReceiver = curReceiver
	nextSpecial := special
	blurSet := isBlur
	nextForce := false

	if e, ok := m.passByBeat[beatKey(beat+lastLength)]; ok {
		if e.datamodel == "tossBoys/pop" {
			return
		}
		blurSet = e.datamodel == "tossBoys/blur"
		if blurSet {
			curReceiver = kidNone
		} else {
			curReceiver = e.who
		}
		currentLength = e.length
		nextSpecial = isSpecialEvent(e.datamodel)
		datamodel = e.datamodel
	} else {
		curReceiver = tempLast
		nextForce = true
	}
	nextBeat := beat + lastLength
	m.ctx.At(nextBeat-2, func() {
		m.scheduleAutoRec(nextBeat, index, interval, ignore, call, curReceiver, previousReceiver, blurSet, currentLength, nextSpecial, nextForce, datamodel)
	})
}
