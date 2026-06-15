package monkeywatch

import (
	"math"
	"sort"
)

func (m *Module) buildMonkeyTimeline() {
	m.monkeyEvents = nil
	for _, c := range m.claps {
		m.monkeyEvents = append(m.monkeyEvents, monkeyTimelineEvt{beat: c.beat, length: c.length, kind: eventClap, min: c.min})
	}
	for _, p := range m.pinks {
		kind := eventPink
		if p.interval {
			kind = eventPinkInterval
		}
		m.monkeyEvents = append(m.monkeyEvents, monkeyTimelineEvt{beat: p.beat, length: p.length, kind: kind})
	}
	sort.SliceStable(m.monkeyEvents, func(i, j int) bool { return m.monkeyEvents[i].beat < m.monkeyEvents[j].beat })
}

func (m *Module) startSecondForClap(idx int) int {
	cur := m.claps[idx]
	if !cur.auto {
		return cur.min % 60
	}
	lastKnownPosition := 0
	lastKnownBeat := 0.0
	for _, ev := range m.monkeyEventsBefore(cur.beat) {
		startPosition := lastKnownPosition
		if !m.switchBetween(lastKnownBeat, ev.beat) && ev.beat > lastKnownBeat {
			startPosition += round((ev.beat - lastKnownBeat) / 2)
		}
		monkeys := 0
		eventEnd := ev.beat
		switch ev.kind {
		case eventClap:
			startPosition = ev.min
			lookahead := m.lookaheadEnd(ev.beat)
			monkeys = round((lookahead - ev.beat) / 2)
			eventEnd = lookahead
		case eventPink:
			monkeys = ceil(ev.length)
			eventEnd = ev.beat + ev.length
		case eventPinkInterval:
			monkeys = len(m.customBetween(ev.beat, ev.beat+ev.length))
			eventEnd = ev.beat + ev.length
		}
		lastKnownPosition = startPosition + monkeys
		lastKnownBeat = eventEnd
	}
	final := lastKnownPosition
	if !m.switchBetween(lastKnownBeat, cur.beat) && cur.beat > lastKnownBeat {
		final += round((cur.beat - lastKnownBeat) / 2)
	}
	return ((final % 60) + 60) % 60
}

func (m *Module) monkeyEventsBefore(beat float64) []monkeyTimelineEvt {
	var out []monkeyTimelineEvt
	for _, ev := range m.monkeyEvents {
		if ev.beat < beat {
			out = append(out, ev)
		}
	}
	return out
}

func (m *Module) switchBetween(a, b float64) bool {
	if m.ctx == nil {
		return false
	}
	for _, e := range m.ctx.Entities() {
		if e.Game() == "gameManager" && (actionNameGM(e.Datamodel) == "switchGame" || actionNameGM(e.Datamodel) == "end") && e.Beat > a && e.Beat < b {
			return true
		}
	}
	return false
}

func (m *Module) lookaheadEnd(beat float64) float64 {
	next := math.Inf(1)
	for _, ev := range m.monkeyEvents {
		if ev.beat > beat && ev.beat < next {
			next = ev.beat
		}
	}
	if m.ctx != nil {
		for _, e := range m.ctx.Entities() {
			if e.Game() == "gameManager" && (actionNameGM(e.Datamodel) == "switchGame" || actionNameGM(e.Datamodel) == "end") && e.Beat > beat && e.Beat < next {
				next = e.Beat
			}
		}
	}
	if math.IsInf(next, 1) {
		if m.ctx == nil {
			return beat
		}
		next = m.ctx.NextSwitchBeat(beat)
	}
	return next
}

func actionNameGM(datamodel string) string {
	if len(datamodel) > len("gameManager/") && datamodel[:len("gameManager/")] == "gameManager/" {
		return datamodel[len("gameManager/"):]
	}
	return datamodel
}

