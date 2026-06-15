package engine

// StartBar stores finalScore*barDuration in Unity, so low scores reach rank
// faster than high scores instead of using a fixed gauge duration.
func (a *App) resultBarDoneTime() float64 {
	return resultBarStart + clamp01(a.result.Score)*resultBarDuration
}

func (a *App) resultRankTime() float64 {
	return a.resultBarDoneTime() + resultBarRankWait
}

func (a *App) resultRankMusicTime() float64 {
	return a.resultRankTime() + resultRankMusicWait
}
