package engine

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (a *App) updatePlay() {
	t := a.cond.Time()
	beat := a.cond.Beat()

	// 游戏切换
	for a.swIdx < len(a.switches) && a.switches[a.swIdx].beat <= beat {
		a.setActive(a.switches[a.swIdx].id, a.switches[a.swIdx].beat)
		a.swIdx++
	}
	// 时间轴动作
	for a.actIdx < len(a.actions) && a.actions[a.actIdx].beat <= beat {
		a.actions[a.actIdx].fn()
		a.actIdx++
	}
	// 时机条箭头
	dt := 1.0 / float64(ebiten.TPS())
	a.tdArrow += (a.tdTarget - a.tdArrow) * math.Min(4*dt, 1)

	// 音量时间轴（riq__VolumeChange）再乘游戏局部 ducking
	// （Tunnel/FadeMinigameVolume 等不改写谱面 volume 事件）。
	a.player.SetVolume(a.bm.VolumeAt(beat) * a.MusicFadeAt(beat))

	// 延迟校准热键
	if inpututil.IsKeyJustPressed(ebiten.KeyLeftBracket) {
		a.LatencyMS -= 5
		a.setMsg(fmt.Sprintf("latency %+.0fms", a.LatencyMS))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRightBracket) {
		a.LatencyMS += 5
		a.setMsg(fmt.Sprintf("latency %+.0fms", a.LatencyMS))
	}

	a.pressedNow, a.releasedNow = pressed(), released()
	if a.pressedNow && a.inputOn {
		a.judgePress(t-a.LatencyMS/1000, beat, false, 0)
	}
	for act := 1; act <= 3; act++ {
		if pressedN(act) && a.inputOn {
			a.judgePress(t-a.LatencyMS/1000, beat, false, act)
		}
	}
	if a.releasedNow && a.inputOn {
		a.judgePress(t-a.LatencyMS/1000, beat, true, 0)
	}
	if a.Autoplay {
		for _, in := range a.inputs {
			if !in.judged && !in.NoAutoplay && t >= in.hitT {
				a.judgePress(in.hitT, beat, in.Release, in.Action)
			}
		}
	}

	// 超窗 miss
	for _, in := range a.inputs {
		if !in.judged && t > in.hitT+WinNG {
			if in.CanHit != nil && !in.CanHit() {
				in.judged = true
				continue
			}
			in.judged = true
			in.Result = JudgeMiss
			if !in.NoScore {
				a.recordInputScore(in, 0)
				a.misses++
				a.setMsg("MISS...")
			}
			if in.OnMiss != nil {
				in.OnMiss()
			}
		}
	}

	if a.active != nil {
		a.active.Update(t, beat)
	}

	if beat > a.endBeat {
		a.enterResult()
	}
}

// judgePress 判定一次按下（release=false）或抬起（release=true）。
// 抬起只匹配 Release 输入，且空抬不计 whiff（HS 的 flick release whiff
// 由游戏侧自行处理，如 totemClimb HoldCo）。
func (a *App) judgePress(t, beat float64, release bool, action int) {
	var best *Input
	bestDiff := math.Inf(1)
	for _, in := range a.inputs {
		if in.judged || in.Release != release || in.Action != action {
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
