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
	a.timingDisplayState.reset()
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
	a.setMinigamePitch(1)
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
	a.setMinigamePitch(1)
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
