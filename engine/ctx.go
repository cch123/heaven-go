// ctx.go：模块对引擎的访问句柄（HS 中 Minigame 基类提供的服务）。
package engine

import (
	"bytes"
	"math"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/kart"
	"hsdemo/riq"
)

// Ctx 是模块与引擎之间的接口：资产、场景、时间轴调度与输入注册。
type Ctx struct {
	App    *App
	Assets *kart.Assets
	Scene  *kart.SceneInst

	module Module
}

// LoadAssets 加载模块资产目录（assetsRoot/<id>）并创建场景实例。
func (c *Ctx) LoadAssets(id string) error {
	as, err := kart.Load(filepath.Join(c.App.assetsRoot, id), SampleRate)
	if err != nil {
		return err
	}
	c.Assets = as
	c.Scene = kart.NewScene(as)
	return nil
}

// Role 取脚本字段绑定的节点 path（roles.json）。
func (c *Ctx) Role(field string) string { return c.Assets.Roles[field] }

// Play 在子树上播放剪辑（DoScaledAnimationAsync 语义）。
func (c *Ctx) Play(rolePath, clip string, startBeat, timeScale float64) {
	c.Scene.Play(rolePath, clip, startBeat, timeScale)
}

// At 在指定拍执行回调（BeatAction 等价物），载入期与运行期均可调用。
func (c *Ctx) At(beat float64, fn func()) { c.App.at(beat, fn) }

// Sound 立即播放音效。
func (c *Ctx) Sound(name string) { c.SoundVol(name, 1) }

// SoundVol 以指定音量立即播放音效。
func (c *Ctx) SoundVol(name string, vol float64) {
	pcm, ok := c.Assets.Sounds[name]
	if !ok {
		return
	}
	p := audioCtx.NewPlayerFromBytes(pcm)
	p.SetVolume(vol)
	p.Play()
}

// SoundPitch 以指定音量与音高播放音效（SoundByte pitch 语义）。
func (c *Ctx) SoundPitch(name string, vol, pitch float64) {
	c.SoundPitchPan(name, vol, pitch, 0)
}

// SoundPitchPan 以指定音量、音高和左右声像播放音效（MultiSound panning）。
func (c *Ctx) SoundPitchPan(name string, vol, pitch, pan float64) {
	pcm, ok := c.Assets.Sounds[name]
	if !ok {
		return
	}
	pcm = kart.ResamplePCM(pcm, pitch)
	pcm = kart.PanPCM(pcm, pan)
	p := audioCtx.NewPlayerFromBytes(pcm)
	p.SetVolume(vol)
	p.Play()
}

// SoundAtOff 在指定拍播放音效并跳过开头 offset 秒（SoundByte 的 offset 参数）。
func (c *Ctx) SoundAtOff(beat float64, name string, vol, offsetSec float64) {
	c.At(beat, func() {
		pcm, ok := c.Assets.Sounds[name]
		if !ok {
			return
		}
		skip := int(offsetSec*float64(SampleRate)) * 4
		if skip < 0 || skip >= len(pcm) {
			skip = 0
		}
		p := audioCtx.NewPlayerFromBytes(pcm[skip:])
		p.SetVolume(vol)
		p.Play()
	})
}

// SoundAt 在指定拍播放音效（MultiSound 等价物）。
func (c *Ctx) SoundAt(beat float64, name string, vol float64) {
	c.At(beat, func() { c.SoundVol(name, vol) })
}

// SoundAtPitchPan 在指定拍播放带音高和声像的音效。
func (c *Ctx) SoundAtPitchPan(beat float64, name string, vol, pitch, pan float64) {
	c.At(beat, func() { c.SoundPitchPan(name, vol, pitch, pan) })
}

// ScheduleInput 注册一次输入判定（HS Minigame.ScheduleInput 等价物）。
// onHit 的 state：just 窗归一化偏移，|state|<=1 = just，1<|state|<=2 = NG，负 = 早。
func (c *Ctx) ScheduleInput(beat float64, onHit func(state float64, j Judgment), onMiss func()) {
	c.App.scheduleInput(beat, false, 0, onHit, onMiss)
}

// ScheduleInputAction 注册指定动作通道的按下判定（0=主键，1=副键）。
func (c *Ctx) ScheduleInputAction(beat float64, action int, onHit func(state float64, j Judgment), onMiss func()) {
	c.App.scheduleInput(beat, false, action, onHit, onMiss)
}

