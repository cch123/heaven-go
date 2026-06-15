package engine

const (
	ScreenW = 960
	ScreenH = 540

	SampleRate = 44100

	// 判定窗口（秒），对应 Minigame.cs 的 ace/just/ngTimeBase
	WinAce  = 0.01
	WinJust = 0.05
	WinNG   = 0.10

	rankOkThreshold = 0.6
	rankHiThreshold = 0.8
)

const (
	menuGridX        = 54
	menuGridY        = 116
	menuCardW        = 148
	menuCardH        = 172
	menuCardGapX     = 20
	menuCardGapY     = 24
	menuGridCols     = 3
	menuGridRows     = 2
	menuVisibleItems = menuGridCols * menuGridRows
)

const (
	// JudgementOpen.playable signal times. The bar parameters are serialized
	// on Judgement.unity's JudgementManager component.
	resultMessage1Time  = 1.2333333333333325
	resultMessage0Time  = 1.7333333333333325
	resultMessage2Time  = 2.2333333333333325
	resultBarStart      = 3.899999999999999
	resultBarDuration   = 2.5
	resultBarRankWait   = 1
	resultRankMusicWait = 1.5
	resultEpilogueWait  = 1.5
)
