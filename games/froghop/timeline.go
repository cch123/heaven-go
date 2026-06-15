package froghop

import (
	"math"

	"hsdemo/engine"
)

func (m *Module) scheduleCount(ev countEvt) {
	p := pitchAt(m.ctx, ev.beat, m.pitches)
	if ev.leader {
		for i := 0; i < 4; i++ {
			b := ev.beat + float64(i)
			m.ctx.SoundAtPitchPan(b, "SE_NTR_FROG_EN_COUNT"+itoa(i+1), 1, p, 0)
			m.ctx.At(b, func() { m.talk([]*frog{m.leader}, "Wide", ev.beat) })
		}
	}
	if ev.backup {
		for i := 0; i < 4; i++ {
			b := ev.beat + float64(i)
			m.ctx.SoundAtPitchPan(b, "SE_NTR_FROG_EN_COUNT"+itoa(i+1)+"_EXTRAS_CUSTOM", 1, p, 0)
			m.ctx.SoundAtPitchPan(b, "SE_NTR_FROG_EN_COUNT"+itoa(i+1)+"_PLAYER_CUSTOM", 1, p, 0)
			m.ctx.At(b, func() { m.talk(m.back, "Wide", ev.beat) })
		}
	}
	if ev.start {
		m.addHopEvent(ev.beat + 4)
	}
}

func (m *Module) scheduleCountForce(ev countForceEvt) {
	n := ev.number + 1
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	p := pitchAt(m.ctx, ev.beat, m.pitches)
	if ev.leader {
		m.ctx.SoundAtPitchPan(ev.beat, "SE_NTR_FROG_EN_COUNT"+itoa(n), 1, p, 0)
		m.ctx.At(ev.beat, func() { m.talk([]*frog{m.leader}, "Wide", ev.beat) })
	}
	if ev.backup {
		m.ctx.SoundAtPitchPan(ev.beat, "SE_NTR_FROG_EN_COUNT"+itoa(n)+"_EXTRAS_CUSTOM", 1, p, 0)
		m.ctx.SoundAtPitchPan(ev.beat, "SE_NTR_FROG_EN_COUNT"+itoa(n)+"_PLAYER_CUSTOM", 1, p, 0)
		m.ctx.At(ev.beat, func() { m.talk(m.back, "Wide", ev.beat) })
	}
}

func (m *Module) scheduleRegularHops() {
	coveredUntil := math.Inf(-1)
	for _, h := range m.hops {
		if h.stop {
			continue
		}
		if h.beat < coveredUntil {
			continue
		}
		end := m.nextHopStopOrSwitch(h.beat)
		coveredUntil = end
		m.scheduleRegularWindow(h.beat, end)
	}
}

func (m *Module) addHopEvent(beat float64) {
	m.hops = append(m.hops, hopEvt{beat: beat})
	sortHopEvents(m.hops)
}

func sortHopEvents(h []hopEvt) {
	for i := 1; i < len(h); i++ {
		for j := i; j > 0 && h[j].beat < h[j-1].beat; j-- {
			h[j], h[j-1] = h[j-1], h[j]
		}
	}
}

func (m *Module) scheduleRegularWindow(start, end float64) {
	if math.IsInf(end, 1) {
		end = start + 256
	}
	first := math.Ceil(start - 1e-6)
	for b := first; b < end; b++ {
		target := b
		if m.inNoHop(target) {
			continue
		}
		m.ctx.ScheduleInputCond(target, func() bool { return m.regularActiveAt(target) && !m.inNoHop(target) },
			func(state float64, _ engine.Judgment) { m.playerHopNormal(target, state) },
			func() { m.playerMiss(target, true) })
		m.ctx.At(target, func() { m.npcHop(m.other, target, false) })
		if !m.inBackHop(target) {
			m.ctx.At(target, func() {
				m.npcHop(m.front, target, false)
				m.ctx.Sound("SE_NTR_FROG_EN_E_BEAT")
			})
		}
	}
}

func (m *Module) nextHopStopOrSwitch(beat float64) float64 {
	next := m.ctx.NextSwitchBeat(beat)
	for _, h := range m.hops {
		if h.beat > beat && h.stop && h.beat < next {
			next = h.beat
		}
	}
	return next
}

func (m *Module) regularActiveAt(beat float64) bool {
	active := false
	for _, h := range m.hops {
		if h.beat > beat {
			break
		}
		active = !h.stop
	}
	return active
}

func (m *Module) inNoHop(target float64) bool {
	for _, c := range m.cues {
		if target >= c.beat+2 && target < c.beat+4 {
			return true
		}
	}
	return false
}

func (m *Module) inBackHop(target float64) bool {
	for _, c := range m.cues {
		if target >= c.beat && target < c.beat+4 {
			return true
		}
	}
	return false
}

func (m *Module) scheduleCue(ev cueEvt) {
	switch ev.kind {
	case "two":
		m.scheduleTwo(ev)
	case "three":
		m.scheduleThree(ev)
	case "spin":
		m.scheduleSpin(ev)
	}
}

