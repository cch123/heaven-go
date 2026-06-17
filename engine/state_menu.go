package engine

type menuRuntimeState struct {
	allLevels  []menuLevel
	levels     []menuLevel
	menuSel    int
	menuScroll int

	menuSort        menuSortMode
	menuQuery       string
	menuSearchOpen  bool
	favoritesOnly   bool
	currentLevelKey string
	libraryRecords  map[string]menuRecord
	libraryAssets   libraryAssets
}
