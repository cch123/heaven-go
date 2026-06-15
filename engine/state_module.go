package engine

type moduleRuntimeState struct {
	modules  map[string]Module
	active   Module
	switches []gameSwitch
	swIdx    int
	actions  []beatAction
	actIdx   int
	unported []string
}
