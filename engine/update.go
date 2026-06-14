package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// ---------- Update ----------

func (a *App) Update() error {
	if HandleFullscreenShortcut() {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		a.debug = !a.debug
	}
	a.pollDroppedRiq()

	switch a.state {
	case stateTitle:
		if a.bm == nil {
			a.updateLevelSelect()
			return nil
		}
		if a.bm != nil && (titlePressed() || a.Autoplay) {
			a.cond.Play()
			a.state = statePlay
		}
	case statePlay:
		a.cond.Update()
		a.updatePlay()
	case stateResult:
		a.resultT += 1 / float64(ebiten.TPS())
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			a.stopResultAudio()
			return a.restart()
		}
		if titlePressed() {
			if !a.resultEpilogue {
				if a.resultT < resultRankTime {
					a.resultT = resultRankTime
					a.skipResultAudioToRank()
				} else {
					a.enterResultEpilogue()
				}
			} else if a.resultT > 1.5 {
				a.returnToLevelSelect()
				return nil
			}
		}
		a.updateResultAudio()
	}
	return nil
}
