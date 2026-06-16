package engine

import "hsdemo/kart"

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

// SoundPitchPanOff 立即播放带音高、声像并跳过开头 offset 秒的音效。
// Rap Men 的 MultiSound 声线同时依赖 offset 和 panning，不能降级成只支持其一。
func (c *Ctx) SoundPitchPanOff(name string, vol, pitch, pan, offsetSec float64) {
	pcm, ok := c.Assets.Sounds[name]
	if !ok {
		return
	}
	skip := int(offsetSec*float64(SampleRate)) * 4
	if skip < 0 || skip >= len(pcm) {
		skip = 0
	}
	pcm = kart.ResamplePCM(pcm[skip:], pitch)
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

// SoundAtPitchPanOff 在指定拍播放带音高、声像和开头 offset 的音效。
func (c *Ctx) SoundAtPitchPanOff(beat float64, name string, vol, pitch, pan, offsetSec float64) {
	c.At(beat, func() { c.SoundPitchPanOff(name, vol, pitch, pan, offsetSec) })
}

// SetMinigamePitch changes the current chart music pitch and beat clock,
// matching HS Conductor.SetMinigamePitch for game-local slowdowns.
func (c *Ctx) SetMinigamePitch(pitch float64) {
	c.App.setMinigamePitch(pitch)
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

// SoundLoopPitchVol 带音高与音量的循环播放，返回停止函数。
// 需要播放中变调的调用点应使用 SoundLoopPitchHandle。
func (c *Ctx) SoundLoopPitchVol(name string, pitch, vol float64) func() {
	return c.SoundLoopPitchHandle(name, pitch, vol).StopFunc()
}

// SoundLoopPitchVolUntil mirrors Sound.SetLoopParams: the loop stays at full
// volume until endBeat, then fades over fadeSec before being stopped.
func (c *Ctx) SoundLoopPitchVolUntil(name string, pitch, vol, endBeat, fadeSec float64) *SoundLoopHandle {
	handle := c.SoundLoopPitchHandle(name, pitch, vol)
	c.fadeAndStopLoop(handle, endBeat, fadeSec, vol)
	return handle
}

func (c *Ctx) fadeAndStopLoop(handle *SoundLoopHandle, endBeat, fadeSec, vol float64) {
	if handle == nil {
		return
	}
	if fadeSec <= 0 || c.App == nil || c.App.bm == nil {
		c.At(endBeat, handle.Stop)
		return
	}
	startTime := c.BeatToTime(endBeat)
	stopTime := startTime + fadeSec
	const steps = 6
	for i := 1; i <= steps; i++ {
		u := float64(i) / steps
		level := vol * (1 - u)
		beat := c.TimeToBeat(startTime + fadeSec*u)
		c.At(beat, func() { handle.SetVolume(level) })
	}
	c.At(c.TimeToBeat(stopTime), handle.Stop)
}

// SoundLoopPitchHandle 创建可在播放期间变更 pitch 的循环音效。
// Rockers 的 BendUp/BendDown 与 Fillbots 的 water loop 都依赖 Unity
// AudioSource.pitch 连续变化，因此这里不能再预先重采样成固定 pitch。
func (c *Ctx) SoundLoopPitchHandle(name string, pitch, vol float64) *SoundLoopHandle {
	pcm, ok := c.Assets.Sounds[name]
	if !ok {
		return &SoundLoopHandle{}
	}
	reader := newPitchLoopReader(pcm, pitch)
	p, err := audioCtx.NewPlayer(reader)
	if err != nil {
		return &SoundLoopHandle{}
	}
	p.SetVolume(vol)
	p.Play()
	return &SoundLoopHandle{player: p, reader: reader}
}