func (m *Module) pinkAtBeat(beat float64) (pinkEvt, bool) {
	for _, p := range m.pinks {
		if !p.interval && math.Abs(p.beat-beat) < 1e-4 {
			return p, true
		}
	}
	return pinkEvt{}, false
}

func (m *Module) customPinkAtBeat(beat float64) (pinkEvt, bool) {
	for _, p := range m.pinks {
		if p.interval && math.Abs(p.beat-beat) < 1e-4 {
			return p, true
		}
	}
	return pinkEvt{}, false
}

func (m *Module) customBetween(a, b float64) []customPinkEvt {
	var out []customPinkEvt
	for _, c := range m.custom {
		if c.beat >= a && c.beat < b {
			out = append(out, c)
		}
	}
	return out
}

func (m *Module) schedulePinkSounds() {
	for _, p := range m.pinks {
		p := p
		if p.interval {
			m.schedulePinkSoundCustom(p)
		} else {
			m.schedulePinkSound(p)
		}
	}
}

func (m *Module) schedulePinkSound(p pinkEvt) {
	if !p.muteOoki {
		for _, s := range []struct {
			name string
			off  float64
		}{
			{"voiceUki1", -2}, {"voiceUki1Echo1", -1.75}, {"voiceUki2", -1}, {"voiceUki2Echo1", -0.75}, {"voiceUki3", 0}, {"voiceUki3Echo1", 0.25},
		} {
			m.ctx.SoundAt(p.beat+s.off, s.name, 1)
		}
	}
	if !p.muteEek {
		for i := 0; i < int(p.length); i++ {
			b := p.beat + float64(i)
			ki := soundChoice("voiceKi", b, 2)
			echo := ki + "Echo" + itoa(1+int(seed01(b, 9)*2))
			m.ctx.SoundAt(b+0.5, ki, 1)
			m.ctx.SoundAt(b+0.75, echo, 1)
		}
	}
}

func (m *Module) schedulePinkSoundCustom(p pinkEvt) {
	customs := m.customBetween(p.beat, p.beat+p.length)
	if !p.muteOoki {
		for _, s := range []struct {
			name string
			off  float64
		}{
			{"voiceUki1", -2}, {"voiceUki1Echo1", -1.75}, {"voiceUki2", -1}, {"voiceUki2Echo1", -0.75},
		} {
			m.ctx.SoundAt(p.beat+s.off, s.name, 1)
		}
		hasAtStart := false
		for _, c := range customs {
			hasAtStart = hasAtStart || math.Abs(c.beat-p.beat) < 1e-4
		}
		if !hasAtStart {
			m.ctx.SoundAt(p.beat, "voiceUki3", 1)
			m.ctx.SoundAt(p.beat+0.25, "voiceUki3Echo1", 1)
		}
	}
	if !p.muteEek {
		for _, c := range customs {
			ki := soundChoice("voiceKi", c.beat, 2)
			echo := ki + "Echo" + itoa(1+int(seed01(c.beat, 13)*2))
			m.ctx.SoundAt(c.beat, ki, 1)
			m.ctx.SoundAt(c.beat+0.25, echo, 1)
		}
	}
}

