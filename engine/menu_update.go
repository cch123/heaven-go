package engine

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"hsdemo/riq"
)

func (a *App) updateLevelSelect() {
	if len(a.levels) == 0 {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) ||
		inpututil.IsKeyJustPressed(ebiten.KeyUp) ||
		inpututil.IsKeyJustPressed(ebiten.KeyW) {
		a.moveMenu(-menuGridCols)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeyDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeyS) {
		a.moveMenu(menuGridCols)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyA) {
		a.moveMenu(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyD) {
		a.moveMenu(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		a.moveMenu(-menuVisibleItems)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
		a.moveMenu(menuVisibleItems)
	}
	_, wheelY := ebiten.Wheel()
	if wheelY > 0 {
		a.moveMenu(-menuGridCols)
	} else if wheelY < 0 {
		a.moveMenu(menuGridCols)
	}
	if idx, ok := a.hoveredMenuLevel(); ok {
		a.menuSel = idx
		a.keepMenuSelectionVisible()
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			a.loadSelectedLevel()
			return
		}
	}
	if menuConfirmPressed() {
		a.loadSelectedLevel()
	}
}

func (a *App) moveMenu(delta int) {
	a.menuSel += delta
	if a.menuSel < 0 {
		a.menuSel = 0
	}
	if a.menuSel >= len(a.levels) {
		a.menuSel = len(a.levels) - 1
	}
	a.keepMenuSelectionVisible()
}

func (a *App) keepMenuSelectionVisible() {
	if a.menuSel < a.menuScroll {
		a.menuScroll = a.menuSel
	}
	if a.menuSel >= a.menuScroll+menuVisibleItems {
		a.menuScroll = a.menuSel - menuVisibleItems + 1
	}
	maxScroll := len(a.levels) - menuVisibleItems
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.menuScroll > maxScroll {
		a.menuScroll = maxScroll
	}
	if a.menuScroll < 0 {
		a.menuScroll = 0
	}
}

func (a *App) hoveredMenuLevel() (int, bool) {
	x, y := ebiten.CursorPosition()
	if x < menuGridX || x >= menuGridX+menuGridCols*(menuCardW+menuCardGapX)-menuCardGapX {
		return 0, false
	}
	if y < menuGridY || y >= menuGridY+menuGridRows*(menuCardH+menuCardGapY)-menuCardGapY {
		return 0, false
	}
	colStep := menuCardW + menuCardGapX
	rowStep := menuCardH + menuCardGapY
	col := (x - menuGridX) / colStep
	row := (y - menuGridY) / rowStep
	if x >= menuGridX+col*colStep+menuCardW || y >= menuGridY+row*rowStep+menuCardH {
		return 0, false
	}
	idx := a.menuScroll + row*menuGridCols + col
	if idx < 0 || idx >= len(a.levels) {
		return 0, false
	}
	return idx, true
}

func (a *App) loadSelectedLevel() {
	if a.menuSel < 0 || a.menuSel >= len(a.levels) {
		return
	}
	level := a.levels[a.menuSel]
	r, err := riq.Load(level.path)
	if err != nil {
		a.loadErr = fmt.Sprintf("read %s failed: %v", level.displayName(), err)
		return
	}
	if err := a.loadRiq(r); err != nil {
		a.loadErr = fmt.Sprintf("load %s failed: %v", level.displayName(), err)
		return
	}
	a.loadErr = ""
	a.state = stateTitle
}
