package engine

type menuRuntimeState struct {
	levels     []menuLevel
	menuSel    int
	menuScroll int

	libraryAssets libraryAssets
}
