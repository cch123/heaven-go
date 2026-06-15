package engine

func (a *App) startPlay() {
	a.cond.Play()
	a.state = statePlay
}

func (a *App) enterResult() {
	a.cond.Pause()
	a.result = a.buildResultSummary()
	a.resultT = 0
	a.resultEpilogue = false
	a.resetResultAudioCues()
	a.state = stateResult
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
