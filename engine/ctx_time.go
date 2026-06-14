package engine

import (
	"math"

	"hsdemo/riq"
)

// Time / Beat 返回当前歌曲时间与节拍。
func (c *Ctx) Time() float64 { return c.App.cond.Time() }
func (c *Ctx) Beat() float64 { return c.App.cond.Beat() }

// BeatToTime 经 tempo map 换算。
func (c *Ctx) BeatToTime(beat float64) float64 { return c.App.bm.BeatToTime(beat) }

// TimeToBeat 是 BeatToTime 的逆映射（HS Conductor.GetBeatFromSongPos）。
func (c *Ctx) TimeToBeat(t float64) float64 { return c.App.bm.TimeToBeat(t) }

// SecPerBeat 返回某拍处的秒/拍（用于以 1x 真实速度播放剪辑：timeScale = SecPerBeat）。
func (c *Ctx) SecPerBeat(beat float64) float64 { return 60 / c.App.bm.BPMAt(beat) }

// GameAt 返回 beat 时刻的活动 minigame id（switchGame 时间轴；
// 模块用它复刻"游戏未激活时事件只出声不生成判定"的 inactiveFunction 语义）。
func (c *Ctx) GameAt(beat float64) string {
	id := ""
	for _, sw := range c.App.switches {
		if sw.beat > beat {
			break
		}
		id = sw.id
	}
	return id
}

// Entities 返回谱面实体（cheerReaders 的 Automatic 仲裁等需要重查参数）。
func (c *Ctx) Entities() []riq.Entity { return c.App.bm.Entities }

// NextSwitchBeat 返回 beat 之后下一次 switchGame/end 的拍（无则 +Inf；
// Lockstep.QueueSwitches 的 nextGameSwitchBeat 语义）。
func (c *Ctx) NextSwitchBeat(beat float64) float64 {
	next := math.Inf(1)
	for _, sw := range c.App.switches {
		if sw.beat > beat {
			next = sw.beat
			break
		}
	}
	if c.App.endBeat > beat && c.App.endBeat < next {
		next = c.App.endBeat
	}
	return next
}
