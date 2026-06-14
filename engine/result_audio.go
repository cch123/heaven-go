// result_audio.go：Heaven Studio Judgement 场景音效/音乐事件。
package engine

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/kart"
)

type resultAudioAssets struct {
	clips map[string][]byte
}

type resultAudioState struct {
	fired         map[string]bool
	loop          *audio.Player
	pendingLoop   string
	pendingLoopAt float64
}

func loadResultAudio(dir string) resultAudioAssets {
	out := resultAudioAssets{clips: map[string][]byte{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("engine: no result sound directory %s", dir)
		return out
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".wav" && ext != ".ogg" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		pcm, err := kart.DecodePCM(raw, ext, SampleRate)
		if err != nil {
			log.Printf("engine: decode result sound %s: %v", e.Name(), err)
			continue
		}
		key := strings.ToLower(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		out.clips[key] = pcm
	}
	return out
}

func (a *App) resetResultAudioCues() {
	a.stopResultAudio()
	a.resultAudioState = resultAudioState{fired: map[string]bool{}}
}

func (a *App) stopResultAudio() {
	if a.resultAudioState.loop != nil {
		a.resultAudioState.loop.Close()
		a.resultAudioState.loop = nil
	}
	a.resultAudioState.pendingLoop = ""
}

func (a *App) skipResultAudioToRank() {
	a.stopResultAudio()
	a.markResultAudioFired("message0", "message1", "message2", "barStart", "barStop", "star", "noMiss")
}

func (a *App) markResultAudioFired(keys ...string) {
	if a.resultAudioState.fired == nil {
		a.resultAudioState.fired = map[string]bool{}
	}
	for _, key := range keys {
		a.resultAudioState.fired[key] = true
	}
}

func (a *App) updateResultAudio() {
	if a.resultEpilogue {
		return
	}
	if a.result.TwoMessage {
		a.playResultOnceAt(resultMsgTime, "message1", "resultMessage2", 1)
		a.playResultOnceAt(resultMsg2Time, "message2", "resultMessage3", 1)
	} else {
		a.playResultOnceAt(resultMsgTime, "message0", "resultMessage3", 1)
	}
	a.playResultLoopOnceAt(resultBarStart, "barStart", "resultGauge", 0.85)
	if a.resultT >= resultBarStart+resultBarDur {
		if a.fireResultCue("barStop") {
			a.stopResultAudio()
			a.playResultSound("resultGaugeStop", 1)
		}
		if a.result.Star {
			a.playResultOnce("star", "resultStarGet", 1)
		}
		if a.result.NoMiss {
			a.playResultOnce("noMiss", "resultNoMiss", 1)
		}
	}
	rankSound, rankStart, rankLoop, _ := a.resultRankAudioNames()
	a.playResultOnceAt(resultRankTime, "rank", rankSound, 1)
	if a.resultT >= resultRankTime+resultRankMusicWait && a.fireResultCue("rankMusic") {
		a.playResultSound(rankStart, 0.9)
		a.scheduleResultLoop(rankLoop, a.resultT+a.resultSoundDuration(rankStart))
	}
	if a.resultAudioState.pendingLoop != "" && a.resultT >= a.resultAudioState.pendingLoopAt {
		name := a.resultAudioState.pendingLoop
		a.resultAudioState.pendingLoop = ""
		a.startResultLoop(name, 0.72)
	}
}

func (a *App) enterResultEpilogue() {
	a.stopResultAudio()
	_, _, _, jingle := a.resultRankAudioNames()
	a.playResultSound(jingle, 1)
	a.resultEpilogue = true
	a.resultT = 0
}

func (a *App) resultRankAudioNames() (rank, start, loop, jingle string) {
	switch a.result.Rank {
	case resultRankHi:
		return "resultRankHi", "mus_superb00", "mus_superb01", "jgl_superb"
	case resultRankOk:
		return "resultRankOk", "mus_ok00", "mus_ok01", "jgl_ok"
	default:
		return "resultRankNg", "mus_tryagain00", "mus_tryagain01", "jgl_tryagain"
	}
}

func (a *App) playResultOnceAt(t float64, key, name string, vol float64) {
	if a.resultT >= t {
		a.playResultOnce(key, name, vol)
	}
}

func (a *App) playResultOnce(key, name string, vol float64) {
	if a.fireResultCue(key) {
		a.playResultSound(name, vol)
	}
}

func (a *App) playResultLoopOnceAt(t float64, key, name string, vol float64) {
	if a.resultT >= t && a.fireResultCue(key) {
		a.startResultLoop(name, vol)
	}
}

func (a *App) fireResultCue(key string) bool {
	if a.resultAudioState.fired == nil {
		a.resultAudioState.fired = map[string]bool{}
	}
	if a.resultAudioState.fired[key] {
		return false
	}
	a.resultAudioState.fired[key] = true
	return true
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
