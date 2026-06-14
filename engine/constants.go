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
	resultMsgTime  = 0.55
	resultMsg2Time = 1.25
	resultBarStart = 1.8
	resultBarDur   = 1.25
	resultRankTime = 3.45
	// JudgementManager.WaitAndRank waits 1.5s after the rank sound before
	// starting the rank jingle/loop music.
	resultRankMusicWait = 1.5
)
