package engine

func (a *App) setMsg(s string) {
	a.lastMsg = s
	a.msgT = a.cond.Time()
}

func (a *App) pushTiming(signed float64, j Judgment) {
	a.timingDisplayState.push(signed, j, a.cond.Time())
}
