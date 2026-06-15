package drummerduel

import (
	"math"

	"hsdemo/engine"
)

func (m *Module) resetCamera(loc int, x float64) {
	m.cameraLoc = loc
	m.cameraX, m.cameraFrom, m.cameraTo = x, x, x
	m.cameraStart, m.cameraLen, m.cameraEase = math.Inf(1), 0, 0
	m.cameraMoving = false
}

func (m *Module) moveCamera(beat, length float64, loc, ease int) {
	if length <= 0 {
		length = autoCamLength
	}
	m.cameraFrom = m.cameraAt(beat)
	m.cameraTo = m.cameraXFor(loc)
	m.cameraStart = beat
	m.cameraLen = length
	m.cameraEase = ease
	m.cameraLoc = loc
	m.cameraMoving = true
}

func (m *Module) updateCamera(beat float64) {
	m.cameraX = m.cameraAt(beat)
	if m.cameraMoving && beat >= m.cameraStart+m.cameraLen {
		m.cameraMoving = false
		m.cameraFrom = m.cameraTo
	}
}

func (m *Module) cameraAt(beat float64) float64 {
	if !m.cameraMoving || m.cameraLen <= 0 || math.IsInf(m.cameraStart, 1) {
		return m.cameraTo
	}
	u := (beat - m.cameraStart) / m.cameraLen
	if u <= 0 {
		return m.cameraFrom
	}
	if u >= 1 {
		return m.cameraTo
	}
	return engine.Ease(m.cameraEase, m.cameraFrom, m.cameraTo, u)
}

func (m *Module) cameraXFor(loc int) float64 {
	switch loc {
	case camLeft:
		return m.cameraLeft
	case camRight:
		return m.cameraRight
	default:
		return m.cameraCenter
	}
}

func (m *Module) restoreCamera(beat float64) {
	loc := camCenter
	for _, ev := range m.intervals {
		if ev.beat > beat {
			break
		}
		if !ev.camMove {
			continue
		}
		switch {
		case beat < ev.beat:
			loc = camLeft
		case beat <= ev.beat+ev.length:
			loc = camLeft
		case ev.auto && beat <= ev.beat+ev.length*2:
			loc = camRight
		}
	}
	for _, ev := range m.passes {
		if ev.beat > beat {
			break
		}
		loc = camRight
	}
	for _, ev := range m.cameras {
		if ev.beat > beat {
			break
		}
		loc = ev.pos
	}
	m.resetCamera(loc, m.cameraXFor(loc))
}
