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
	for act := 1; act <= 3; act++ {
		if releasedN(act) && a.inputOn {
			a.judgePress(t-a.LatencyMS/1000, beat, true, act)
		}
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
