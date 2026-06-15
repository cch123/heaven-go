package engine

import "github.com/hajimehoshi/ebiten/v2"

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