func (m *Module) scheduleTwo(ev cueEvt) {
	m.cueCommon(ev.beat, ev.spotlights, 0)
	p := pitchAt(m.ctx, ev.beat, m.pitches)
	if ev.enabled {
		m.ctx.SoundAtPitchPan(ev.beat, "SE_NTR_FROG_EN_T_HA", 1, p, 0)
		m.ctx.SoundAt(ev.beat, "SE_NTR_FROG_EN_POP_DEFAULT", 1)
		m.ctx.SoundAtPitchPan(ev.beat+0.5, "SE_NTR_FROG_EN_T_HAAI", 1, p, 0)
		m.ctx.SoundAt(ev.beat+0.5, "SE_NTR_FROG_EN_POP_HAAI", 1)
	}
	m.ctx.At(ev.beat, func() {
		m.npcHop(m.front, ev.beat, false)
		if ev.enabled {
			m.talk([]*frog{m.leader}, "Wide", ev.beat)
		}
	})
	m.ctx.At(ev.beat+0.5, func() {
		m.npcHop(m.front, ev.beat+0.5, true)
		if ev.enabled {
			end := ev.beat + 1.5
			if ev.jazz {
				end = ev.beat + 2.25
			}
			m.talk([]*frog{m.leader}, "Narrow", end)
		}
	})
	m.ctx.At(ev.beat+2, func() { m.npcHop(m.other, ev.beat+2, false); m.talk(m.back, "Wide", ev.beat) })
	m.ctx.At(ev.beat+2.5, func() {
		end := ev.beat + 3.5
		if ev.jazz {
			end = ev.beat + 4.25
		}
		m.npcHop(m.other, ev.beat+2.5, true)
		m.talk(m.back, "Narrow", end)
	})
	m.ctx.SoundAtPitchPan(ev.beat+2, "SE_NTR_FROG_EN_E_HA", 1, p, 0)
	m.ctx.SoundAtPitchPan(ev.beat+2.5, "SE_NTR_FROG_EN_E_HAAI", 1, p, 0)
	m.ctx.ScheduleInput(ev.beat+2, func(state float64, _ engine.Judgment) { m.playerHopYa(ev.beat+2, state) }, func() { m.playerMiss(ev.beat+2, true) })
	m.ctx.ScheduleInput(ev.beat+2.5, func(state float64, _ engine.Judgment) { m.playerHopHoo(ev.beat+2.5, state) }, func() { m.playerMiss(ev.beat+2.5, true) })
}

func (m *Module) scheduleThree(ev cueEvt) {
	m.cueCommon(ev.beat, ev.spotlights, 0)
	p := pitchAt(m.ctx, ev.beat, m.pitches)
	if ev.enabled {
		for _, off := range []float64{0, 0.5, 1} {
			m.ctx.SoundAtPitchPan(ev.beat+off, "SE_NTR_FROG_EN_T_HAI", 1, p, 0)
			m.ctx.SoundAt(ev.beat+off, "SE_NTR_FROG_EN_POP_DEFAULT", 1)
		}
	}
	m.ctx.At(ev.beat, func() {
		m.npcHop(m.front, ev.beat, false)
		if ev.enabled {
			end := ev.beat
			if ev.jazz {
				end = ev.beat + 1.75
			}
			m.talk([]*frog{m.leader}, "Narrow", end)
		}
	})
	m.ctx.At(ev.beat+0.5, func() {
		m.npcHop(m.front, ev.beat+0.5, false)
		if ev.enabled && !ev.jazz {
			m.talk([]*frog{m.leader}, "Narrow", ev.beat)
		}
	})
	m.ctx.At(ev.beat+1, func() {
		m.npcHop(m.front, ev.beat+1, true)
		if ev.enabled && !ev.jazz {
			m.talk([]*frog{m.leader}, "Narrow", ev.beat)
		}
	})
	for _, off := range []float64{2, 2.5, 3} {
		o := off
		m.ctx.At(ev.beat+o, func() {
			m.npcHop(m.other, ev.beat+o, o == 3)
			end := ev.beat
			if ev.jazz {
				end = ev.beat + 3.75
			}
			m.talk(m.back, "Narrow", end)
		})
		m.ctx.SoundAtPitchPan(ev.beat+o, "SE_NTR_FROG_EN_E_HAI", 1, p, 0)
	}
	m.ctx.ScheduleInput(ev.beat+2, func(state float64, _ engine.Judgment) { m.playerHopYeah(ev.beat+2, state, false) }, func() { m.playerMiss(ev.beat+2, true) })
	m.ctx.ScheduleInput(ev.beat+2.5, func(state float64, _ engine.Judgment) { m.playerHopYeah(ev.beat+2.5, state, false) }, func() { m.playerMiss(ev.beat+2.5, true) })
	m.ctx.ScheduleInput(ev.beat+3, func(state float64, _ engine.Judgment) { m.playerHopYeah(ev.beat+3, state, true) }, func() { m.playerMiss(ev.beat+3, true) })
}

