package engine

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (a *App) handleLevelSelectCommands() {
	openedSearch := false
	if inpututil.IsKeyJustPressed(ebiten.KeySlash) {
		a.menuSearchOpen = !a.menuSearchOpen
		openedSearch = a.menuSearchOpen
	}
	if a.menuSearchOpen {
		a.handleMenuSearchText(openedSearch)
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyO) {
		a.menuSort = (a.menuSort + 1) % 3
		a.rebuildMenuLevelsPreservingSelection()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyV) {
		a.favoritesOnly = !a.favoritesOnly
		a.rebuildMenuLevelsPreservingSelection()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		a.toggleSelectedFavorite()
	}
}

func (a *App) handleMenuSearchText(openedSearch bool) {
	changed := false
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		rs := []rune(a.menuQuery)
		if len(rs) > 0 {
			a.menuQuery = string(rs[:len(rs)-1])
			changed = true
		} else {
			a.menuSearchOpen = false
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) && a.menuQuery != "" {
		a.menuQuery = ""
		changed = true
	}
	if !openedSearch {
		if chars := ebiten.AppendInputChars(nil); len(chars) > 0 {
			a.menuQuery += strings.TrimSpace(string(chars))
			changed = true
		}
	}
	if changed {
		a.rebuildMenuLevelsPreservingSelection()
	}
}

func (a *App) handleLevelSelectMovement() {
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
}

func (a *App) selectHoveredLevel() bool {
	idx, ok := a.hoveredMenuLevel()
	if !ok {
		return false
	}
	a.menuSel = idx
	a.keepMenuSelectionVisible()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		a.loadSelectedLevel()
		return true
	}
	return false
}
