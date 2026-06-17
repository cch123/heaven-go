package engine

func (a *App) toggleSelectedFavorite() {
	if a.menuSel < 0 || a.menuSel >= len(a.levels) {
		return
	}
	key := a.levels[a.menuSel].key
	rec := a.libraryRecords[key]
	rec.Favorite = !rec.Favorite
	a.libraryRecords[key] = rec
	a.applyRecordToLevel(key, rec)
	saveMenuRecords(a.libraryRecords)
	a.rebuildMenuLevelsPreservingSelection()
}

func (a *App) recordCurrentLevelResult() {
	if a.currentLevelKey == "" {
		return
	}
	rec := a.libraryRecords[a.currentLevelKey]
	rec.Rank = menuRankString(menuRankFromResult(a.result.Rank))
	rec.Perfect = a.result.Perfect
	rec.Played++
	a.libraryRecords[a.currentLevelKey] = rec
	a.applyRecordToLevel(a.currentLevelKey, rec)
	saveMenuRecords(a.libraryRecords)
	a.rebuildMenuLevelsPreservingSelection()
}

func (a *App) applyRecordToLevel(key string, rec menuRecord) {
	for i := range a.allLevels {
		if a.allLevels[i].key == key {
			a.allLevels[i].applyRecord(rec)
		}
	}
}
