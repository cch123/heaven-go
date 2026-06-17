package engine

import (
	"fmt"

	"hsdemo/riq"
)

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
	a.currentLevelKey = level.key
	a.loadErr = ""
	a.state = stateTitle
}
