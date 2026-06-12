// ctx.go：模块对引擎的访问句柄（HS 中 Minigame 基类提供的服务）。
package engine

import (
	"path/filepath"

	"hsdemo/kart"
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
	pcm, ok := c.Assets.Sounds[name]
	if !ok {
		return
	}
	p := audioCtx.NewPlayerFromBytes(kart.ResamplePCM(pcm, pitch))
	p.SetVolume(vol)
	p.Play()
}

// SoundAt 在指定拍播放音效（MultiSound 等价物）。
func (c *Ctx) SoundAt(beat float64, name string, vol float64) {
	c.At(beat, func() { c.SoundVol(name, vol) })
}

// ScheduleInput 注册一次输入判定（HS Minigame.ScheduleInput 等价物）。
// onHit 的 state：just 窗归一化偏移，|state|<=1 = just，1<|state|<=2 = NG，负 = 早。
func (c *Ctx) ScheduleInput(beat float64, onHit func(state float64, j Judgment), onMiss func()) {
	c.App.scheduleInput(beat, onHit, onMiss)
}

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
