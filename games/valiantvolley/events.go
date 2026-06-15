package valiantvolley

import "sort"

func (m *Module) scheduleBops() {
	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() {
			if ev.single {
				for _, a := range m.ants {
					if a != nil {
						a.playAnimation(m, ev.which, ev.beat)
					}
				}
			}
			if ev.keep {
				m.bopStatus = ev.which
			}
		})
	}
}

func (m *Module) scheduleIntervalsAndObjects() {
	used := map[int]bool{}
	for _, iv := range m.intervals {
		iv := iv
		m.ctx.At(iv.beat, func() { m.startInterval(iv) })
		m.ctx.At(iv.beat+iv.length, func() { m.passTurn(iv) })
		group := m.hitsInInterval(iv)
		for typ := objDirt; typ <= objFruit; typ++ {
			events := group[typ]
			if len(events) == 0 {
				continue
			}
			for _, ev := range events {
				used[eventKey(ev)] = true
				m.schedulePrepare(ev)
			}
			m.spawnVolleyObject(m.multiSpawn(events, iv))
		}
	}
	for _, ev := range m.hits {
		if used[eventKey(ev)] {
			continue
		}
		m.schedulePrepare(ev)
		m.spawnVolleyObject(objectPlan{
			start: ev.beat - ev.length, distance: ev.length, typ: ev.typ,
		})
	}
}

func (m *Module) hitsInInterval(iv intervalEvt) map[int][]hitEvt {
	out := map[int][]hitEvt{objDirt: {}, objFruit: {}}
	for _, ev := range m.hits {
		if ev.beat >= iv.beat && ev.beat < iv.beat+iv.length {
			out[ev.typ] = append(out[ev.typ], ev)
		}
	}
	for k := range out {
		sort.Slice(out[k], func(i, j int) bool { return out[k][i].beat < out[k][j].beat })
	}
	return out
}

func (m *Module) multiSpawn(events []hitEvt, iv intervalEvt) objectPlan {
	first := events[0]
	plan := objectPlan{
		start: first.beat - iv.length, distance: iv.length, typ: first.typ,
		juggle: true, intervalStart: iv.beat, intervalLen: iv.length,
	}
	for i, ev := range events {
		plan.juggleLengths = append(plan.juggleLengths, ev.length)
		if i > 0 {
			plan.inputs = append(plan.inputs, ev.beat)
		}
		if nearly(ev.beat+ev.length, iv.beat+iv.length) {
			plan.lastJuggle = ev.beat
			// The current-game code adds beat-multiIntervalStartBeat to the
			// last event length; this preserves that WIP timing quirk.
			plan.lastJuggleLength = ev.length + first.beat - iv.beat
		}
	}
	return plan
}

func (m *Module) startInterval(iv intervalEvt) {
	m.multiInputInterval = true
	m.multiIntervalBeat = iv.beat
	m.multiIntervalLength = iv.length
	for _, a := range m.ants {
		if a != nil {
			a.cantBop = true
		}
	}
}

func (m *Module) passTurn(iv intervalEvt) {
	m.multiInputInterval = false
	for _, a := range m.ants {
		if a != nil {
			a.justHit = false
		}
	}
}

func (m *Module) schedulePrepare(ev hitEvt) {
	if !ev.shouldPrep {
		return
	}
	prepBeat := ev.beat - ev.prepBeats
	m.ctx.At(prepBeat, func() {
		for _, a := range m.ants {
			if a != nil {
				a.queuePrepare = prepBeat
			}
		}
	})
}

func eventKey(ev hitEvt) int {
	return int(ev.beat*1000+0.5)*10 + ev.typ
}
