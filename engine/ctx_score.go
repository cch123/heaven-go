package engine

// ScoreMiss 记一次 miss（HS Minigame.ScoreMiss：无对应判定窗的扣分，
// 如 totemClimb 高跳保持期提前松手）。
func (c *Ctx) ScoreMiss() {
	c.App.misses++
	c.App.recordMissScore(c.App.cond.Beat())
	c.App.setMsg("MISS...")
}