func (m *Module) scheduleClapSequences() {
	m.sequenceEnds = nil
	for _, c := range m.claps {
		skip := false
		for _, end := range m.sequenceEnds {
			if c.beat < end {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		end := m.ctx.NextSwitchBeat(c.beat)
		m.sequenceEnds = append(m.sequenceEnds, end)
		m.scheduleSequence(c.beat, end)
	}
}

func (m *Module) scheduleSequence(beat, end float64) {
	if beat >= end {
		return
	}
	if p, ok := m.pinkAtBeat(beat); ok {
		m.schedulePinkClap(beat, p.length, end)
		return
	}
	if p, ok := m.customPinkAtBeat(beat); ok {
		m.scheduleCustomPinkClap(beat, p.length, end)
		return
	}
	m.scheduleNormalClap(beat, end)
}

func (m *Module) scheduleNormalClap(beat, end float64) {
	m.ctx.At(beat-4, func() {
		m.spawnMonkey(beat, false, beat-4 < m.ctx.Beat())
		m.scheduleSequence(beat+2, end)
		m.cameraWantAngle += degreePerMonkey
	})
	m.ctx.At(beat-1, func() {
		if mk := m.monkeyAtBeat(beat); mk != nil {
			mk.prepare(beat, beat+1)
		}
	})
	m.ctx.At(beat, func() { m.cameraMoving = true })
}

func (m *Module) schedulePinkClap(beat, length, end float64) {
	m.ctx.At(beat-4, func() { m.scheduleSequence(beat+length, end) })
	m.ctx.At(beat, func() { m.cameraMoving = true })
	for i := 0; i < int(length); i++ {
		b := beat + float64(i)
		m.ctx.At(b-4, func() {
			m.spawnMonkey(b, true, b-4 < m.ctx.Beat())
			m.cameraWantAngle += degreePerMonkey
		})
		m.ctx.At(b-1, func() {
			if mk := m.monkeyAtBeat(b); mk != nil {
				mk.prepare(b, b+0.5)
			}
		})
	}
}

func (m *Module) scheduleCustomPinkClap(beat, length, end float64) {
	m.ctx.At(beat-4, func() { m.scheduleSequence(beat+length, end) })
	m.ctx.At(beat, func() { m.cameraMoving = true })
	for _, c := range m.customBetween(beat, beat+length) {
		b := c.beat
		m.ctx.At(b-4, func() {
			m.spawnMonkey(b, true, b-4 < m.ctx.Beat())
			m.cameraWantAngle += degreePerMonkey
		})
		m.ctx.At(b-1.5, func() {
			if mk := m.monkeyAtBeat(b); mk != nil {
				mk.prepare(b-0.5, b)
			}
		})
	}
}

func (m *Module) scheduleAppear() {
	for _, ap := range m.appear {
		ap := ap
		m.scheduleAppearEvent(ap, -1)
	}
}

func (m *Module) scheduleAppearEvent(ap appearEvt, gameSwitchBeat float64) {
	lastBeat := m.firstClapBeatBeforeOrAt(ap.beat)
	idx := 0
	for idx < ap.count {
		if p, ok := m.pinkAtBeat(lastBeat); ok {
			for i := 0; i < int(p.length) && idx < ap.count; i++ {
				spawnAt, monkeyBeat := ap.beat+float64(idx)*ap.length, lastBeat
				m.ctx.At(spawnAt, func() { m.spawnMonkey(monkeyBeat, true, gameSwitchBeat >= 0 && spawnAt < gameSwitchBeat) })
				idx++
				lastBeat++
			}
		} else if p, ok := m.customPinkAtBeat(lastBeat); ok {
			for _, c := range m.customBetween(lastBeat, lastBeat+p.length) {
				if idx >= ap.count {
					break
				}
				spawnAt, monkeyBeat := ap.beat+float64(idx)*ap.length, c.beat
				m.ctx.At(spawnAt, func() { m.spawnMonkey(monkeyBeat, true, gameSwitchBeat >= 0 && spawnAt < gameSwitchBeat) })
				idx++
			}
			lastBeat += p.length
		} else {
			spawnAt, monkeyBeat := ap.beat+float64(idx)*ap.length, lastBeat
			m.ctx.At(spawnAt, func() { m.spawnMonkey(monkeyBeat, false, gameSwitchBeat >= 0 && spawnAt < gameSwitchBeat) })
			idx++
			lastBeat += 2
		}
	}
}

func (m *Module) firstClapBeatBeforeOrAt(beat float64) float64 {
	last := math.Inf(-1)
	for _, c := range m.claps {
		if c.beat <= beat {
			last = c.beat
		}
	}
	if !math.IsInf(last, -1) {
		return last
	}
	if len(m.claps) > 0 {
		return m.claps[0].beat
	}
	return beat
}
