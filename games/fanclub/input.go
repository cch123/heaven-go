package fanclub

import "hsdemo/engine"

func (m *Module) prepare(beat float64, typ int) {
	p := m.player()
	if p == nil {
		return
	}
	switch typ {
	case 1:
		m.ctx.ScheduleInputRelease(beat,
			func(state float64, _ engine.Judgment) { p.jumpStartNow(m, beat, state < 1 && state > -1) },
			func() { m.angerOnMiss() })
	case 2:
		m.ctx.ScheduleInput(beat,
			func(state float64, _ engine.Judgment) { p.clapStart(m, beat, state < 1 && state > -1, true, 0) },
			func() { m.angerOnMiss() })
	case 3:
		m.ctx.ScheduleInput(beat,
			func(state float64, _ engine.Judgment) { p.clapStart(m, beat, state < 1 && state > -1, false, 1) },
			func() { m.angerOnMiss() })
	default:
		m.ctx.ScheduleInput(beat,
			func(state float64, _ engine.Judgment) { p.clapStart(m, beat, state < 1 && state > -1, false, 0.1) },
			func() { m.angerOnMiss() })
	}
}
