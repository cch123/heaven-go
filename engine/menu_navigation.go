package engine

import "github.com/hajimehoshi/ebiten/v2"

func (a *App) moveMenu(delta int) {
	if len(a.levels) == 0 {
		a.menuSel = 0
		a.menuScroll = 0
		return
	}
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
	if len(a.levels) == 0 {
		a.menuSel = 0
		a.menuScroll = 0
		return
	}
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

func (a *App) rebuildMenuLevelsPreservingSelection() {
	key := ""
	if a.menuSel >= 0 && a.menuSel < len(a.levels) {
		key = a.levels[a.menuSel].key
	}
	a.menuRuntimeState.rebuildMenuLevels()
	if key != "" {
		for i := range a.levels {
			if a.levels[i].key == key {
				a.menuSel = i
				break
			}
		}
	}
	a.keepMenuSelectionVisible()
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
