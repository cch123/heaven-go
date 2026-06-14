// result_audio.go：Heaven Studio Judgement 场景音效/音乐事件。
package engine

import (
	"github.com/hajimehoshi/ebiten/v2/audio"
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

func (a *App) resetResultAudioCues() {
	a.stopResultAudio()
	a.resultAudioState = resultAudioState{fired: map[string]bool{}}
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
