package engine

import "github.com/hajimehoshi/ebiten/v2/text/v2"

type menuRuntimeState struct {
	levels     []menuLevel
	menuSel    int
	menuScroll int

	libraryAssets libraryAssets
}

type resultRuntimeState struct {
	result           resultSummary
	resultAssets     resultAssets
	resultAudio      resultAudioAssets
	resultAudioState resultAudioState
	resultT          float64
	resultEpilogue   bool
}

type fontState struct {
	faceBig, faceMid, faceSmall *text.GoTextFace
}
