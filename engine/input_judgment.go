package engine

import "math"

// judgePress 判定一次按下（release=false）或抬起（release=true）。
// 抬起只匹配 Release 输入，且空抬不计 whiff（HS 的 flick release whiff
// 由游戏侧自行处理，如 totemClimb HoldCo）。
func (a *App) judgePress(t, beat float64, release bool, action int) {
	var best *Input
	bestDiff := math.Inf(1)
	for _, in := range a.inputs {
		if in.judged || in.Release != release || (in.Action != action && in.Action != -1) {
			continue
		}
		if in.CanHit != nil && !in.CanHit() {
			continue
		}
		if d := math.Abs(t - in.hitT); d < bestDiff {
			best, bestDiff = in, d
		}
	}
	if best == nil || bestDiff > WinNG {
		if release {
			return
		}
		a.whiffs++
		a.setMsg("...")
		if a.active != nil {
			if aw, ok := a.active.(ActionWhiffer); ok {
				aw.WhiffAction(beat, action)
			} else if action == 0 {
				a.active.Whiff(beat)
			}
		}
		return
	}

	best.judged = true
	signed := t - best.hitT
	state := signed / WinJust // |state|<=1 = just，与 C# stateProg 同语义
	var j Judgment
	switch d := math.Abs(signed); {
	case d <= WinAce:
		j = JudgeAce
		if !best.NoScore {
			a.aces++
			a.setMsg("ACE!!")
		}
	case d <= WinJust:
		j = JudgeJust
		if !best.NoScore {
			a.justs++
			a.setMsg("OK!")
		}
	default:
		j = JudgeNG
		if !best.NoScore {
			a.ngs++
			a.setMsg("NG")
		}
	}
	best.Result = j
	if !best.NoScore {
		a.recordInputScore(best, accuracyForDiff(math.Abs(signed)))
		if j == JudgeAce && a.starBeat >= 0 && !a.starGot && math.Abs(best.Beat-a.starBeat) < 0.25 {
			a.starGot = true
			a.setMsg("SKILL STAR!")
		}
		a.pushTiming(signed, j)
	}
	if best.OnHit != nil {
		best.OnHit(state, j)
	}
}

func accuracyForDiff(d float64) float64 {
	switch {
	case d <= WinAce:
		return 1
	case d <= WinJust:
		u := (d - WinAce) / (WinJust - WinAce)
		return rankHiThreshold + (1-u)*(1-rankHiThreshold)
	case d <= WinNG:
		u := (d - WinJust) / (WinNG - WinJust)
		return (1 - u) * rankOkThreshold
	default:
		return 0
	}
}
