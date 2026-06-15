package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (a *App) updateResult() error {
	a.resultT += 1 / float64(ebiten.TPS())
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		a.stopResultAudio()
		return a.restart()
	}
	if titlePressed() && a.advanceResult() {
		return nil
	}
	a.updateResultAudio()
	return nil
}

// advanceResult returns true when it leaves the result screen; the caller then
// skips the rest of the result-frame work, matching the old inline state switch.
func (a *App) advanceResult() bool {
	if !a.resultEpilogue {
		if a.resultT < a.resultRankTime() {
			a.resultT = a.resultRankTime()
			a.skipResultAudioToRank()
		} else {
			a.enterResultEpilogue()
		}
		return false
	}
	if a.resultT > resultEpilogueWait {
		a.returnToLevelSelect()
		return true
	}
	return false
}
