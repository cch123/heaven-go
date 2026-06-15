package lumbearjack

import "math"

func (m *Module) scheduleCatPut(ev objectEvt) {
	effective := m.unit(ev)
	start := ev.beat - 0.5*effective
	m.ctx.At(start, func() {
		right := m.shouldBeRight(ev.beat, ev.cat)
		m.disableCatObjects(right)
		if right {
			m.ctx.Scene.PlayNormalized(m.catRight, "CatGrab", 0)
		} else {
			m.ctx.Scene.PlayNormalized(m.catLeft, "CatGrab", 0)
		}
	})
	m.ctx.At(ev.beat, func() {
		right := m.shouldBeRight(ev.beat, ev.cat)
		m.activateCatObject(ev, right)
	})
	m.ctx.At(ev.beat+effective, func() {
		m.disableCatObjects(m.shouldBeRight(ev.beat, ev.cat))
	})
}

func (m *Module) activateCatObject(ev objectEvt, right bool) {
	var arr []string
	idx := 0
	switch ev.kind {
	case objSmall:
		idx = int(ev.small)
		if right {
			arr = m.catRightSmall
		} else {
			arr = m.catLeftSmall
		}
	case objBig:
		idx = int(ev.big)
		if right {
			arr = m.catRightBig
		} else {
			arr = m.catLeftBig
		}
	default:
		idx = int(ev.huge)
		if right {
			arr = m.catRightHuge
		} else {
			arr = m.catLeftHuge
		}
	}
	if idx >= 0 && idx < len(arr) {
		m.ctx.Scene.SetActive(arr[idx], true)
	}
}

func (m *Module) disableCatObjects(right bool) {
	groups := [][]string{m.catRightSmall, m.catRightBig, m.catRightHuge}
	if !right {
		groups = [][]string{m.catLeftSmall, m.catLeftBig, m.catLeftHuge}
	}
	for _, g := range groups {
		for _, p := range g {
			m.ctx.Scene.SetActive(p, false)
		}
	}
}

func (m *Module) shouldBeRight(beat float64, cat catPutChoice) bool {
	presence := m.catPresenceAt(beat, true)
	switch cat {
	case catLeft:
		return presence == mainCatRight
	case catRight:
		return presence != mainCatLeft
	}
	right := m.startMainCat(beat) != mainCatLeft
	first := true
	for _, ev := range m.catPuts {
		if ev.beat > beat {
			break
		}
		switch ev.cat {
		case catAlternate:
			if !first {
				right = !right
			}
			if presence != mainCatBoth {
				right = presence != mainCatLeft
			}
		case catRight:
			right = presence != mainCatLeft
		case catLeft:
			right = presence == mainCatRight
		}
		first = false
	}
	return right
}

func (m *Module) startMainCat(beat float64) mainCatChoice {
	return m.catPresenceAt(beat, false)
}

func (m *Module) catPresenceAt(beat float64, inclusive bool) mainCatChoice {
	cat := mainCatRight
	for _, ev := range m.cats {
		if inclusive {
			if ev.beat > beat {
				break
			}
		} else if ev.beat >= beat {
			break
		}
		cat = ev.main
	}
	return cat
}

func (m *Module) bgPresenceAt(beat float64, inclusive bool) int {
	amount := 0
	for _, ev := range m.cats {
		if inclusive {
			if ev.beat > beat {
				break
			}
		} else if ev.beat >= beat {
			break
		}
		amount = ev.bg
	}
	return amount
}

func (m *Module) applyPresenceAt(beat float64) {
	before := m.catPresenceAt(beat, false)
	m.setMainCats(beat, 0, before, true)
	bg := m.bgPresenceAt(beat, false)
	m.setBgCats(beat, 0, bg, bg, true, true)
}

func (m *Module) applyCatPresence(ev catPresenceEvt, instantStart bool) {
	beforeMain := m.catPresenceAt(ev.beat, false)
	if beforeMain != ev.main {
		m.setMainCats(ev.beat, ev.length, ev.main, ev.instant || instantStart)
	}
	beforeBG := m.bgPresenceAt(ev.beat, false)
	m.setBgCats(ev.beat, ev.length, ev.bg, beforeBG, ev.instant, ev.dance)
}

