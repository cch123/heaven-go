package engine

import (
	"bytes"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

func (a *App) stopResultAudio() {
	if a.resultAudioState.loop != nil {
		a.resultAudioState.loop.Close()
		a.resultAudioState.loop = nil
	}
	a.resultAudioState.pendingLoop = ""
}

func (a *App) playResultLoopOnceAt(t float64, key, name string, vol float64) {
	if a.resultT >= t && a.fireResultCue(key) {
		a.startResultLoop(name, vol)
	}
}

func (a *App) playResultSound(name string, vol float64) {
	pcm := a.resultSoundPCM(name)
	if len(pcm) == 0 || audioCtx == nil {
		return
	}
	p := audioCtx.NewPlayerFromBytes(pcm)
	p.SetVolume(vol)
	p.Play()
}

func (a *App) startResultLoop(name string, vol float64) {
	pcm := a.resultSoundPCM(name)
	if len(pcm) == 0 || audioCtx == nil {
		return
	}
	a.stopResultAudio()
	loop := audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm)))
	p, err := audioCtx.NewPlayer(loop)
	if err != nil {
		return
	}
	p.SetVolume(vol)
	p.Play()
	a.resultAudioState.loop = p
}

func (a *App) scheduleResultLoop(name string, at float64) {
	if _, ok := a.resultAudio.clips[strings.ToLower(name)]; !ok {
		return
	}
	a.resultAudioState.pendingLoop = name
	a.resultAudioState.pendingLoopAt = at
}

func (a *App) resultSoundDuration(name string) float64 {
	pcm := a.resultSoundPCM(name)
	if len(pcm) == 0 {
		return 0
	}
	return float64(len(pcm)/4) / SampleRate
}

func (a *App) resultSoundPCM(name string) []byte {
	if a.resultAudio.clips == nil {
		return nil
	}
	return a.resultAudio.clips[strings.ToLower(name)]
}
