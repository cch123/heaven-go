package engine

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (a *App) drawResultEpilogue(screen *ebiten.Image, white color.RGBA) {
	img := a.resultAssets.epNg
	msg := a.resultProp("epilogue_ng")
	switch a.result.Rank {
	case resultRankOk:
		img, msg = a.resultAssets.epOk, a.resultProp("epilogue_ok")
	case resultRankHi:
		img, msg = a.resultAssets.epHi, a.resultProp("epilogue_hi")
	}
	if img != nil {
		drawImageCover(screen, img, 0, 0, ScreenW, ScreenH, 1)
	} else {
		screen.Fill(resultRankColor(a.result.Rank))
	}
	vector.DrawFilledRect(screen, 0, ScreenH-116, ScreenW, 116, color.RGBA{24, 20, 30, 218}, false)
	a.text(screen, msg, a.faceBig, 54, ScreenH-96, white, false)
	a.text(screen, fmt.Sprintf("Final score %d  |  ACE %d  OK %d  NG %d  MISS %d",
		int(a.result.Score*100), a.aces, a.justs, a.ngs, a.misses),
		a.faceSmall, 58, ScreenH-42, color.RGBA{218, 214, 226, 255}, false)
	a.text(screen, "Enter / Click - level select    R - replay    Esc - quit", a.faceSmall, ScreenW-326, ScreenH-42, white, false)
}
