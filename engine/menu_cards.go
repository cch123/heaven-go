package engine

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (a *App) drawLevelCard(screen *ebiten.Image, level menuLevel, idx int, x, y float64, selected bool) {
	if selected {
		vector.DrawFilledRect(screen, float32(x-5), float32(y-5), menuCardW+10, menuCardH+10, color.RGBA{255, 227, 95, 235}, false)
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), menuCardW, menuCardH, color.RGBA{255, 252, 242, 236}, false)
	vector.DrawFilledRect(screen, float32(x), float32(y+menuCardH-37), menuCardW, 37, color.RGBA{246, 239, 229, 245}, false)
	if selected {
		vector.DrawFilledRect(screen, float32(x), float32(y+menuCardH-4), menuCardW, 4, color.RGBA{118, 88, 148, 255}, false)
	}
	a.drawLevelThumbnail(screen, level, idx, x+11, y+9, 126)
	title := a.fitText(level.displayName(), a.faceSmall, menuCardW-20)
	a.text(screen, title, a.faceSmall, x+10, y+menuCardH-29, color.RGBA{68, 54, 82, 255}, false)
	meta := "RIQ"
	if len(level.games) > 0 {
		meta = fmt.Sprintf("%d games", len(level.games))
	}
	a.text(screen, meta, a.faceSmall, x+10, y+menuCardH-12, color.RGBA{120, 106, 133, 255}, false)
}
