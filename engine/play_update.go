package engine

func (a *App) updatePlay() {
	t := a.cond.Time()
	beat := a.cond.Beat()

	a.advancePlayTimeline(beat)
	a.updateTimingArrow()
	a.updatePlayMusicVolume(beat)
	a.updateLatencyHotkeys()
	a.captureInputEdges()
	a.judgeRealtimeInputs(t, beat)
	a.judgeAutoplayInputs(t, beat)
	a.expireMissedInputs(t)

	if a.active != nil {
		a.active.Update(t, beat)
	}

	if beat > a.endBeat {
		a.enterResult()
	}
}
