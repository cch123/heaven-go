package engine

import "github.com/hajimehoshi/ebiten/v2"

// ScheduleInput 注册一次输入判定（HS Minigame.ScheduleInput 等价物）。
// onHit 的 state：just 窗归一化偏移，|state|<=1 = just，1<|state|<=2 = NG，负 = 早。
func (c *Ctx) ScheduleInput(beat float64, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInput(beat, false, 0, onHit, onMiss)
}

// ScheduleInputAction 注册指定动作通道的按下判定（0=主键，1=副键）。
func (c *Ctx) ScheduleInputAction(beat float64, action int, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInput(beat, false, action, onHit, onMiss)
}

// ScheduleInputActionCond registers a non-primary press window with an
// optional can-hit predicate. Multi-action games use this to keep their late
// windows from leaking across a switchGame boundary.
func (c *Ctx) ScheduleInputActionCond(beat float64, action int, canHit func() bool, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInputCond(beat, false, action, canHit, onHit, onMiss)
}

// ScheduleInputCond is ScheduleInput with HS' optional canJust predicate.
// Board Meeting uses this to invalidate a pending stop cue after the player's
// chair was already stopped by an early whiff.
func (c *Ctx) ScheduleInputCond(beat float64, canHit func() bool, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInputCond(beat, false, 0, canHit, onHit, onMiss)
}

// ScheduleInputNoScore registers a press window that prevents whiffs and runs
// callbacks without affecting score, timing display, or result rank.
func (c *Ctx) ScheduleInputNoScore(beat float64, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInputNoScore(beat, false, 0, onHit, onMiss)
}

// ScheduleInputRelease 注册一次"抬起"判定（InputAction_FlickRelease，
// totemClimb 高跳甩出等）。空抬不触发 Whiff。
func (c *Ctx) ScheduleInputRelease(beat float64, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInput(beat, true, 0, onHit, onMiss)
}

// ScheduleInputReleaseCond is the release-channel equivalent of
// ScheduleInputCond. Rhythm Tweezers needs this because a long hair release
// window is cancelled if the player lets go early and the game has already
// scored the miss from script-side hold polling.
func (c *Ctx) ScheduleInputReleaseCond(beat float64, canHit func() bool, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInputCond(beat, true, 0, canHit, onHit, onMiss)
}

// AutoHitRelease resolves a pending release input exactly on its target beat.
// HS' Rhythm Tweezers does this when the player is still holding a curly hair
// at the end of the pull, so "do not release" becomes a success instead of a
// missed release input.
func (c *Ctx) AutoHitRelease(beat float64) {
	c.App.judgePress(c.BeatToTime(beat), c.App.cond.Beat(), true, 0)
}

// PressedNow / ReleasedNow 报告本逻辑帧是否有按下/抬起（HoldCo 式轮询用）。
func (c *Ctx) PressedNow() bool  { return c.App.pressedNow }
func (c *Ctx) ReleasedNow() bool { return c.App.releasedNow }

// PressingNow 报告主键当前是否保持按下（HS InputAction_BasicPressing）。
func (c *Ctx) PressingNow() bool {
	return ebiten.IsKeyPressed(ebiten.KeySpace) ||
		ebiten.IsKeyPressed(ebiten.KeyJ) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
}

// ExpectingReleaseNow 报告当前时刻是否在某个未判定 release 输入的 NG 窗口内
// （IsExpectingInputNow(InputAction_FlickRelease) 等价物）。
func (c *Ctx) ExpectingReleaseNow() bool {
	t := c.App.cond.Time()
	for _, in := range c.App.inputs {
		if in.Release && !in.judged && t >= in.hitT-WinNG && t <= in.hitT+WinNG {
			return true
		}
	}
	return false
}

// ExpectingPressNow 报告当前时刻是否在某个未判定主键按下的 NG 窗口内
// （Glee Club 的空按闭嘴判定用）。
func (c *Ctx) ExpectingPressNow() bool {
	t := c.App.cond.Time()
	for _, in := range c.App.inputs {
		if !in.Release && in.Action == 0 && !in.judged && t >= in.hitT-WinNG && t <= in.hitT+WinNG {
			return true
		}
	}
	return false
}

// ScoreMiss 记一次 miss（HS Minigame.ScoreMiss：无对应判定窗的扣分，
// 如 totemClimb 高跳保持期提前松手）。
func (c *Ctx) ScoreMiss() {
	c.App.misses++
	c.App.recordMissScore(c.App.cond.Beat())
	c.App.setMsg("MISS...")
}
