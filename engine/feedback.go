package engine

import "math"

func (a *App) setMsg(s string) {
	a.lastMsg = s
	a.msgT = a.cond.Time()
}

func (a *App) pushTiming(signed float64, j Judgment) {
	signed = math.Max(-WinNG, math.Min(WinNG, signed))
	y := timingBarNorm(signed)
	a.tdTarget = (a.tdTarget + y) * 0.5
	a.tdHits = append(a.tdHits, timingHit{y: y, signed: signed, rating: j, t: a.cond.Time()})
}