// ScheduleInputCond is ScheduleInput with HS' optional canJust predicate.
// Board Meeting uses this to invalidate a pending stop cue after the player's
// chair was already stopped by an early whiff.
func (c *Ctx) ScheduleInputCond(beat float64, canHit func() bool, onHit func(state float64, j Judgment), onMiss func()) {
	c.App.scheduleInputCond(beat, false, 0, canHit, onHit, onMiss)
}

// ScheduleInputRelease 注册一次"抬起"判定（InputAction_FlickRelease，
// totemClimb 高跳甩出等）。空抬不触发 Whiff。
func (c *Ctx) ScheduleInputRelease(beat float64, onHit func(state float64, j Judgment), onMiss func()) {
	c.App.scheduleInput(beat, true, 0, onHit, onMiss)
}

// PressedNow / ReleasedNow 报告本逻辑帧是否有按下/抬起（HoldCo 式轮询用）。
func (c *Ctx) PressedNow() bool  { return c.App.pressedNow }
func (c *Ctx) ReleasedNow() bool { return c.App.releasedNow }

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

// ScoreMiss 记一次 miss（HS Minigame.ScoreMiss：无对应判定窗的扣分，
// 如 totemClimb 高跳保持期提前松手）。
func (c *Ctx) ScoreMiss() {
	c.App.misses++
	c.App.recordMissScore(c.App.cond.Beat())
	c.App.setMsg("MISS...")
}

// CameraAt 返回 beat 时刻相机世界位置（vfx/move camera 时间轴，默认 (0,0,-10)）。
func (c *Ctx) CameraAt(beat float64) [3]float64 { return c.App.CameraAt(beat) }

// SampleScene applies the global GameCamera timeline before sampling the scene.
// Remix charts drive cross-game zooms/pans through vfx/move camera, so every
// scene-backed minigame must go through this helper instead of sampling with the
// prefab default camera.
func (c *Ctx) SampleScene(beat float64) [3]float64 {
	return c.SampleSceneZ(beat, 0)
}

// SampleSceneZ is SampleScene with an additional Z offset for game-local punch
// zooms (for example Lockstep and Cheer Readers).
func (c *Ctx) SampleSceneZ(beat, addZ float64) [3]float64 {
	cam := c.App.CameraAt(beat)
	if c.Scene != nil {
		c.Scene.SetCamera(cam[0], cam[1], cam[2]+addZ)
		c.Scene.Sample(beat)
	}
	return cam
}

// PlayCommon 播放公共音效（assets/common：miss/nearMiss/count-ins 等）。
func (c *Ctx) PlayCommon(name string) { c.App.PlayCommon(name, 1) }

// Time / Beat 返回当前歌曲时间与节拍。
func (c *Ctx) Time() float64 { return c.App.cond.Time() }
func (c *Ctx) Beat() float64 { return c.App.cond.Beat() }

// BeatToTime 经 tempo map 换算。
func (c *Ctx) BeatToTime(beat float64) float64 { return c.App.bm.BeatToTime(beat) }

// SecPerBeat 返回某拍处的秒/拍（用于以 1x 真实速度播放剪辑：timeScale = SecPerBeat）。
func (c *Ctx) SecPerBeat(beat float64) float64 { return 60 / c.App.bm.BPMAt(beat) }

// PlaySeq 播放音效序列（PlaySoundSequence 等价物），片段拍偏移相对 beat。
func (c *Ctx) PlaySeq(name string, beat float64) {
	for _, clip := range c.Assets.Extra.Sequences[name] {
		clip := clip
		c.SoundAt(beat+clip.Beat, clip.Clip, clip.Volume)
	}
}

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

// SoundLoop 循环播放音效，返回停止函数（HS looping SoundByte + KillLoop，
// totemClimb 的 charge_start 等）。
func (c *Ctx) SoundLoop(name string) func() { return c.SoundLoopVol(name, 1) }

// SoundLoopVol 带音量的循环播放（kitties spinnya 0.85 等）。
func (c *Ctx) SoundLoopVol(name string, vol float64) func() {
	pcm, ok := c.Assets.Sounds[name]
	if !ok {
		return func() {}
	}
	loop := audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm)))
	p, err := audioCtx.NewPlayer(loop)
	if err != nil {
		return func() {}
	}
	p.SetVolume(vol)
	p.Play()
	return func() { p.Pause() }
}
