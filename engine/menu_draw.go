package engine

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (a *App) drawTitle(screen *ebiten.Image, white, dim color.RGBA) {
	if a.bm == nil {
		a.drawLevelSelect(screen, white, dim)
		return
	} else {
		title := a.bm.Prop("remixtitle")
		if title == "" {
			title = "Untitled Remix"
		}
		a.text(screen, title, a.faceBig, ScreenW/2, 110, white, true)
		if author := a.bm.Prop("remixauthor"); author != "" {
			a.text(screen, "chart by "+author, a.faceMid, ScreenW/2, 170, dim, true)
		}
		a.text(screen, fmt.Sprintf("%d inputs | %.0f BPM | games: %s",
			len(a.inputs), a.bm.Tempos[0].BPM, strings.Join(keys(a.modules), ", ")),
			a.faceSmall, ScreenW/2, 208, dim, true)
		if len(a.unported) > 0 {
			a.text(screen, "Unported: "+strings.Join(a.unported, ", "), a.faceSmall, ScreenW/2, 232,
				color.RGBA{255, 170, 120, 255}, true)
		}
		a.text(screen, "Space / J / Click to play    (drop another .riq to switch)", a.faceMid, ScreenW/2, ScreenH-110, white, true)
		a.text(screen, "press to start", a.faceMid, ScreenW/2, ScreenH-72, white, true)
	}
	if a.loadErr != "" {
		a.text(screen, a.loadErr, a.faceSmall, ScreenW/2, ScreenH-36, color.RGBA{255, 120, 120, 255}, true)
	}
}

func (a *App) drawLevelSelect(screen *ebiten.Image, white, dim color.RGBA) {
	a.drawLibraryBackground(screen)
	vector.DrawFilledRect(screen, 0, 0, ScreenW, 78, color.RGBA{255, 250, 236, 220}, false)
	vector.DrawFilledRect(screen, 0, 77, ScreenW, 2, color.RGBA{118, 88, 148, 160}, false)
	vector.DrawFilledRect(screen, 0, ScreenH-54, ScreenW, 54, color.RGBA{255, 250, 236, 220}, false)

	ink := color.RGBA{66, 50, 88, 255}
	soft := color.RGBA{104, 92, 118, 255}
	a.text(screen, "Library", a.faceBig, 58, 20, ink, false)
	a.text(screen, "HEAVEN GO", a.faceSmall, 854, 30, color.RGBA{122, 105, 142, 255}, true)

	if len(a.levels) == 0 {
		vector.DrawFilledRect(screen, 248, 178, 464, 154, color.RGBA{255, 252, 242, 230}, false)
		vector.DrawFilledRect(screen, 248, 178, 464, 4, color.RGBA{118, 88, 148, 210}, false)
		a.text(screen, "No .riq levels found under levels/", a.faceMid, ScreenW/2, 222, ink, true)
		a.text(screen, "Drop a .riq file here to play", a.faceMid, ScreenW/2, 274, soft, true)
		if a.loadErr != "" {
			a.text(screen, a.fitText(a.loadErr, a.faceSmall, 760), a.faceSmall, ScreenW/2, ScreenH-36, color.RGBA{255, 120, 120, 255}, true)
		}
		return
	}

	a.keepMenuSelectionVisible()
	for slot := 0; slot < menuVisibleItems; slot++ {
		idx := a.menuScroll + slot
		if idx >= len(a.levels) {
			break
		}
		col := slot % menuGridCols
		row := slot / menuGridCols
		x := float64(menuGridX + col*(menuCardW+menuCardGapX))
		y := float64(menuGridY + row*(menuCardH+menuCardGapY))
		a.drawLevelCard(screen, a.levels[idx], idx, x, y, idx == a.menuSel)
	}

	first := a.menuScroll + 1
	last := a.menuScroll + menuVisibleItems
	if last > len(a.levels) {
		last = len(a.levels)
	}
	a.drawLibraryLevelInfo(screen, a.levels[a.menuSel], a.menuSel, ink, soft)
	a.text(screen, fmt.Sprintf("%d-%d / %d", first, last, len(a.levels)), a.faceSmall, 66, ScreenH-34, soft, false)
	a.text(screen, "Enter / Click    Arrows / WASD    Drop .riq", a.faceSmall, ScreenW/2, ScreenH-34, soft, true)
	if a.loadErr != "" {
		a.text(screen, a.fitText(a.loadErr, a.faceSmall, 840), a.faceSmall, ScreenW/2, ScreenH-18, color.RGBA{255, 120, 120, 255}, true)
	}
}

func (a *App) drawLibraryBackground(screen *ebiten.Image) {
	if a.libraryAssets.bgBase == nil {
		screen.Fill(color.RGBA{231, 226, 215, 255})
		return
	}
	drawImageCover(screen, a.libraryAssets.bgBase, 0, 0, ScreenW, ScreenH, 1)
	drawImageCover(screen, a.libraryAssets.bgGradient, 0, 0, ScreenW, ScreenH, 0.7)
	drawImageCover(screen, a.libraryAssets.bgStars, 0, 0, ScreenW, ScreenH, 0.28)
	drawImageCover(screen, a.libraryAssets.bgWaves, 0, 0, ScreenW, ScreenH, 0.24)
	vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{255, 250, 238, 70}, false)
}
