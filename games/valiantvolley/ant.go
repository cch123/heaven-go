package valiantvolley

import "math"

type ant struct {
	path           string
	num            int
	player         bool
	cantBop        bool
	justHit        bool
	isPreparing    bool
	queuePrepare   float64
	lastVolleyBeat float64
}

func (a *ant) reset() {
	a.cantBop = false
	a.justHit = false
	a.isPreparing = false
	a.queuePrepare = math.Inf(1)
	a.lastVolleyBeat = math.Inf(-1)
}

func (a *ant) update(m *Module, beat float64) {
	if beat >= a.queuePrepare {
		if !a.isPreparing && m.ctx.Scene.Current(a.path) != "Animations/Volley" {
			m.ctx.Scene.PlayState(a.path, "AntPrepare", a.queuePrepare, 0.5)
			a.isPreparing = true
		}
		a.queuePrepare = math.Inf(1)
	}
}

func (a *ant) requestBop(m *Module, beat float64) {
	cur := m.ctx.Scene.Current(a.path)
	if cur == "Animations/AntBop" || cur == "Animations/AntHappy" || cur == "Animations/AntAngry" || cur == "Animations/AntOops" {
		return
	}
	if !a.isPreparing && !a.justHit && !a.cantBop {
		a.playAnimation(m, m.bopStatus, beat)
		return
	}
	if m.bopStatus == bopAngry && m.ants[2] != nil {
		m.ants[2].cantBop = false
		m.ants[2].isPreparing = false
		m.ants[2].justHit = false
	}
}

func (a *ant) playAnimation(m *Module, which int, beat float64) {
	if which == bopAngry && a.player {
		which = bopOops
	}
	state := "AntBop"
	switch which {
	case bopHappy:
		state = "AntHappy"
	case bopAngry:
		state = "AntAngry"
	case bopOops:
		state = "AntOops"
	}
	m.ctx.Scene.PlayState(a.path, state, beat, 0.5)
	a.justHit = false
	if a.player && (which == bopHappy || which == bopOops) {
		m.bopStatus = bopNormal
	}
}

func (a *ant) action(m *Module, beat float64, action string, pitch float64) {
	if beat-a.lastVolleyBeat > 0.15 {
		if a.num != 2 ||
			(action == "dirtHit" && m.expecting(objDirt, beat)) ||
			(action == "fruitHit" && m.expecting(objFruit, beat)) {
			a.justHit = true
		}
	}
	if action == "dirtHit" || action == "fruitHit" {
		a.lastVolleyBeat = beat
	}
	m.ctx.SoundPitch("woosh", 1, pitch)
	m.ctx.Scene.PlayState(a.path, "Volley", beat, 0.5)
	a.isPreparing = false
	a.queuePrepare = math.Inf(1)
}

func (m *Module) expecting(typ int, beat float64) bool {
	for _, o := range m.objects {
		if o.typ == typ && !o.dead && !o.missed && o.expectsInputAt(beat) {
			return true
		}
	}
	return false
}
