package engine

func (a *App) enterResult() {
	a.cond.Pause()
	a.result = a.buildResultSummary()
	a.resultT = 0
	a.resultEpilogue = false
	a.resetResultAudioCues()
	a.state = stateResult
}
