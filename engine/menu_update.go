package engine

func (a *App) updateTitle() {
	if a.bm == nil {
		a.updateLevelSelect()
		return
	}
	if titlePressed() || a.Autoplay {
		a.startPlay()
	}
}

func (a *App) updateLevelSelect() {
	if len(a.levels) == 0 {
		a.handleLevelSelectCommands()
		return
	}
	a.handleLevelSelectCommands()
	a.handleLevelSelectMovement()
	if a.selectHoveredLevel() {
		return
	}
	if menuConfirmPressed() {
		a.loadSelectedLevel()
	}
}
