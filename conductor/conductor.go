// Package conductor 是 Heaven Studio Conductor.cs 核心逻辑的 Go 移植：
// 以单调时钟（monotonic clock）推进歌曲时间，用音频播放位置做粗同步锚。
// Go/Ebitengine 暴露的 audio.Player.Position() 会按后端缓冲块跳动；如果每帧
// 跟随它，缓冲读数的台阶会直接变成所有游戏的动画抖动。
//
// 原版做法（Conductor.cs）：absTime 每帧累加 deltaTime，
// 周期性与 AudioSettings.dspTime 比对并校正。这里等价地用
// time.Now()（Go 的单调时钟）外推、只在与 audio.Player 的播放位置偏差过大
// 时才修正。
//
//	               +------------------+     |pos-real| > syncMargin ?
//	time.Now() --> | 外推: pos += dt  |--否---------------------------+
//	               +------------------+                              |
//	                         | 是                                     v
//	                         v                         继续使用线性 pos，避免缓冲块抖动
//	               +------------------+
//	player.    --> | 粗同步修正       |
//	Position()     +------------------+
package conductor

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/riq"
)

const (
	syncMargin        = 0.12 // 秒：Ebitengine 的 Position 可能按缓冲块跳动；低于该差值不校正
	maxSyncIterations = 8    // 与 Conductor.cs 一样，单帧最多做 8 次半步收敛
)

// Conductor 维护歌曲位置并提供节拍换算。
type Conductor struct {
	bm     *riq.Beatmap
	player *audio.Player

	pos      float64 // 平滑后的歌曲时间（秒）
	lastTick time.Time
	playing  bool
	drift    float64 // 最近一次外推值与音频时钟的偏差（诊断用）
	pitch    float64
	clock    func() float64 // 测试用：覆盖 audio.Player.Position 的输出时间
	now      func() time.Time

	clockOutBase  float64 // 上一次 pitch 变更时的音频输出时间
	clockSongBase float64 // 上一次 pitch 变更时对应的歌曲源时间
}

func New(bm *riq.Beatmap, player *audio.Player) *Conductor {
	return &Conductor{bm: bm, player: player, pitch: 1}
}

// SetClock overrides the output-position clock used for drift correction. The
// clock must advance in actually audible time, not decoder/read-ahead source
// time; otherwise buffer prefetch jumps become visible animation jitter.
func (c *Conductor) SetClock(clock func() float64) {
	c.clock = clock
	c.rebaseOutputClock()
}

// SetMinigamePitch changes how quickly song position advances, equivalent to
// Heaven Studio's minigamePitch multiplier. The audio reader is controlled by
// engine.App; this method only owns the conductor time mapping.
func (c *Conductor) SetMinigamePitch(pitch float64) {
	if math.IsNaN(pitch) || math.IsInf(pitch, 0) || pitch <= 0 {
		pitch = 1
	} else if pitch < 0.01 {
		pitch = 0.01
	}
	c.rebaseOutputClock()
	c.pitch = pitch
}

// Play 启动音乐与时钟。
func (c *Conductor) Play() {
	c.player.Play()
	c.rebaseOutputClock()
	c.lastTick = c.nowTime()
	c.playing = true
}

// Pause 暂停音乐与时钟。
func (c *Conductor) Pause() {
	c.rebaseOutputClock()
	c.player.Pause()
	c.playing = false
}

// Reset 停止播放并把位置归零（用于重开）。
func (c *Conductor) Reset() error {
	c.player.Pause()
	c.playing = false
	c.pos = 0
	c.drift = 0
	c.pitch = 1
	c.clockOutBase = 0
	c.clockSongBase = 0
	return c.player.SetPosition(0)
}

// SeekTime moves the audio clock to an absolute song time. It preserves the
// playing state so verification tools can jump into long remixes without
// invalidating the conductor's monotonic-clock extrapolation.
func (c *Conductor) SeekTime(pos float64) error {
	if pos < 0 {
		pos = 0
	}
	wasPlaying := c.playing
	c.player.Pause()
	c.playing = false
	if err := c.player.SetPosition(time.Duration(pos * float64(time.Second))); err != nil {
		return err
	}
	c.pos = pos
	c.drift = 0
	c.clockOutBase = c.outputPosition()
	c.clockSongBase = pos
	c.lastTick = c.nowTime()
	if wasPlaying {
		c.player.Play()
		c.playing = true
	}
	return nil
}

// SeekBeat moves to a beat through the chart tempo map.
func (c *Conductor) SeekBeat(beat float64) error {
	return c.SeekTime(c.bm.BeatToTime(beat))
}

// Update 每帧调用一次，推进并校正歌曲时间。
func (c *Conductor) Update() {
	if !c.playing {
		return
	}
	now := c.nowTime()
	dt := now.Sub(c.lastTick).Seconds()
	c.lastTick = now

	c.pos += dt * c.pitch

	// 音频播完后 Position() 冻结：改纯单调时钟推进，否则漂移校正会把
	// 时间拽住，谱面尾部（音频结束之后的 end 事件）永远到不了
	real := c.realPosition()
	if !c.outputPlaying() && c.pos >= real {
		return
	}

	c.drift = c.pos - real
	if abs(c.drift) <= syncMargin {
		return
	}

	// Unity Conductor.cs uses DateTime for the visible song clock and only
	// adjusts absTimeAdjust when it drifts outside the DSP-buffer margin. We
	// mirror that "deadband then converge" behavior here. The deadband is
	// deliberately wider than Unity's because audio.Player.Position is a
	// coarse backend clock, not a high-resolution dspTime replacement.
	for i := 0; i < maxSyncIterations && abs(c.drift) > syncMargin; i++ {
		c.pos -= c.drift * 0.5
		c.drift = c.pos - real
	}
}

// Time 返回当前歌曲时间（秒）。
func (c *Conductor) Time() float64 { return c.pos }

// Beat 返回当前节拍（经 tempo map 换算）。
func (c *Conductor) Beat() float64 { return c.bm.TimeToBeat(c.pos) }

// Drift 返回外推时钟与音频时钟的瞬时偏差（秒），调试叠层用。
func (c *Conductor) Drift() float64 { return c.drift }

// Playing 报告时钟是否在走。
func (c *Conductor) Playing() bool { return c.playing }

func (c *Conductor) realPosition() float64 {
	return c.clockSongBase + (c.outputPosition()-c.clockOutBase)*c.pitch
}

func (c *Conductor) outputPosition() float64 {
	if c.clock != nil {
		return c.clock()
	}
	if c.player == nil {
		return c.pos
	}
	return c.player.Position().Seconds()
}

func (c *Conductor) outputPlaying() bool {
	if c.player == nil {
		return c.clock != nil && c.playing
	}
	return c.player.IsPlaying()
}

func (c *Conductor) rebaseOutputClock() {
	c.clockSongBase = c.pos
	c.clockOutBase = c.outputPosition()
}

func (c *Conductor) nowTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
