package engine

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (a *App) updateLatencyHotkeys() {
	if inpututil.IsKeyJustPressed(ebiten.KeyLeftBracket) {
		a.LatencyMS -= 5
		a.setMsg(fmt.Sprintf("latency %+.0fms", a.LatencyMS))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRightBracket) {
		a.LatencyMS += 5
		a.setMsg(fmt.Sprintf("latency %+.0fms", a.LatencyMS))
	}
}

func (a *App) captureInputEdges() {
	a.pressedNow, a.releasedNow = pressed(), released()
}

func (a *App) judgeRealtimeInputs(t, beat float64) {
	if !a.inputOn {
		return
	}
	adjustedT := t - a.LatencyMS/1000
	if a.pressedNow {
		a.judgePress(adjustedT, beat, false, 0)
	}
	for act := 1; act <= 3; act++ {
		if pressedN(act) {
			a.judgePress(adjustedT, beat, false, act)
		}
	}
	if a.releasedNow {
		a.judgePress(adjustedT, beat, true, 0)
	}
	for act := 1; act <= 3; act++ {
		if releasedN(act) {
			a.judgePress(adjustedT, beat, true, act)
		}
	}
}

func (a *App) judgeAutoplayInputs(t, beat float64) {
	if !a.Autoplay {
		return
	}
	for _, in := range a.inputs {
		if !in.judged && !in.NoAutoplay && t >= in.hitT {
			a.judgePress(in.hitT, beat, in.Release, in.Action)
		}
	}
}

func (a *App) expireMissedInputs(t float64) {
	for _, in := range a.inputs {
		if in.judged || t <= in.hitT+WinNG {
			continue
		}
		if in.CanHit != nil && !in.CanHit() {
			// 有些游戏会在输入窗口结束前禁用目标；原版会把这类输入消费掉，
			// 但不触发 miss 分数或 miss 回调。
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
