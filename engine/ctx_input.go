package engine

// ScheduleInput 注册一次输入判定（HS Minigame.ScheduleInput 等价物）。
// onHit 的 state：just 窗归一化偏移，|state|<=1 = just，1<|state|<=2 = NG，负 = 早。
func (c *Ctx) ScheduleInput(beat float64, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInput(beat, false, 0, onHit, onMiss)
}

// ScheduleInputAction 注册指定动作通道的按下判定（0=主键，1=副键）。
func (c *Ctx) ScheduleInputAction(beat float64, action int, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInput(beat, false, action, onHit, onMiss)
}

// ScheduleInputAny registers a press window consumed by any press action.
// Dress Your Best's IA_PadAny accepts all D-pad/cardinal buttons as the same
// sewing input; one shared window avoids duplicate misses from four channels.
func (c *Ctx) ScheduleInputAny(beat float64, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInput(beat, false, -1, onHit, onMiss)
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

// ScheduleInputActionNoScore is the action-channel form of ScheduleInputNoScore.
// Working Dough registers the wrong button on the same beat as the correct
// button; that wrong-button window must suppress whiffs while leaving scoring
// entirely to the minigame script.
func (c *Ctx) ScheduleInputActionNoScore(beat float64, action int, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInputNoScore(beat, false, action, onHit, onMiss)
}

// ScheduleInputRelease 注册一次"抬起"判定（InputAction_FlickRelease，
// totemClimb 高跳甩出等）。空抬不触发 Whiff。
func (c *Ctx) ScheduleInputRelease(beat float64, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInput(beat, true, 0, onHit, onMiss)
}

// ScheduleInputActionRelease registers a release window for a non-primary
// action channel. Samurai Slice Ntr's AltUp uses this for the South-button
// unstep cue; empty releases intentionally do not whiff, matching base release.
func (c *Ctx) ScheduleInputActionRelease(beat float64, action int, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInput(beat, true, action, onHit, onMiss)
}

// ScheduleInputActionReleaseCond is the action-channel release variant used by
// games with a hold/release sub-action. Pajama Party's throw release should be
// ignored if the charge never started, matching CtrPillowPlayer.CanThrow.
func (c *Ctx) ScheduleInputActionReleaseCond(beat float64, action int, canHit func() bool, onHit func(state float64, j Judgment), onMiss func()) *Input {
	return c.App.scheduleInputCond(beat, true, action, canHit, onHit, onMiss)
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
