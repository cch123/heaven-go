package engine

type resultRuntimeState struct {
	result           resultSummary
	resultAssets     resultAssets
	resultAudio      resultAudioAssets
	resultAudioState resultAudioState
	resultT          float64
	resultEpilogue   bool
}
