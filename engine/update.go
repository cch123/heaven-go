package engine

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"hsdemo/riq"
)

// ---------- Update ----------

func (a *App) Update() error {
	if HandleFullscreenShortcut() {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		a.debug = !a.debug
	}
	a.pollDroppedRiq()

	switch a.state {
	case stateTitle:
		if a.bm == nil {
			a.updateLevelSelect()
			return nil
		}
		if a.bm != nil && (titlePressed() || a.Autoplay) {
			a.cond.Play()
			a.state = statePlay
		}
	case statePlay:
		a.cond.Update()
		a.updatePlay()
	case stateResult:
		a.resultT += 1 / float64(ebiten.TPS())
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			a.stopResultAudio()
			return a.restart()
		}
		if titlePressed() {
			if !a.resultEpilogue {
				if a.resultT < resultRankTime {
					a.resultT = resultRankTime
					a.skipResultAudioToRank()
				} else {
					a.enterResultEpilogue()
				}
			} else if a.resultT > 1.5 {
				a.returnToLevelSelect()
				return nil
			}
		}
		a.updateResultAudio()
	}
	return nil
}

func (a *App) updateLevelSelect() {
	if len(a.levels) == 0 {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) ||
		inpututil.IsKeyJustPressed(ebiten.KeyUp) ||
		inpututil.IsKeyJustPressed(ebiten.KeyW) {
		a.moveMenu(-menuGridCols)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeyDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeyS) {
		a.moveMenu(menuGridCols)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyA) {
		a.moveMenu(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyD) {
		a.moveMenu(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		a.moveMenu(-menuVisibleItems)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
		a.moveMenu(menuVisibleItems)
	}
	_, wheelY := ebiten.Wheel()
	if wheelY > 0 {
		a.moveMenu(-menuGridCols)
	} else if wheelY < 0 {
		a.moveMenu(menuGridCols)
	}
	if idx, ok := a.hoveredMenuLevel(); ok {
		a.menuSel = idx
		a.keepMenuSelectionVisible()
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			a.loadSelectedLevel()
			return
		}
	}
	if menuConfirmPressed() {
		a.loadSelectedLevel()
	}
}

func menuConfirmPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyJ) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
}

func titlePressed() bool {
	return pressed() ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
}

func (a *App) moveMenu(delta int) {
	a.menuSel += delta
	if a.menuSel < 0 {
		a.menuSel = 0
	}
	if a.menuSel >= len(a.levels) {
		a.menuSel = len(a.levels) - 1
	}
	a.keepMenuSelectionVisible()
}

func (a *App) keepMenuSelectionVisible() {
	if a.menuSel < a.menuScroll {
		a.menuScroll = a.menuSel
	}
	if a.menuSel >= a.menuScroll+menuVisibleItems {
		a.menuScroll = a.menuSel - menuVisibleItems + 1
	}
	maxScroll := len(a.levels) - menuVisibleItems
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.menuScroll > maxScroll {
		a.menuScroll = maxScroll
	}
	if a.menuScroll < 0 {
		a.menuScroll = 0
	}
}

func (a *App) hoveredMenuLevel() (int, bool) {
	x, y := ebiten.CursorPosition()
	if x < menuGridX || x >= menuGridX+menuGridCols*(menuCardW+menuCardGapX)-menuCardGapX {
		return 0, false
	}
	if y < menuGridY || y >= menuGridY+menuGridRows*(menuCardH+menuCardGapY)-menuCardGapY {
		return 0, false
	}
	colStep := menuCardW + menuCardGapX
	rowStep := menuCardH + menuCardGapY
	col := (x - menuGridX) / colStep
	row := (y - menuGridY) / rowStep
	if x >= menuGridX+col*colStep+menuCardW || y >= menuGridY+row*rowStep+menuCardH {
		return 0, false
	}
	idx := a.menuScroll + row*menuGridCols + col
	if idx < 0 || idx >= len(a.levels) {
		return 0, false
	}
	return idx, true
}

func (a *App) loadSelectedLevel() {
	if a.menuSel < 0 || a.menuSel >= len(a.levels) {
		return
	}
	level := a.levels[a.menuSel]
	r, err := riq.Load(level.path)
	if err != nil {
		a.loadErr = fmt.Sprintf("read %s failed: %v", level.displayName(), err)
		return
	}
	if err := a.loadRiq(r); err != nil {
		a.loadErr = fmt.Sprintf("load %s failed: %v", level.displayName(), err)
		return
	}
	a.loadErr = ""
	a.state = stateTitle
}

func pressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

// pressedN：动作通道 1=左（F/←/↑）、2=右（K/→）、3=替代（L/↓/X）。
func pressedN(action int) bool {
	switch action {
	case 1:
		return inpututil.IsKeyJustPressed(ebiten.KeyF) ||
			inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
			inpututil.IsKeyJustPressed(ebiten.KeyUp)
	case 2:
		return inpututil.IsKeyJustPressed(ebiten.KeyK) ||
			inpututil.IsKeyJustPressed(ebiten.KeyRight)
	case 3:
		return inpututil.IsKeyJustPressed(ebiten.KeyL) ||
			inpututil.IsKeyJustPressed(ebiten.KeyDown) ||
			inpututil.IsKeyJustPressed(ebiten.KeyX)
	}
	return false
}

func released() bool {
	return inpututil.IsKeyJustReleased(ebiten.KeySpace) ||
		inpututil.IsKeyJustReleased(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
}

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
			if !in.judged && t >= in.hitT {
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
