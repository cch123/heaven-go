package freezeframe

import (
	"math"

	"hsdemo/engine"
)

type carSpawnEvt struct {
	beat, length float64
	kind         carType
	near         bool
	state        string
}

func (m *Module) scheduleCarCues() {
	for _, cue := range m.cues {
		cue := cue
		if !cue.mute {
			if cue.kind == carFast {
				m.ctx.SoundAt(cue.beat, "fastCarFar", 1)
				m.ctx.At(cue.beat+2, func() {
					if m.ctx.GameAt(cue.beat) == gameID {
						m.ctx.Sound("fastCarNear")
					}
				})
			} else {
				m.ctx.SoundAt(cue.beat, "slowCarFar", 1)
			}
		}
		target := cue.beat + 2
		m.ctx.ScheduleInput(target, func(state float64, _ engine.Judgment) {
			args := photoArgs{car: cue.kind, typ: cue.photo, clear: cue.clear}
			switch {
			case state >= 1:
				args.state = 1
			case state <= -1:
				args.state = -1
			default:
				args.state = 0
			}
			m.pushPhoto(args, cue.beat)
			m.cameraFlash(m.ctx.Beat())
		}, func() {
			m.pushPhoto(photoArgs{car: cue.kind, typ: cue.photo, state: -2, clear: cue.clear}, cue.beat)
		})
	}
}

func (m *Module) scheduleAutoShows() {
	shown := []float64{}
	for _, cue := range m.cues {
		if !cue.autoShow {
			continue
		}
		showOffset := 3.0
		if m.hasCarAt(cue.beat - 1) {
			showOffset = 3.5
		}
		if cue.kind == carFast {
			showOffset = 4
		}
		if m.hasCarAt(cue.beat - 2) {
			showOffset = 4.5
		}
		showBeat := cue.beat + showOffset
		if m.hasCarBetween(cue.beat, showBeat) {
			continue
		}
		overlap := false
		for _, b := range shown {
			if showBeat >= b && showBeat < b+2 {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}
		shown = append(shown, showBeat)
		cue := cue
		m.ctx.At(showBeat, func() {
			m.showPhotos(showBeat, 2, cue.grade, cue.audience, false)
		})
	}
}

func (m *Module) buildCarSpawns() {
	for _, cue := range m.cues {
		if cue.kind == carFast {
			m.spawns = append(m.spawns,
				carSpawnEvt{beat: cue.beat - 0.5, length: 1, kind: carFast, near: false, state: "FastCarGo"},
				carSpawnEvt{beat: cue.beat + 2 - 0.09375, length: 3.0 / 16.0, kind: carFast, near: true, state: "FastCarGo"},
			)
			continue
		}
		m.spawns = append(m.spawns, carSpawnEvt{beat: cue.beat + 2 - 1.0/6.0, length: 1.0 / 3.0, kind: carSlow, near: true, state: "SlowCarGo"})
	}

	slowBeats := []float64{}
	for _, cue := range m.cues {
		if cue.kind == carSlow {
			slowBeats = append(slowBeats, cue.beat)
		}
	}
	for len(slowBeats) > 0 {
		minBeat := slowBeats[0]
		maxBeat := minBeat
		cluster := []float64{}
		rest := slowBeats[:0]
		for _, b := range slowBeats {
			if b >= minBeat && b < minBeat+2 {
				cluster = append(cluster, b)
				if b > maxBeat {
					maxBeat = b
				}
			} else {
				rest = append(rest, b)
			}
		}
		slowBeats = rest
		midBeat := minBeat + (maxBeat-minBeat)/2
		for _, b := range cluster {
			diff := midBeat - b
			modifiedBeat := midBeat + diff/4
			m.spawns = append(m.spawns, carSpawnEvt{beat: modifiedBeat - 4, length: 3, kind: carSlow, near: false, state: "SlowCarGo"})
		}
	}
}

func (m *Module) scheduleIntroLights(ev introLightsEvt) {
	if !ev.on {
		m.ctx.At(ev.beat, func() { m.ctx.Scene.PlayStateLayer("freezeFrame/intro/lights", m.introSign, "LightsOff", ev.beat, 0.5) })
		return
	}
	m.ctx.SoundAt(ev.beat, "beginningSignal1", 1)
	m.ctx.SoundAt(ev.beat+ev.length, "beginningSignal1", 1)
	m.ctx.SoundAt(ev.beat+2*ev.length, "beginningSignal2", 1)
	m.ctx.At(ev.beat, func() { m.ctx.Scene.PlayStateLayer("freezeFrame/intro/lights", m.introSign, "Light01", ev.beat, 0.5) })
	m.ctx.At(ev.beat+ev.length, func() {
		m.ctx.Scene.PlayStateLayer("freezeFrame/intro/lights", m.introSign, "Light02", ev.beat+ev.length, 0.5)
	})
	m.ctx.At(ev.beat+2*ev.length, func() {
		m.ctx.Scene.PlayStateLayer("freezeFrame/intro/lights", m.introSign, "Light03", ev.beat+2*ev.length, 0.5)
	})
}

func (m *Module) hasCarAt(beat float64) bool {
	for _, cue := range m.cues {
		if math.Abs(cue.beat-beat) < 1e-6 {
			return true
		}
	}
	return false
}

func (m *Module) hasCarBetween(from, to float64) bool {
	for _, cue := range m.cues {
		if cue.beat > from && cue.beat <= to {
			return true
		}
	}
	return false
}