func (m *Module) setMainCats(beat, length float64, main mainCatChoice, instant bool) {
	switch main {
	case mainCatRight:
		m.moveCat("Cat", beat, length, true, instant)
		m.moveCat("CatLeft", beat, length, false, instant)
	case mainCatLeft:
		m.moveCat("CatLeft", beat, length, true, instant)
		m.moveCat("Cat", beat, length, false, instant)
	case mainCatBoth:
		m.moveCat("Cat", beat, length, true, instant)
		m.moveCat("CatLeft", beat, length, true, instant)
	}
}

func (m *Module) moveCat(path string, beat, length float64, inToScene bool, instant bool) {
	spec, ok := m.catMoveSpecs[path]
	if !ok {
		m.ctx.Scene.SetActive(path, inToScene)
		return
	}
	if instant {
		length = 0
	}
	m.catMoves[path] = catMoveRuntime{spec: spec, startBeat: beat, length: length, inToScene: inToScene}
	m.ctx.Scene.SetActive(path, true)
	if length == 0 {
		pos := spec.other
		if inToScene {
			pos = spec.this
		}
		m.ctx.Scene.SetPosOver(path, pos[0], pos[1])
	}
}

func (m *Module) updateCatMoves(beat float64) {
	for path, mv := range m.catMoves {
		pos := mv.spec.this
		if mv.length <= 0 {
			if !mv.inToScene {
				pos = mv.spec.other
			}
		} else {
			t := clamp01((beat - mv.startBeat) / mv.length)
			from, to := mv.spec.this, mv.spec.other
			if mv.inToScene {
				from, to = mv.spec.other, mv.spec.this
			}
			pos[0] = from[0] + (to[0]-from[0])*t
			pos[1] = from[1] + (to[1]-from[1])*t
		}
		m.ctx.Scene.SetPosOver(path, pos[0], pos[1])
	}
}

func (m *Module) setBgCats(beat, length float64, bgCats, before int, instant, dance bool) {
	if bgCats < 0 {
		bgCats = 0
	}
	if bgCats > len(m.bgCats) {
		bgCats = len(m.bgCats)
	}
	bg := bgCats - 1
	before--
	for i, path := range m.bgCats {
		in := bg >= i
		moveInstant := instant
		if bg < before {
			moveInstant = instant || !(i > bg && i <= before)
		} else if bg > before {
			moveInstant = instant || !(i > before && i <= bg)
		} else {
			moveInstant = true
		}
		m.moveCat(path, beat, length, in, moveInstant)
		overflow := math.Mod(beat, 2)
		toBeat := 2 - overflow
		danceBeat := beat + toBeat - 0.5
		stopBeat := beat + toBeat - 0.5
		if instant || (bg >= before && i <= before) || (bg < before && i <= bg) {
			danceBeat = beat - overflow - 0.5
		}
		if !in {
			danceBeat = math.Inf(1)
		}
		if dance {
			stopBeat = math.Inf(1)
		}
		m.bgCatDance[path] = catDanceRuntime{danceBeat: danceBeat, stopBeat: stopBeat}
	}
}

func (m *Module) updateBgCatDance(beat float64) {
	if m.bgCatDance == nil {
		m.bgCatDance = map[string]catDanceRuntime{}
	}
	for _, path := range m.bgCats {
		anim := path + "/CatHolder"
		rt := m.bgCatDance[path]
		switch {
		case beat >= rt.stopBeat:
			m.ctx.Scene.PlayNormalized(anim, "CatDance", 0)
		case rt.danceBeat > beat:
			m.ctx.Scene.PlayNormalized(anim, "CatIdle", 0)
		default:
			n := math.Mod((beat-rt.danceBeat)/2, 1)
			m.ctx.Scene.PlayNormalized(anim, "CatDance", n)
		}
	}
}
