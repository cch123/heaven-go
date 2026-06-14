package engine

import "sort"

func (a *App) setActive(id string, beat float64) {
	if m, ok := a.modules[id]; ok {
		a.active = m
		m.OnSwitch(beat)
	}
}

func (a *App) resetRunState() {
	a.swIdx = 0
	a.inputOn = true
	a.starGot = false
	a.aces, a.justs, a.ngs, a.misses, a.whiffs = 0, 0, 0, 0, 0
	a.lastMsg, a.msgT = "", 0
	a.tdArrow, a.tdTarget, a.tdHits = 0, 0, nil
	a.scores = nil
	a.result, a.resultT, a.resultEpilogue = resultSummary{}, 0, false
	a.stopResultAudio()
}

// SeekBeat jumps an already loaded chart to beat. It is primarily used by
// verification tooling for long remixes: past timeline actions are skipped,
// past inputs are marked consumed, and the active minigame is restored as if
// the chart had naturally switched to the containing segment.
func (a *App) SeekBeat(beat float64) error {
	if a.bm == nil || a.cond == nil {
		return nil
	}
	if err := a.cond.SeekBeat(beat); err != nil {
		return err
	}
	a.actIdx = sort.Search(len(a.actions), func(i int) bool { return a.actions[i].beat > beat })
	pos := a.bm.BeatToTime(beat)
	for _, in := range a.inputs {
		if in.hitT < pos {
			in.judged = true
		}
	}
	a.active = nil
	a.swIdx = 0
	for a.swIdx < len(a.switches) && a.switches[a.swIdx].beat <= beat {
		if m, ok := a.modules[a.switches[a.swIdx].id]; ok {
			a.active = m
		}
		a.swIdx++
	}
	if a.active != nil {
		a.active.OnSwitch(beat)
	}
	return nil
}

// restart 重置一轮游玩（不重载资产）。
func (a *App) restart() error {
	if err := a.cond.Reset(); err != nil {
		return err
	}
	for _, in := range a.inputs {
		in.judged = false
		in.Result = JudgeNone
	}
	a.actIdx = 0
	a.resetRunState()
	if len(a.switches) > 0 {
		a.setActive(a.switches[0].id, 0)
		a.swIdx = 1
	}
	a.state = stateTitle
	return nil
}

// returnToLevelSelect unloads the active chart so stateTitle falls through to
// the Library selector instead of the current chart's title card.
func (a *App) returnToLevelSelect() {
	if a.player != nil {
		a.player.Close()
		a.player = nil
	}
	a.r, a.bm, a.cond = nil, nil, nil
	a.modules = nil
	a.active = nil
	a.switches = nil
	a.actions = nil
	a.inputs = nil
	a.flashes = nil
	a.camEvts = nil
	a.musicFades = nil
	a.viewScales = nil
	a.viewBuf = nil
	a.unported = nil
	a.actIdx = 0
	a.starBeat, a.endBeat = -1, 0
	a.fx.reset()
	a.flt.reset()
	a.tbx.reset()
	a.resetRunState()
	a.loadErr = ""
	a.state = stateTitle
	a.keepMenuSelectionVisible()
}
