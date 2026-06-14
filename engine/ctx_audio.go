package engine

import (
	"bytes"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/kart"
)

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

// SoundPitchOff 立即播放带音高并跳过开头 offset 秒的音效。
func (c *Ctx) SoundPitchOff(name string, vol, pitch, offsetSec float64) {
	pcm, ok := c.Assets.Sounds[name]
	if !ok {
		return
	}
	skip := int(offsetSec*float64(SampleRate)) * 4
	if skip < 0 || skip >= len(pcm) {
		skip = 0
	}
	pcm = kart.ResamplePCM(pcm[skip:], pitch)
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

// SoundAtPitchOff 在指定拍播放带音高并跳过开头 offset 秒的音效。
func (c *Ctx) SoundAtPitchOff(beat float64, name string, vol, pitch, offsetSec float64) {
	c.At(beat, func() { c.SoundPitchOff(name, vol, pitch, offsetSec) })
}

// SoundAt 在指定拍播放音效（MultiSound 等价物）。
func (c *Ctx) SoundAt(beat float64, name string, vol float64) {
	c.At(beat, func() { c.SoundVol(name, vol) })
}

// SoundAtPitchPan 在指定拍播放带音高和声像的音效。
func (c *Ctx) SoundAtPitchPan(beat float64, name string, vol, pitch, pan float64) {
	c.At(beat, func() { c.SoundPitchPan(name, vol, pitch, pan) })
}

// PlayCommon 播放公共音效（assets/common：miss/nearMiss/count-ins 等）。
func (c *Ctx) PlayCommon(name string) { c.App.PlayCommon(name, 1) }

// PlaySeq 播放音效序列（PlaySoundSequence 等价物），片段拍偏移相对 beat。
func (c *Ctx) PlaySeq(name string, beat float64) {
	for _, clip := range c.Assets.Extra.Sequences[name] {
		clip := clip
		c.SoundAt(beat+clip.Beat, clip.Clip, clip.Volume)
	}
}

// SoundLoop 循环播放音效，返回停止函数（HS looping SoundByte + KillLoop，
// totemClimb 的 charge_start 等）。
func (c *Ctx) SoundLoop(name string) func() { return c.SoundLoopVol(name, 1) }

// SoundLoopVol 带音量的循环播放（kitties spinnya 0.85 等）。
func (c *Ctx) SoundLoopVol(name string, vol float64) func() {
	return c.SoundLoopPitchVol(name, 1, vol)
}

// SoundLoopPitchVol 带音高与音量的循环播放。Glee Club 的 WailLoop
// 用 SoundByte.GetPitchFromSemiTones 变调后循环，必须在创建 loop 前重采样。
func (c *Ctx) SoundLoopPitchVol(name string, pitch, vol float64) func() {
	pcm, ok := c.Assets.Sounds[name]
	if !ok {
		return func() {}
	}
	pcm = kart.ResamplePCM(pcm, pitch)
	loop := audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm)))
	p, err := audioCtx.NewPlayer(loop)
	if err != nil {
		return func() {}
	}
	p.SetVolume(vol)
	p.Play()
	return func() { p.Pause() }
}