func (m *Module) scheduleSpin(ev cueEvt) {
	m.cueCommon(ev.beat, ev.spotlights, 1)
	p := pitchAt(m.ctx, ev.beat, m.pitches)
	if ev.enabled {
		m.ctx.SoundAtPitchPan(ev.beat, "SE_NTR_FROG_EN_T_KURU_1", 1, p, 0)
		m.ctx.SoundAtPitchPan(ev.beat+0.5, "SE_NTR_FROG_EN_T_KURU_2", 1, p, 0)
		m.ctx.SoundAtPitchPan(ev.beat+1, "SE_NTR_FROG_EN_T_LIN", 1, p, 0)
		m.ctx.SoundAt(ev.beat+1, "SE_NTR_FROG_EN_T_SPIN", 1)
	}
	m.ctx.At(ev.beat, func() {
		m.npcCharge(m.front, ev.beat)
		if ev.enabled {
			m.talk([]*frog{m.leader}, "Narrow", ev.beat)
		}
	})
	m.ctx.At(ev.beat+1, func() {
		m.npcSpin(m.front, ev.beat+1, ev.hs)
		if ev.enabled {
			m.talk([]*frog{m.leader}, "Wide", ev.beat)
		}
	})
	m.ctx.At(ev.beat+2, func() { m.npcCharge(m.other, ev.beat+2); m.talk(m.back, "Narrow", ev.beat) })
	m.ctx.At(ev.beat+3, func() { m.npcSpin(m.other, ev.beat+3, false); m.talk(m.back, "Wide", ev.beat) })
	m.ctx.SoundAtPitchPan(ev.beat+2, "SE_NTR_FROG_EN_E_KURU_1", 1, p, 0)
	m.ctx.SoundAtPitchPan(ev.beat+2.5, "SE_NTR_FROG_EN_E_KURU_2", 1, p, 0)
	m.ctx.SoundAtPitchPan(ev.beat+3, "SE_NTR_FROG_EN_E_LIN", 1, p, 0)
	m.ctx.SoundAt(ev.beat+3, "SE_NTR_FROG_EN_E_SPIN", 1)
	m.ctx.ScheduleInputAction(ev.beat+2, actionAlt, func(state float64, _ engine.Judgment) { m.playerCharge(ev.beat+2, state) }, func() { m.playerMiss(ev.beat+2, false) })
	m.ctx.ScheduleInputActionRelease(ev.beat+3, actionAlt, func(state float64, _ engine.Judgment) { m.playerSpin(ev.beat+3, state) }, func() { m.playerMissNoFlip(ev.beat + 3) })
}

func (m *Module) cueCommon(beat float64, spotlights bool, spin float64) {
	if !spotlights {
		return
	}
	m.ctx.At(beat+1.5+spin, func() { m.setSpotlights(false, true, true) })
	m.ctx.At(beat+3.5+spin, func() { m.setSpotlights(true, false, true) })
}

func (m *Module) scheduleThank(ev thankEvt) {
	p := 1.0
	if ev.manual {
		p = ev.pitch
	} else if ev.pitched {
		p = m.ctx.BPMAt(ev.beat) / 156
	}
	m.ctx.SoundAtPitchOff(ev.beat, "tyvm", 1, p, 0.2/p)
	m.ctx.At(ev.beat, func() { m.singer.bop(ev.beat) })
	for _, s := range []struct {
		off   float64
		state string
		end   float64
	}{
		{0, "Narrow", ev.beat}, {0.5, "Narrow", ev.beat},
		{2, "Wide", ev.beat + 4}, {4.5, "Narrow", ev.beat}, {5.5, "Narrow", ev.beat},
	} {
		s := s
		m.ctx.At(ev.beat+s.off, func() { m.talk([]*frog{m.singer}, s.state, s.end) })
	}
}

func (m *Module) scheduleMouth(ev mouthEvt) {
	frogs := m.frogsFor(ev.blue, ev.orange, ev.green)
	end := ev.beat + ev.length - 0.5
	if ev.wink {
		end = ev.beat + ev.length
		for _, f := range frogs {
			f.wink(ev.beat, ev.state, end)
		}
		return
	}
	m.talk(frogs, ev.state, end)
}

func (m *Module) scheduleForce(ev forceEvt) {
	for i := 0; i < int(ev.length); i++ {
		b := ev.beat + float64(i)
		if ev.front {
			m.ctx.At(b, func() { m.npcHop(m.front, b, false) })
		}
		if ev.back {
			m.ctx.At(b, func() {
				m.npcHop(m.other, b, false)
				m.ctx.Sound("SE_NTR_FROG_EN_E_BEAT")
			})
			m.ctx.ScheduleInput(b, func(state float64, _ engine.Judgment) { m.playerHopNormal(b, state) }, func() { m.playerMiss(b, true) })
		}
	}
}
