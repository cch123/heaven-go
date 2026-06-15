package engine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

func (a *App) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{16, 16, 20, 255})
	t, beat := a.frameClock()
	a.drawGameCanvas(screen, t, beat)

	white := color.RGBA{245, 245, 250, 255}
	dim := color.RGBA{200, 200, 210, 200}
	switch a.state {
	case stateTitle:
		a.drawTitle(screen, white, dim)
	case statePlay:
		a.drawPlayHUD(screen, t, beat, white)
	case stateResult:
		a.drawResult(screen, white)
	}

	if a.debug {
		a.drawDebug(screen, t, beat)
	}
}

func (a *App) frameClock() (float64, float64) {
	if a.cond == nil {
		return 0, 0
	}
	return a.cond.Time(), a.cond.Beat()
}
