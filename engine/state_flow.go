package engine

type appFlowState struct {
	state   gameState
	endBeat float64

	lastMsg string
	msgT    float64
	debug   bool
	loadErr string
}
