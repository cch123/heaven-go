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
	a.unloadLoadedRiq()
	a.state = stateTitle
	a.keepMenuSelectionVisible()
}
