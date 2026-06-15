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
		a.updateTitle()
	case statePlay:
		a.cond.Update()
		a.updatePlay()
	case stateResult:
		return a.updateResult()
	}
	return nil
}
