// Package conductor 是 Heaven Studio Conductor.cs 核心逻辑的 Go 移植：
// 以音频播放位置为权威时钟（采样时钟），用单调时钟（monotonic clock）
// 做帧间外推 + 漂移校正，得到平滑且与音频锁定的歌曲时间。
//
// 原版做法（Conductor.cs）：absTime 每帧累加 deltaTime，
// 周期性与 AudioSettings.dspTime 比对并校正。这里等价地用
// time.Now()（Go 的单调时钟）外推、与 audio.Player 的播放位置比对。
//
//	               +------------------+
//	time.Now() --> | 外推: pos += dt  |--+
//	               +------------------+  |   |pos-real| > snap ?
//	                                     v
//	               +------------------+  是 -> pos = real（跳变重锚）
//	player.    --> | 漂移校正         |
//	Position()     +------------------+  否 -> pos -= (pos-real)*k（缓收敛）
package conductor

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/riq"
)

const (
	snapThreshold = 0.05 // 秒：外推值与音频时钟偏差超过此值则直接重锚
	driftGain     = 0.08 // 缓校正系数，每帧消除 8% 偏差，避免时间轴抖动
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
	clock    func() float64
}

func New(bm *riq.Beatmap, player *audio.Player) *Conductor {
	return &Conductor{bm: bm, player: player, pitch: 1}
}

// SetClock overrides the audio-position clock used for drift correction. Chart
// music with runtime pitch resampling reports source time here; audio.Player
// itself reports output time and would erase minigame slowdowns after restore.
func (c *Conductor) SetClock(clock func() float64) {
	c.clock = clock
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
	c.pitch = pitch
}

// Play 启动音乐与时钟。
func (c *Conductor) Play() {
	c.player.Play()
	c.lastTick = time.Now()
	c.playing = true
}

// Pause 暂停音乐与时钟。
func (c *Conductor) Pause() {
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
	c.lastTick = time.Now()
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
	now := time.Now()
	dt := now.Sub(c.lastTick).Seconds()
	c.lastTick = now

	c.pos += dt * c.pitch

	// 音频播完后 Position() 冻结：改纯单调时钟推进，否则漂移校正会把
	// 时间拽住，谱面尾部（音频结束之后的 end 事件）永远到不了
	real := c.realPosition()
	if !c.player.IsPlaying() && c.pos >= real {
		return
	}

	c.drift = c.pos - real
	if abs(c.drift) > snapThreshold {
		c.pos = real
	} else {
		c.pos -= c.drift * driftGain
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
	if c.clock != nil {
		return c.clock()
	}
	if c.player == nil {
		return c.pos
	}
	return c.player.Position().Seconds()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
