package lovelab

import (
	"sort"

	"hsdemo/riq"
)

func (m *Module) OnEvent(e *riq.Entity) {
	switch eventName(e) {
	case "bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop:  boolDefault(e, "toggle", true),
			auto: boolDefault(e, "toggle2", false),
		})
	case "beat intervals":
		m.intervals = append(m.intervals, intervalEvt{
			idx: len(m.intervals), beat: e.Beat, length: e.Length,
			autoPass: boolDefault(e, "auto", true),
		})
	case "boy shakes":
		m.shakes = append(m.shakes, shakeEvt{
			beat: e.Beat, length: e.Length,
			speed: intParam(e, "speed", flaskFast),
		})
	case "girl blush":
		m.blushes = append(m.blushes, blushEvt{beat: e.Beat, auto: boolDefault(e, "toggle", false)})
	case "boxGuy":
		m.boxes = append(m.boxes, boxEvt{beat: e.Beat, action: intParam(e, "type", boxTakeAway)})
	case "set object colors":
		m.colors = append(m.colors, colorEvt{
			beat: e.Beat,
			a:    colorParam(e, "colorA", m.boyLiquid),
			b:    colorParam(e, "colorB", m.girlLiquid),
			c:    colorParam(e, "colorC", m.weirdLiquid),
		})
	case "set time of day":
		m.times = append(m.times, timeEvt{beat: e.Beat, typ: intParam(e, "type", timeSunset)})
	case "clouds":
		m.cloudEvts = append(m.cloudEvts, cloudEvt{beat: e.Beat, on: boolDefault(e, "toggle", true)})
	case "spotlight":
		m.spots = append(m.spots, spotEvt{
			beat: e.Beat, active: boolDefault(e, "toggle", false),
			typ: intParam(e, "spotType", spotNormal), where: intParam(e, "posType", spotBoy),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.SliceStable(m.shakes, func(i, j int) bool { return m.shakes[i].beat < m.shakes[j].beat })
	sort.SliceStable(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })
	sort.SliceStable(m.times, func(i, j int) bool { return m.times[i].beat < m.times[j].beat })
	sort.SliceStable(m.cloudEvts, func(i, j int) bool { return m.cloudEvts[i].beat < m.cloudEvts[j].beat })
	sort.SliceStable(m.spots, func(i, j int) bool { return m.spots[i].beat < m.spots[j].beat })
	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() { m.bop(ev) })
	}
	for _, iv := range m.intervals {
		iv := iv
		m.ctx.At(iv.beat-0.9, func() { m.preInterval(iv, iv.beat) })
	}
	for _, ev := range m.colors {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setObjectColors(ev.a, ev.b, ev.c) })
	}
	for _, ev := range m.times {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setTimeOfDay(ev.typ) })
	}
	for _, ev := range m.cloudEvts {
		ev := ev
		m.ctx.At(ev.beat, func() { m.canCloudsMove = ev.on })
	}
	for _, ev := range m.spots {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setSpotlight(ev.active, ev.typ, ev.where) })
	}
	for _, ev := range m.boxes {
		ev := ev
		m.ctx.At(ev.beat, func() { m.boxGuy(ev.beat, ev.action) })
	}
	for _, ev := range m.blushes {
		ev := ev
		m.ctx.At(ev.beat, func() { m.play(m.labGirlHead, "GirlBlushFace", ev.beat) })
	}
}

func (m *Module) preInterval(iv intervalEvt, gameSwitchBeat float64) {
	if m.started[iv.idx] {
		return
	}
	shakes := m.shakesBetween(iv.beat, iv.beat+iv.length)
	if len(shakes) == 0 {
		return
	}
	m.started[iv.idx] = true
	m.currentHearts = append(m.currentHearts, len(shakes))
	m.hasMissed = false
	arc := ""
	if len(m.flaskArcsBoy) > 0 {
		arc = m.flaskArcsBoy[0]
		if shakes[len(shakes)-1].speed != flaskFast && len(m.flaskArcsBoy) > 1 {
			arc = m.flaskArcsBoy[1]
		}
	}
	m.spawnCustomFlask(shakes[0].beat, arc)
	m.ctx.At(iv.beat, func() {
		m.hasMissed = false
		if iv.autoPass {
			m.passToGirl(iv.beat+iv.length, iv.beat, iv.beat+iv.length, 1)
		}
	})
	for i, sh := range shakes {
		sh := sh
		if sh.beat < gameSwitchBeat-1e-6 {
			continue
		}
		m.boyShake(sh, iv.beat, iv.beat+iv.length, iv.length)
		if i == 0 {
			m.ctx.SoundAt(sh.beat, "leftCatch", 1)
		} else {
			m.ctx.SoundAt(sh.beat, "shakeDown", 1)
		}
		if sh.beat+sh.length >= iv.beat+iv.length-1e-6 {
			m.ctx.SoundAt(sh.beat+sh.length, "leftThrow", 1)
		} else {
			m.ctx.SoundAt(sh.beat+sh.length, "shakeUp", 1)
		}
	}
}

func (m *Module) boxGuy(beat float64, action int) {
	box := m.boxPerson
	if m.isDay {
		box = m.boxPersonDay
	}
	switch action {
	case boxPutBack:
		m.play(box, "BoxPutBack", beat)
	case boxNoBox:
		m.ctx.Scene.PlayState(box, "NoBox", beat, 0)
	case boxInstaBox:
		m.ctx.Scene.PlayState(box, "BoxIdle", beat, 0)
	default:
		m.play(box, "BoxTakeAway", beat)
	}
}
