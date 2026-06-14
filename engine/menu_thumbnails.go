package engine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (a *App) drawLevelThumbnail(screen *ebiten.Image, level menuLevel, idx int, x, y, size float64) {
	inner := size * 0.78
	innerX := x + (size-inner)/2
	innerY := y + (size-inner)/2
	vector.DrawFilledRect(screen, float32(innerX), float32(innerY), float32(inner), float32(inner), color.RGBA{240, 232, 220, 255}, false)
	if level.customIcon != nil {
		drawImageFit(screen, level.customIcon, innerX, innerY, inner, inner, 1)
	} else {
		a.drawFallbackLevelIcon(screen, level, idx, innerX, innerY, inner)
	}
	if a.libraryAssets.border != nil {
		drawImageFit(screen, a.libraryAssets.border, x, y, size, size, 1)
	} else {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(size), 5, color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x), float32(y+size-5), float32(size), 5, color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x), float32(y), 5, float32(size), color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x+size-5), float32(y), 5, float32(size), color.RGBA{116, 94, 128, 255}, false)
	}
}

func (a *App) drawFallbackLevelIcon(screen *ebiten.Image, level menuLevel, idx int, x, y, size float64) {
	games := level.games
	if len(games) == 0 {
		games = []string{"RIQ"}
	}
	if len(games) == 1 {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(size), float32(size), menuAccent(idx), false)
		a.text(screen, a.fitText(menuGameLabel(games[0]), a.faceMid, size-18), a.faceMid, x+size/2, y+size/2-12, color.RGBA{255, 252, 242, 255}, true)
		return
	}
	tile := (size - 5) / 2
	for i := 0; i < 4; i++ {
		tx := x + float64(i%2)*(tile+5)
		ty := y + float64(i/2)*(tile+5)
		c := menuAccent(idx + i)
		vector.DrawFilledRect(screen, float32(tx), float32(ty), float32(tile), float32(tile), c, false)
		if i < len(games) {
			label := a.fitText(menuGameLabel(games[i]), a.faceSmall, tile-8)
			a.text(screen, label, a.faceSmall, tx+tile/2, ty+tile/2-8, color.RGBA{255, 252, 242, 255}, true)
		}
	}
}
